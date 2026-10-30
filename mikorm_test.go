package mikorm

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type Base struct {
	ID       string `mikorm:".id"`
	Disabled bool   `mikorm:"disabled"`
	Comment  string `mikorm:"comment"`
}

type IPAddress struct {
	Base
	Address   netip.Prefix `mikorm:"address"`
	Network   string       `mikorm:"network,readonly"`
	Interface string       `mikorm:"interface"`
	Dynamic   bool         `mikorm:"dynamic,readonly"`
	Internal  string       `mikorm:"-"`
}

type Lease struct {
	ID         string        `mikorm:".id"`
	MACAddress string        // untagged: "mac-address"
	LeaseTime  time.Duration `mikorm:"lease-time"`
	Priority   int           `mikorm:"priority"`
	Tags       []string      `mikorm:"tags"`
	Blocked    *bool         `mikorm:"blocked"`
}

func seedAddresses(f *fakeRouter) {
	f.seed("/ip/address",
		map[string]string{"address": "10.0.0.1/24", "network": "10.0.0.0", "interface": "ether1", "disabled": "false", "dynamic": "false"},
		map[string]string{"address": "10.0.1.1/24", "network": "10.0.1.0", "interface": "ether2", "disabled": "true", "dynamic": "false", "comment": "wan"},
		map[string]string{"address": "10.0.2.1/24", "network": "10.0.2.0", "interface": "ether1", "disabled": "false", "dynamic": "true"},
	)
}

func TestFind(t *testing.T) {
	f := newFakeRouter()
	seedAddresses(f)
	c := f.connect(t)
	ctx := context.Background()

	var all []IPAddress
	if err := c.Query("/ip/address").Find(ctx, &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d records, want 3", len(all))
	}
	want := IPAddress{
		Base:      Base{ID: "*2", Disabled: true, Comment: "wan"},
		Address:   netip.MustParsePrefix("10.0.1.1/24"),
		Network:   "10.0.1.0",
		Interface: "ether2",
	}
	if !reflect.DeepEqual(all[1], want) {
		t.Errorf("record = %+v, want %+v", all[1], want)
	}

	tests := []struct {
		name  string
		query *Query
		want  []string // expected .id values
	}{
		{"where", c.Query("/ip/address").Where("interface", "ether1"), []string{"*1", "*3"}},
		{"where bool", c.Query("/ip/address").Where("disabled", true), []string{"*2"}},
		{"where chained", c.Query("/ip/address").Where("interface", "ether1").Where("dynamic", false), []string{"*1"}},
		{"where not", c.Query("/ip/address").WhereNot("interface", "ether1"), []string{"*2"}},
		{"where in", c.Query("/ip/address").WhereIn("network", "10.0.0.0", "10.0.2.0"), []string{"*1", "*3"}},
		{"where in empty", c.Query("/ip/address").WhereIn("network"), nil},
		{"where exists", c.Query("/ip/address").WhereExists("comment"), []string{"*2"}},
		{"where not exists", c.Query("/ip/address").WhereNotExists("comment"), []string{"*1", "*3"}},
		{"where model", c.Query("/ip/address").WhereModel(IPAddress{Interface: "ether1", Dynamic: true}), []string{"*3"}},
		{"where model map", c.Query("/ip/address").WhereModel(map[string]string{"interface": "ether2"}), []string{"*2"}},
		{"cli path", c.Query("/ip address").Where("interface", "ether2"), []string{"*2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, err := tt.query.IDs(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(ids) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(ids, tt.want) {
				t.Errorf("ids = %v, want %v", ids, tt.want)
			}
		})
	}
}

func TestQueryImmutable(t *testing.T) {
	f := newFakeRouter()
	seedAddresses(f)
	c := f.connect(t)
	ctx := context.Background()

	base := c.Query("/ip/address").Where("interface", "ether1")
	a := base.Where("dynamic", true)
	b := base.Where("dynamic", false)

	for _, tc := range []struct {
		q    *Query
		want int
	}{{base, 2}, {a, 1}, {b, 1}} {
		n, err := tc.q.Count(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if n != tc.want {
			t.Errorf("count = %d, want %d", n, tc.want)
		}
	}
}

func TestFirstAndCount(t *testing.T) {
	f := newFakeRouter()
	seedAddresses(f)
	c := f.connect(t)
	ctx := context.Background()

	var addr IPAddress
	if err := c.Query("/ip/address").Where("comment", "wan").First(ctx, &addr); err != nil {
		t.Fatal(err)
	}
	if addr.ID != "*2" || addr.Interface != "ether2" {
		t.Errorf("unexpected record %+v", addr)
	}

	err := c.Query("/ip/address").Where("comment", "nope").First(ctx, &addr)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}

	var m map[string]string
	if err := c.Query("/ip/address").Select("address", "interface").First(ctx, &m); err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"address": "10.0.0.1/24", "interface": "ether1"}; !reflect.DeepEqual(m, want) {
		t.Errorf("map = %v, want %v", m, want)
	}

	n, err := c.Query("/ip/address").Select("address").Where("disabled", false).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}

func TestDecodeTypes(t *testing.T) {
	f := newFakeRouter()
	f.seed("/ip/dhcp-server/lease", map[string]string{
		"mac-address": "AA:BB:CC:DD:EE:FF",
		"lease-time":  "1d00:30:00",
		"priority":    "10",
		"tags":        "a,b",
		"blocked":     "no",
	})
	c := f.connect(t)

	var leases []*Lease
	if err := c.Query("/ip/dhcp-server/lease").Find(context.Background(), &leases); err != nil {
		t.Fatal(err)
	}
	if len(leases) != 1 {
		t.Fatalf("got %d leases", len(leases))
	}
	l := leases[0]
	if l.MACAddress != "AA:BB:CC:DD:EE:FF" || l.LeaseTime != 24*time.Hour+30*time.Minute ||
		l.Priority != 10 || !reflect.DeepEqual(l.Tags, []string{"a", "b"}) || l.Blocked == nil || *l.Blocked {
		t.Errorf("unexpected lease %+v", l)
	}

	f.seed("/bad", map[string]string{"priority": "high"})
	var bad []Lease
	if err := c.Query("/bad").Find(context.Background(), &bad); err == nil {
		t.Error("expected decode error")
	}
}

func TestAddSetRemove(t *testing.T) {
	f := newFakeRouter()
	c := f.connect(t)
	ctx := context.Background()
	q := c.Query("/ip/address")

	id, err := q.Add(ctx, &IPAddress{
		Base:      Base{ID: "*ignored", Comment: "lan"},
		Address:   netip.MustParsePrefix("192.168.1.1/24"),
		Network:   "ignored",
		Interface: "bridge",
		Internal:  "ignored",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("empty id")
	}
	want := []string{"/ip/address/add", "=comment=lan", "=address=192.168.1.1/24", "=interface=bridge"}
	if got := f.lastCommand(); !reflect.DeepEqual(got, want) {
		t.Errorf("command = %q, want %q", got, want)
	}

	if err := q.Set(ctx, id, map[string]any{"comment": "updated", "disabled": true}); err != nil {
		t.Fatal(err)
	}
	var addr IPAddress
	if err := q.Where(".id", id).First(ctx, &addr); err != nil {
		t.Fatal(err)
	}
	if addr.Comment != "updated" || !addr.Disabled {
		t.Errorf("after set: %+v", addr)
	}

	// A non-nil pointer sends a zero value explicitly.
	f.seed("/ip/dhcp-server/lease", map[string]string{"blocked": "true"})
	no := false
	if err := c.Query("/ip/dhcp-server/lease").Set(ctx, "*2", Lease{Blocked: &no}); err != nil {
		t.Fatal(err)
	}
	if got := f.records("/ip/dhcp-server/lease")[0]["blocked"]; got != "false" {
		t.Errorf("blocked = %q, want false", got)
	}

	if err := q.Enable(ctx, id); err != nil {
		t.Fatal(err)
	}
	if got := f.records("/ip/address")[0]["disabled"]; got != "false" {
		t.Errorf("disabled = %q after Enable", got)
	}
	if err := q.Disable(ctx, id); err != nil {
		t.Fatal(err)
	}
	if got := f.records("/ip/address")[0]["disabled"]; got != "true" {
		t.Errorf("disabled = %q after Disable", got)
	}

	if err := q.Remove(ctx); err != nil {
		t.Fatalf("Remove with no ids: %v", err)
	}
	if err := q.Remove(ctx, id); err != nil {
		t.Fatal(err)
	}
	if n, _ := q.Count(ctx); n != 0 {
		t.Errorf("count after remove = %d", n)
	}

	if err := q.Set(ctx, "", map[string]string{"comment": "x"}); err == nil {
		t.Error("Set with empty id should fail")
	}
}

func TestDeviceError(t *testing.T) {
	f := newFakeRouter()
	seedAddresses(f)
	c := f.connect(t)
	ctx := context.Background()

	_, err := c.Query("/fail").Add(ctx, map[string]string{"a": "b"})
	var de *DeviceError
	if !errors.As(err, &de) {
		t.Fatalf("err = %v, want *DeviceError", err)
	}
	if de.Command != "/fail/add" || de.Message != "failure: test error" {
		t.Errorf("unexpected error %+v", de)
	}

	err = c.Query("/ip/address").Remove(ctx, "*99")
	if !errors.As(err, &de) || de.Message != "no such item" {
		t.Errorf("err = %v", err)
	}

	// The connection must stay usable after a !trap.
	n, err := c.Query("/ip/address").Count(ctx)
	if err != nil || n != 3 {
		t.Errorf("count after error = %d, %v", n, err)
	}
}

func TestInvalidArguments(t *testing.T) {
	f := newFakeRouter()
	c := f.connect(t)
	ctx := context.Background()

	var s []IPAddress
	if err := c.Query("/ip/address").Find(ctx, s); err == nil {
		t.Error("Find with non-pointer should fail")
	}
	var x IPAddress
	if err := c.Query("/ip/address").Find(ctx, &x); err == nil {
		t.Error("Find with pointer to struct should fail")
	}
	var ints []int
	if err := c.Query("/ip/address").Find(ctx, &ints); err == nil {
		t.Error("Find with []int should fail")
	}
	if err := c.Query("/ip/address").First(ctx, &s); err == nil {
		t.Error("First with slice should fail")
	}
	if _, err := c.Query("/ip/address").Add(ctx, 42); err == nil {
		t.Error("Add with int should fail")
	}
	if _, err := c.Query("/ip/address").Where("x", struct{}{}).Count(ctx); err == nil {
		t.Error("Where with unsupported value should fail")
	}
}

func TestRun(t *testing.T) {
	f := newFakeRouter()
	seedAddresses(f)
	c := f.connect(t)

	reply, err := c.Run(context.Background(), "/ip/address/print", "?interface=ether2")
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.Records) != 1 || reply.Records[0]["comment"] != "wan" {
		t.Fatalf("unexpected reply %+v", reply)
	}
	var addrs []IPAddress
	if err := reply.Scan(&addrs); err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 || addrs[0].Comment != "wan" {
		t.Errorf("scan = %+v", addrs)
	}
}

func TestContextCancel(t *testing.T) {
	f := newFakeRouter()
	seedAddresses(f)
	c := f.connect(t)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Run(ctx, "/test/slow")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}

	// Other commands still work while the slow one is pending...
	if n, err := c.Query("/ip/address").Count(context.Background()); err != nil || n != 3 {
		t.Errorf("count = %d, %v", n, err)
	}
	// ...and after its late reply arrives.
	close(f.release)
	if n, err := c.Query("/ip/address").Count(context.Background()); err != nil || n != 3 {
		t.Errorf("count = %d, %v", n, err)
	}

	canceled, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if _, err := c.Run(canceled, "/ip/address/print"); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want Canceled", err)
	}
}

func TestConcurrentUse(t *testing.T) {
	f := newFakeRouter()
	seedAddresses(f)
	c := f.connect(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			iface := "ether1"
			want := 2
			if i%2 == 1 {
				iface, want = "ether2", 1
			}
			var addrs []IPAddress
			if err := c.Query("/ip/address").Where("interface", iface).Find(ctx, &addrs); err != nil {
				errs <- err
				return
			}
			if len(addrs) != want {
				errs <- errors.New("wrong result for " + iface)
			}
			for _, a := range addrs {
				if a.Interface != iface {
					errs <- errors.New("mixed up replies")
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestClose(t *testing.T) {
	f := newFakeRouter()
	c := f.connect(t)

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := c.Run(context.Background(), "/ip/address/print"); !errors.Is(err, ErrClosed) {
		t.Errorf("err = %v, want ErrClosed", err)
	}
}

func TestCloseUnblocksPending(t *testing.T) {
	f := newFakeRouter()
	c := f.connect(t)

	errC := make(chan error, 1)
	go func() {
		_, err := c.Run(context.Background(), "/test/slow")
		errC <- err
	}()
	time.Sleep(20 * time.Millisecond)
	c.Close()
	select {
	case err := <-errC:
		if !errors.Is(err, ErrClosed) {
			t.Errorf("err = %v, want ErrClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pending Run not unblocked by Close")
	}
}

func TestConnectionLost(t *testing.T) {
	f := newFakeRouter()
	c := f.connect(t)

	_, err := c.Run(context.Background(), "/test/hangup")
	if err == nil || !strings.Contains(err.Error(), "connection lost") {
		t.Errorf("err = %v, want connection lost", err)
	}
	_, err = c.Run(context.Background(), "/ip/address/print")
	if err == nil || !strings.Contains(err.Error(), "connection lost") {
		t.Errorf("err = %v, want connection lost", err)
	}
}

func TestLoginFailure(t *testing.T) {
	f := newFakeRouter()
	clientConn, serverConn := net.Pipe()
	go f.serve(serverConn)

	_, err := New(context.Background(), clientConn, Config{Username: "admin", Password: "wrong"})
	var de *DeviceError
	if !errors.As(err, &de) || !strings.Contains(de.Message, "invalid user name or password") {
		t.Errorf("err = %v, want login DeviceError", err)
	}
}

func TestLoginTimeout(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	go func() { _, _ = readWords(bufio.NewReader(serverConn)) }() // never replies

	start := time.Now()
	_, err := New(context.Background(), clientConn, Config{Username: "admin", Timeout: 50 * time.Millisecond})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("login took %v", time.Since(start))
	}
}

func TestDial(t *testing.T) {
	f := newFakeRouter()
	f.seed("/system/identity", map[string]string{"name": "router"})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn)
		}
	}()

	ctx := context.Background()
	c, err := Dial(ctx, Config{Address: ln.Addr().String(), Username: "admin", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	var identity struct {
		Name string `mikorm:"name"`
	}
	if err := c.Query("/system/identity").First(ctx, &identity); err != nil {
		t.Fatal(err)
	}
	if identity.Name != "router" {
		t.Errorf("name = %q", identity.Name)
	}

	if _, err := Dial(ctx, Config{Username: "admin"}); err == nil {
		t.Error("Dial without address should fail")
	}
	if _, err := Dial(ctx, Config{Address: "127.0.0.1"}); err == nil {
		t.Error("Dial without username should fail")
	}
	if _, err := Dial(ctx, Config{Address: ln.Addr().String(), Username: "admin", Password: "bad"}); err == nil {
		t.Error("Dial with wrong password should fail")
	}
}

func TestListen(t *testing.T) {
	f := newFakeRouter()
	c := f.connect(t)

	// Stop from the callback.
	stopErr := errors.New("stop")
	var got []string
	err := c.Listen(context.Background(), func(rec map[string]string) error {
		got = append(got, rec["n"])
		if len(got) == 3 {
			return stopErr
		}
		return nil
	}, "/test/listen")
	if !errors.Is(err, stopErr) {
		t.Fatalf("err = %v, want stopErr", err)
	}
	if want := []string{"0", "1", "2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("records = %v, want %v", got, want)
	}

	// Stop with the context; the device must receive /cancel.
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	err = c.Listen(ctx, func(map[string]string) error {
		if n++; n == 3 {
			cancel()
		}
		return nil
	}, "/test/listen")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want Canceled", err)
	}

	// A finite stream ends with nil.
	n = 0
	if err := c.Listen(context.Background(), func(map[string]string) error { n++; return nil }, "/test/stream"); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("got %d records, want 2", n)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		f.mu.Lock()
		canceled := len(f.canceled)
		f.mu.Unlock()
		if canceled == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("device received %d /cancel commands, want 2", canceled)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Connection still healthy.
	if _, err := c.Run(context.Background(), "/ip/address/print"); err != nil {
		t.Fatal(err)
	}
}

func TestReplyVariants(t *testing.T) {
	f := newFakeRouter()
	c := f.connect(t)
	ctx := context.Background()

	reply, err := c.Run(ctx, "/test/empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.Records) != 0 || reply.Records == nil {
		t.Errorf("records = %#v, want empty", reply.Records)
	}
	// The !done that follows !empty must not confuse later commands.
	if _, err := c.Run(ctx, "/ip/address/print"); err != nil {
		t.Fatal(err)
	}

	reply, err = c.Run(ctx, "/test/big")
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.Records[0]["data"]) != 70000 {
		t.Errorf("big word length = %d", len(reply.Records[0]["data"]))
	}

	long := strings.Repeat("y", 20000)
	if _, err := c.Query("/ip/address").Add(ctx, map[string]string{"comment": long}); err != nil {
		t.Fatal(err)
	}
	if got := f.records("/ip/address")[0]["comment"]; got != long {
		t.Errorf("long comment length = %d", len(got))
	}
}

func TestFatal(t *testing.T) {
	f := newFakeRouter()
	c := f.connect(t)

	_, err := c.Run(context.Background(), "/test/fatal")
	var de *DeviceError
	if !errors.As(err, &de) || !de.Fatal || de.Message != "session terminated on request" {
		t.Fatalf("err = %v, want fatal DeviceError", err)
	}
	if _, err := c.Run(context.Background(), "/ip/address/print"); err == nil {
		t.Error("expected error after !fatal")
	}
}

func TestLegacyLogin(t *testing.T) {
	f := newFakeRouter()
	f.legacyLogin = true
	c := f.connect(t)
	if _, err := c.Run(context.Background(), "/ip/address/print"); err != nil {
		t.Fatal(err)
	}

	clientConn, serverConn := net.Pipe()
	go f.serve(serverConn)
	if _, err := New(context.Background(), clientConn, Config{Username: "admin", Password: "bad"}); err == nil {
		t.Error("legacy login with wrong password should fail")
	}
}
