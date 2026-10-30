package mikorm

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeRouter is a tiny in-memory RouterOS API server used by the tests. It
// speaks the real wire protocol and implements print (with queries,
// .proplist and count-only), add, set, remove, enable and disable.
type fakeRouter struct {
	username, password string

	mu      sync.Mutex
	tables  map[string][]map[string]string
	nextID  int
	history [][]string

	// legacyLogin enables the pre-6.43 challenge/response login.
	legacyLogin bool

	// release unblocks "/test/slow" commands.
	release chan struct{}

	// listening maps tags of running "/test/listen" commands to a channel
	// closed by "/cancel".
	listening map[string]chan struct{}
	canceled  []string
}

func newFakeRouter() *fakeRouter {
	return &fakeRouter{
		username: "admin",
		password: "secret",
		tables:   map[string][]map[string]string{},
		release:  make(chan struct{}),

		listening: map[string]chan struct{}{},
	}
}

func (f *fakeRouter) seed(path string, records ...map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range records {
		f.nextID++
		r[".id"] = "*" + strconv.FormatInt(int64(f.nextID), 16)
		f.tables[path] = append(f.tables[path], r)
	}
}

func (f *fakeRouter) records(path string) []map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]string(nil), f.tables[path]...)
}

func (f *fakeRouter) lastCommand() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.history) == 0 {
		return nil
	}
	return f.history[len(f.history)-1]
}

// connect returns a logged in Client connected to f over an in-memory pipe.
func (f *fakeRouter) connect(t *testing.T) *Client {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	go f.serve(serverConn)
	c, err := New(context.Background(), clientConn, Config{Username: f.username, Password: f.password})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func (f *fakeRouter) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := &syncWriter{w: bufio.NewWriter(conn)}
	for {
		words, err := readWords(r)
		if err != nil {
			return
		}
		var tag string
		args := words[:0:0]
		for _, word := range words {
			if strings.HasPrefix(word, ".tag=") {
				tag = word[len(".tag="):]
				continue
			}
			args = append(args, word)
		}
		if len(args) == 0 {
			continue
		}

		if args[0] == "/test/slow" {
			go func() {
				<-f.release
				writeSentences(w, tag, [][]string{{"!done"}})
			}()
			continue
		}
		switch args[0] {
		case "/test/hangup":
			return
		case "/test/fatal":
			writeSentences(w, "", [][]string{{"!fatal", "session terminated on request"}})
			return
		case "/test/empty":
			writeSentences(w, tag, [][]string{{"!empty"}, {"!done"}})
			continue
		case "/test/big":
			writeSentences(w, tag, [][]string{{"!re", "=data=" + strings.Repeat("x", 70000)}, {"!done"}})
			continue
		case "/test/listen":
			stop := make(chan struct{})
			f.mu.Lock()
			f.listening[tag] = stop
			f.mu.Unlock()
			go func() {
				for i := 0; i < 3; i++ {
					writeSentences(w, tag, [][]string{{"!re", "=n=" + strconv.Itoa(i)}})
				}
				<-stop
				writeSentences(w, tag, [][]string{{"!trap", "=category=2", "=message=interrupted"}, {"!done"}})
			}()
			continue
		case "/test/stream":
			writeSentences(w, tag, [][]string{{"!re", "=n=0"}, {"!re", "=n=1"}, {"!done"}})
			continue
		case "/cancel":
			target := strings.TrimPrefix(args[1], "=tag=")
			f.mu.Lock()
			f.canceled = append(f.canceled, target)
			if stop, ok := f.listening[target]; ok {
				close(stop)
				delete(f.listening, target)
			}
			f.mu.Unlock()
			writeSentences(w, tag, [][]string{{"!done"}})
			continue
		}
		writeSentences(w, tag, f.handle(args))
	}
}

type syncWriter struct {
	mu sync.Mutex
	w  *bufio.Writer
}

func writeSentences(w *syncWriter, tag string, sentences [][]string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, s := range sentences {
		if err := writeSentence(w.w, s, tag); err != nil {
			return
		}
	}
}

func trap(msg string) [][]string {
	// Like RouterOS, a !trap is followed by a !done.
	return [][]string{{"!trap", "=message=" + msg}, {"!done"}}
}

func (f *fakeRouter) handle(words []string) [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.history = append(f.history, append([]string(nil), words...))

	params := map[string]string{}
	var queries []string
	for _, w := range words[1:] {
		switch {
		case strings.HasPrefix(w, "="):
			k, v, _ := strings.Cut(w[1:], "=")
			params[k] = v
		case strings.HasPrefix(w, "?"):
			queries = append(queries, w[1:])
		}
	}

	if words[0] == "/login" && f.legacyLogin {
		const challenge = "0123456789abcdef0123456789abcdef"
		if _, ok := params["response"]; !ok {
			return [][]string{{"!done", "=ret=" + challenge}}
		}
		b, _ := hex.DecodeString(challenge)
		h := md5.New()
		h.Write([]byte{0})
		h.Write([]byte(f.password))
		h.Write(b)
		if params["name"] == f.username && params["response"] == "00"+hex.EncodeToString(h.Sum(nil)) {
			return [][]string{{"!done"}}
		}
		return trap("invalid user name or password (6)")
	}
	if words[0] == "/login" {
		if params["name"] == f.username && params["password"] == f.password {
			return [][]string{{"!done"}}
		}
		return trap("invalid user name or password (6)")
	}

	i := strings.LastIndexByte(words[0], '/')
	path, cmd := words[0][:i], words[0][i+1:]
	if path == "/fail" {
		return trap("failure: test error")
	}

	switch cmd {
	case "print":
		var out [][]string
		count := 0
		for _, rec := range f.tables[path] {
			if !matchQuery(rec, queries) {
				continue
			}
			count++
			sen := []string{"!re"}
			keys := make([]string, 0, len(rec))
			for k := range rec {
				keys = append(keys, k)
			}
			if pl, ok := params[".proplist"]; ok {
				keys = strings.Split(pl, ",")
			}
			sort.Strings(keys)
			for _, k := range keys {
				if v, ok := rec[k]; ok {
					sen = append(sen, "="+k+"="+v)
				}
			}
			out = append(out, sen)
		}
		if _, ok := params["count-only"]; ok {
			return [][]string{{"!done", "=ret=" + strconv.Itoa(count)}}
		}
		return append(out, []string{"!done"})

	case "add":
		if _, ok := params[".id"]; ok {
			return trap("unknown parameter .id")
		}
		f.nextID++
		id := "*" + strconv.FormatInt(int64(f.nextID), 16)
		rec := map[string]string{".id": id}
		for k, v := range params {
			rec[k] = v
		}
		f.tables[path] = append(f.tables[path], rec)
		return [][]string{{"!done", "=ret=" + id}}

	case "set", "remove", "enable", "disable":
		ids := strings.Split(params[".id"], ",")
		recs := f.tables[path]
		for _, id := range ids {
			idx := -1
			for i, rec := range recs {
				if rec[".id"] == id {
					idx = i
				}
			}
			if idx < 0 {
				return trap("no such item")
			}
			switch cmd {
			case "set":
				for k, v := range params {
					if k != ".id" {
						recs[idx][k] = v
					}
				}
			case "remove":
				recs = append(recs[:idx], recs[idx+1:]...)
			case "enable":
				recs[idx]["disabled"] = "false"
			case "disable":
				recs[idx]["disabled"] = "true"
			}
		}
		f.tables[path] = recs
		return [][]string{{"!done"}}
	}
	return trap("no such command")
}

// matchQuery evaluates RouterOS API query words against rec.
func matchQuery(rec map[string]string, queries []string) bool {
	var st []bool
	pop := func() bool {
		v := st[len(st)-1]
		st = st[:len(st)-1]
		return v
	}
	for _, q := range queries {
		switch {
		case strings.HasPrefix(q, "#"):
			for _, op := range q[1:] {
				switch op {
				case '!':
					st = append(st, !pop())
				case '|':
					a, b := pop(), pop()
					st = append(st, a || b)
				case '&':
					a, b := pop(), pop()
					st = append(st, a && b)
				}
			}
		case strings.HasPrefix(q, "-"):
			_, ok := rec[q[1:]]
			st = append(st, !ok)
		case strings.HasPrefix(q, "<"), strings.HasPrefix(q, ">"):
			k, v, _ := strings.Cut(q[1:], "=")
			a, _ := strconv.Atoi(rec[k])
			b, _ := strconv.Atoi(v)
			if q[0] == '<' {
				st = append(st, a < b)
			} else {
				st = append(st, a > b)
			}
		default:
			k, v, hasValue := strings.Cut(q, "=")
			got, ok := rec[k]
			st = append(st, ok && (!hasValue || got == v))
		}
	}
	for _, v := range st {
		if !v {
			return false
		}
	}
	return true
}
