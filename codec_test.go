package mikorm

import (
	"reflect"
	"testing"
	"time"
)

func TestKebabCase(t *testing.T) {
	tests := map[string]string{
		"Name":       "name",
		"ID":         "id",
		"MACAddress": "mac-address",
		"RxByte":     "rx-byte",
		"LeaseTime":  "lease-time",
		"IPv6":       "i-pv6",
		"Mtu1":       "mtu1",
		"L2MTU":      "l2-mtu",
		"UsePeer":    "use-peer",
	}
	for in, want := range tests {
		if got := kebabCase(in); got != want {
			t.Errorf("kebabCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEncodeValues(t *testing.T) {
	yes := true
	type inner struct {
		Comment string `mikorm:"comment"`
	}
	type model struct {
		inner
		ID       string        `mikorm:".id"`
		Name     string        `mikorm:"name"`
		Running  bool          `mikorm:"running,readonly"`
		MTU      uint16        `mikorm:"mtu"`
		Timeout  time.Duration `mikorm:"timeout"`
		Ratio    float64       `mikorm:"ratio"`
		Disabled *bool         `mikorm:"disabled"`
		Ports    []string      `mikorm:"ports"`
		Empty    string        `mikorm:"empty"`
		Skip     string        `mikorm:"-"`
	}
	m := model{
		inner:    inner{Comment: "c"},
		ID:       "*1",
		Name:     "n",
		Running:  true,
		MTU:      1500,
		Timeout:  90 * time.Second,
		Ratio:    0.5,
		Disabled: &yes,
		Ports:    []string{"80", "443"},
		Skip:     "x",
	}

	got, err := encodeValues(m, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []pair{
		{"comment", "c"}, {"name", "n"}, {"mtu", "1500"}, {"timeout", "1m30s"},
		{"ratio", "0.5"}, {"disabled", "true"}, {"ports", "80,443"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("write encode = %v, want %v", got, want)
	}

	got, err = encodeValues(&m, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want)+2 { // .id and running are included in queries
		t.Errorf("query encode = %v", got)
	}

	got, err = encodeValues(map[string]any{"b": 2, "a": true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := []pair{{"a", "true"}, {"b", "2"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("map encode = %v, want %v", got, want)
	}

	if got, err := encodeValues(nil, true); err != nil || got != nil {
		t.Errorf("nil encode = %v, %v", got, err)
	}
	if _, err := encodeValues(map[int]string{1: "a"}, true); err == nil {
		t.Error("expected error for non-string map key")
	}
}

func TestDuration(t *testing.T) {
	parse := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"30", 30 * time.Second},
		{"5s", 5 * time.Second},
		{"500ms", 500 * time.Millisecond},
		{"1.5s", 1500 * time.Millisecond},
		{"1h30m", 90 * time.Minute},
		{"1w2d3h4m5s", week + 2*day + 3*time.Hour + 4*time.Minute + 5*time.Second},
		{"00:05:00", 5 * time.Minute},
		{"1d00:00:10", day + 10*time.Second},
		{"-10s", -10 * time.Second},
	}
	for _, tt := range parse {
		got, err := ParseDuration(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("ParseDuration(%q) = %v, %v, want %v", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{"abc", "1x", "1:2", "s", "1h:00"} {
		if _, err := ParseDuration(bad); err == nil {
			t.Errorf("ParseDuration(%q) succeeded", bad)
		}
	}

	format := map[time.Duration]string{
		0:                                 "0s",
		90 * time.Second:                  "1m30s",
		week + day + time.Hour:            "1w1d1h",
		1500 * time.Millisecond:           "1s500ms",
		-time.Minute:                      "-1m",
		3*day + 4*time.Hour + time.Second: "3d4h1s",
	}
	for in, want := range format {
		if got := FormatDuration(in); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", in, got, want)
		}
		if back, err := ParseDuration(want); err != nil || back != in {
			t.Errorf("round trip %q = %v, %v", want, back, err)
		}
	}
}

func TestRedact(t *testing.T) {
	got := redact([]string{"=name=bob", "=password=hunter2", "=Secret=x", "?comment"})
	want := []string{"=name=bob", "=password=***", "=Secret=***", "?comment"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("redact = %v, want %v", got, want)
	}
}
