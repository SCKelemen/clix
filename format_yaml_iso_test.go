package clix

import (
	"math"
	"testing"
	"time"
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
		// v2 keeps yaml's own encoding of struct fields; see the v3 list.
		{"struct field", struct{ Timeout time.Duration }{time.Minute}, "timeout: 1m0s\n"},
	} {
		if got := formatYAMLString(t, tc.data); got != tc.want {
			t.Errorf("%s: yaml %q, want %q", tc.name, got, tc.want)
		}
	}
}

func ptrTo[T any](v T) *T { return &v }

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
		if got := isoDuration(d); got != want {
			t.Errorf("isoDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
