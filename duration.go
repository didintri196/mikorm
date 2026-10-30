package mikorm

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	day  = 24 * time.Hour
	week = 7 * day
)

// ParseDuration parses a RouterOS time value such as "1w2d3h4m5s",
// "500ms", "1d00:05:00" or "00:00:30". A bare number is read as seconds
// and an empty string as zero.
func ParseDuration(s string) (time.Duration, error) {
	orig := s
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")

	var total time.Duration
	for s != "" {
		// Clock form "hh:mm:ss[.frac]", which is always the last component.
		if i := strings.IndexByte(s, ':'); i >= 0 && isNumber(s[:i]) {
			d, err := parseClock(s)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q", orig)
			}
			total += d
			break
		}

		i := strings.IndexFunc(s, func(r rune) bool { return (r < '0' || r > '9') && r != '.' })
		if i == 0 {
			return 0, fmt.Errorf("invalid duration %q", orig)
		}
		if i < 0 {
			i = len(s)
		}
		num := s[:i]
		s = s[i:]

		j := strings.IndexFunc(s, func(r rune) bool { return r >= '0' && r <= '9' })
		if j < 0 {
			j = len(s)
		}
		unit, ok := durationUnits[s[:j]]
		if !ok {
			return 0, fmt.Errorf("invalid duration %q: unknown unit %q", orig, s[:j])
		}
		s = s[j:]

		if strings.Contains(num, ".") {
			f, err := strconv.ParseFloat(num, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q", orig)
			}
			total += time.Duration(f * float64(unit))
		} else {
			n, err := strconv.ParseInt(num, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid duration %q", orig)
			}
			total += time.Duration(n) * unit
		}
	}
	if neg {
		total = -total
	}
	return total, nil
}

var durationUnits = map[string]time.Duration{
	"":   time.Second,
	"w":  week,
	"d":  day,
	"h":  time.Hour,
	"m":  time.Minute,
	"s":  time.Second,
	"ms": time.Millisecond,
	"us": time.Microsecond,
	"ns": time.Nanosecond,
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseClock(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 || !isNumber(parts[0]) || !isNumber(parts[1]) {
		return 0, fmt.Errorf("invalid clock %q", s)
	}
	h, _ := strconv.ParseInt(parts[0], 10, 64)
	m, _ := strconv.ParseInt(parts[1], 10, 64)
	sec, err := strconv.ParseFloat(parts[2], 64)
	if err != nil || strings.HasPrefix(parts[2], "-") || strings.HasPrefix(parts[2], "+") {
		return 0, fmt.Errorf("invalid clock %q", s)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec*float64(time.Second)), nil
}

// FormatDuration formats d in RouterOS notation, e.g. "1d2h3m4s".
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	var b strings.Builder
	if d < 0 {
		b.WriteByte('-')
		d = -d
	}
	for _, u := range []struct {
		suffix string
		unit   time.Duration
	}{
		{"w", week},
		{"d", day},
		{"h", time.Hour},
		{"m", time.Minute},
		{"s", time.Second},
		{"ms", time.Millisecond},
		{"us", time.Microsecond},
		{"ns", time.Nanosecond},
	} {
		if n := d / u.unit; n > 0 {
			b.WriteString(strconv.FormatInt(int64(n), 10))
			b.WriteString(u.suffix)
			d -= n * u.unit
		}
	}
	return b.String()
}
