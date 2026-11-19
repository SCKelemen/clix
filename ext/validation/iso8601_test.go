package validation

import (
	"math"
	"testing"
	"time"
)

func TestISO8601Time(t *testing.T) {
	for v, ok := range map[string]bool{
		"2026-11-19T08:30:00Z":                true,
		"2026-11-19T08:30:00.5Z":              true,
		"2026-11-19T10:30:00+02:00":           true,
		"2026-11-19T10:30:00.123456789-05:30": true,
		"":                                    false,
		"2026-11-19":                          false,
		"2026-11-19T08:30:00":                 false, // no time zone
		"2026-11-19 08:30:00Z":                false,
		"2026-13-19T08:30:00Z":                false,
		"2026-11-19T25:30:00Z":                false,
		"yesterday":                           false,
	} {
		if err := ISO8601Time(v); (err == nil) != ok {
			t.Errorf("ISO8601Time(%q) = %v, want ok = %v", v, err, ok)
		}
	}
}

func TestISO8601Duration(t *testing.T) {
	valid := map[string]time.Duration{
		"PT0S":                        0,
		"PT1M30S":                     90 * time.Second,
		"PT1.5S":                      1500 * time.Millisecond,
		"PT1,5S":                      1500 * time.Millisecond,
		"-PT2H":                       -2 * time.Hour,
		"+PT2H":                       2 * time.Hour,
		"PT26H":                       26 * time.Hour,
		"P1D":                         24 * time.Hour,
		"P1DT12H":                     36 * time.Hour,
		"P0D":                         0,
		"PT1H1M1S":                    time.Hour + time.Minute + time.Second,
		"PT90M":                       90 * time.Minute,
		"PT0.000000001S":              time.Nanosecond,
		"PT0.0000000019S":             time.Nanosecond, // digits beyond nanoseconds are dropped
		"PT2562047H47M16.854775807S":  math.MaxInt64,
		"-PT2562047H47M16.854775808S": math.MinInt64,
	}
	for v, want := range valid {
		if err := ISO8601Duration(v); err != nil {
			t.Errorf("ISO8601Duration(%q) = %v", v, err)
		}
		if got, err := parseISODuration(v); err != nil || got != want {
			t.Errorf("parseISODuration(%q) = %v, %v; want %v", v, got, err, want)
		}
	}
	for _, v := range []string{
		"", "P", "PT", "-P", "P1DT", "1H", "T1H", "pt1h", "PT1h",
		"P1Y", "P1M", "P1W", "P1Y2M", // calendar units
		"PT1.5M", "PT1.5H", // fraction only on seconds
		"PT1S1M", "PT1H1H", "PT1M1H", // order and repetition
		"PT1", "PT.5S", "PT1.S", "PT1..5S", "PT-1S", "P-1D", "PT 1S", "PT1S ",
		"PT2562047H47M16.854775808S", "P106752D", "PT99999999999999999999S",
	} {
		if err := ISO8601Duration(v); err == nil {
			t.Errorf("ISO8601Duration(%q) = nil", v)
		}
	}
}
