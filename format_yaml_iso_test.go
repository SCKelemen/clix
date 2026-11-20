package clix

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/iso8601"
)

// YAML writes durations as ISO 8601, like JSON output, wherever
// FormatData can reach them (not inside struct fields), and times in
// RFC 3339 with fractional seconds only when they are non-zero.
func TestFormatYAMLDurationsAndTimes(t *testing.T) {
	at := time.Date(2026, 11, 19, 8, 30, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		data interface{}
		want string
	}{
		{"durations", []time.Duration{0, 90 * time.Second, 1500 * time.Millisecond, -2 * time.Hour, 26 * time.Hour},
			"- PT0S\n- PT1M30S\n- PT1.5S\n- -PT2H\n- PT26H\n"},
		{"top level", 90 * time.Second, "PT1M30S\n"},
		{"in a map, by pointer", map[string]interface{}{"timeout": time.Minute, "p": ptrTo(2 * time.Second), "name": "x"},
			"name: x\np: PT2S\ntimeout: PT1M\n"},
		{"map of durations", map[string]time.Duration{"a": time.Hour}, "a: PT1H\n"},
		{"times", map[string]time.Time{"at": at, "frac": at.Add(500 * time.Millisecond), "zone": at.In(time.FixedZone("CEST", 2*3600))},
			"at: 2026-11-19T08:30:00Z\nfrac: 2026-11-19T08:30:00.5Z\nzone: 2026-11-19T10:30:00+02:00\n"},
		// Struct fields keep yaml's own encoding, which yaml.Unmarshal can
		// read back into a time.Duration field; see FormatData.
		{"struct field", struct{ Timeout time.Duration }{time.Minute}, "timeout: 1m0s\n"},
	} {
		if got := formatYAMLString(t, tc.data); got != tc.want {
			t.Errorf("%s: yaml %q, want %q", tc.name, got, tc.want)
		}
	}
}

func ptrTo[T any](v T) *T { return &v }

// JSON and YAML output format durations with iso8601.FormatDuration. Its
// output is identical to the formatter clix had before (legacyISODuration,
// kept below), which these tests compare it with.
func TestISODuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                            "PT0S",
		90 * time.Second:             "PT1M30S",
		1500 * time.Millisecond:      "PT1.5S",
		-2 * time.Hour:               "-PT2H",
		26 * time.Hour:               "PT26H",
		time.Hour + time.Second:      "PT1H1S",
		time.Nanosecond:              "PT0.000000001S",
		-time.Millisecond:            "-PT0.001S",
		time.Duration(math.MaxInt64): "PT2562047H47M16.854775807S",
		time.Duration(math.MinInt64): "-PT2562047H47M16.854775808S",
		2*time.Hour + 3*time.Minute + 40*time.Microsecond: "PT2H3M0.00004S",
	} {
		if got := iso8601.FormatDuration(d); got != want {
			t.Errorf("iso8601.FormatDuration(%v) = %q, want %q", d, got, want)
		}
		if got := legacyISODuration(d); got != want {
			t.Errorf("legacyISODuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestISODurationMatchesLegacy(t *testing.T) {
	check := func(d time.Duration) {
		t.Helper()
		if got, want := iso8601.FormatDuration(d), legacyISODuration(d); got != want {
			t.Fatalf("iso8601.FormatDuration(%d) = %q, legacy %q", int64(d), got, want)
		}
	}
	// Every combination of a few hours, minutes, seconds and fractions
	// (each with and without trailing zeros), both signs.
	for _, h := range []time.Duration{0, 1, 23, 24, 25, 2562047} {
		for _, m := range []time.Duration{0, 1, 59} {
			for _, s := range []time.Duration{0, 1, 59} {
				for _, ns := range []time.Duration{0, 1, 10, 100000000, 500000000, 999999999, 40000, 123456789} {
					d := h*time.Hour + m*time.Minute + s*time.Second + ns
					check(d)
					check(-d)
				}
			}
		}
	}
	// Powers of two and ten, and their neighbors, up to the limits.
	for v := int64(1); v > 0 && v <= math.MaxInt64/2; v *= 2 {
		for _, d := range []int64{v - 1, v, v + 1} {
			check(time.Duration(d))
			check(time.Duration(-d))
		}
	}
	for v := int64(1); v <= math.MaxInt64/10; v *= 10 {
		for _, d := range []int64{v - 1, v, v + 1} {
			check(time.Duration(d))
			check(time.Duration(-d))
		}
	}
	check(math.MaxInt64)
	check(math.MinInt64)
	check(math.MinInt64 + 1)
	// Random durations of every magnitude.
	r := rand.New(rand.NewPCG(5, 6))
	for range 500000 {
		d := time.Duration(r.Int64() >> r.IntN(64))
		if r.IntN(2) == 0 {
			d = -d
		}
		check(d)
	}
}

// legacyISODuration is the duration formatter of clix v2.8.0.
func legacyISODuration(d time.Duration) string {
	if d == 0 {
		return "PT0S"
	}
	var b strings.Builder
	u := uint64(d)
	if d < 0 {
		b.WriteByte('-')
		u = -u // also right for math.MinInt64
	}
	b.WriteString("PT")
	h := u / uint64(time.Hour)
	u -= h * uint64(time.Hour)
	m := u / uint64(time.Minute)
	u -= m * uint64(time.Minute)
	sec, frac := u/uint64(time.Second), u%uint64(time.Second)
	if h > 0 {
		b.WriteString(strconv.FormatUint(h, 10) + "H")
	}
	if m > 0 {
		b.WriteString(strconv.FormatUint(m, 10) + "M")
	}
	if sec > 0 || frac > 0 {
		b.WriteString(strconv.FormatUint(sec, 10))
		if frac > 0 {
			b.WriteString("." + strings.TrimRight(fmt.Sprintf("%09d", frac), "0"))
		}
		b.WriteString("S")
	}
	return b.String()
}

func FuzzISODurationMatchesLegacy(f *testing.F) {
	for _, d := range []int64{0, 1, -1, 90e9, math.MaxInt64, math.MinInt64} {
		f.Add(d)
	}
	f.Fuzz(func(t *testing.T, d int64) {
		if got, want := iso8601.FormatDuration(time.Duration(d)), legacyISODuration(time.Duration(d)); got != want {
			t.Fatalf("iso8601.FormatDuration(%d) = %q, legacy %q", d, got, want)
		}
	})
}
