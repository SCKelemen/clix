package validation

import (
	"math"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/iso8601"
)

// The accepted set of ISO8601Time before (v2.8.0, time.Parse with
// time.RFC3339) and after (RFC 3339 through the iso8601 package). The
// only differences are the four forms time.Parse accepts that are not
// RFC 3339.
func TestISO8601TimeAcceptedSet(t *testing.T) {
	for _, tc := range []struct {
		value         string
		before, after bool
	}{
		{"2026-11-19T08:30:00Z", true, true},
		{"2026-11-19T08:30:00.5Z", true, true},
		{"2026-11-19T10:30:00+02:00", true, true},
		{"2026-11-19T10:30:00.123456789-05:30", true, true},
		{"2026-11-19T08:30:00.1234567891Z", true, true}, // digits past nanoseconds are dropped
		{"2026-11-19T08:30:00+00:00", true, true},
		{"2026-11-19T08:30:00-00:00", true, true},
		{"2026-11-19T08:30:00+23:59", true, true},
		{"2024-02-29T08:30:00Z", true, true},
		{"0000-01-01T00:00:00Z", true, true},
		{"9999-12-31T23:59:59Z", true, true},

		// Not RFC 3339, but accepted by time.Parse (go.dev/issue/54580).
		{"2026-11-19T8:30:00Z", true, false},       // one-digit hour
		{"2026-11-19T08:30:00,5Z", true, false},    // comma before the fraction
		{"2026-11-19T08:30:00+24:00", true, false}, // offset hour 24
		{"2026-11-19T08:30:00+23:60", true, false}, // offset minute 60

		{"", false, false},
		{"yesterday", false, false},
		{"2026-11-19", false, false},
		{"2026-11-19T08:30:00", false, false}, // no time zone
		{"2026-11-19T08:30Z", false, false},   // no seconds
		{"2026-11-19 08:30:00Z", false, false},
		{"2026-11-19t08:30:00Z", false, false},
		{"2026-11-19T08:30:00z", false, false},
		{"2026-13-19T08:30:00Z", false, false},
		{"2026-02-29T08:30:00Z", false, false},
		{"2026-11-19T25:30:00Z", false, false},
		{"2026-11-19T24:00:00Z", false, false},
		{"2016-12-31T23:59:60Z", false, false}, // a leap second
		{"2026-11-19T08:60:00Z", false, false},
		{"2026-11-19T08:30:00.Z", false, false},
		{"2026-11-19T08:30:00+0200", false, false},
		{"2026-11-19T08:30:00+02", false, false},
		{"2026-11-19T08:30:00+25:00", false, false},
		{"2026-11-19T08:30:00+2:00", false, false},
		{"2026-11-19T08:30:00 Z", false, false},
		{"2026-11-19T08:30:00Z ", false, false},
		{"20261119T083000Z", false, false},     // basic format
		{"2026-W47-4T08:30:00Z", false, false}, // week date
		{"2026-323T08:30:00Z", false, false},   // ordinal date
		{"+2026-11-19T08:30:00Z", false, false},
	} {
		if got := legacyISO8601Time(tc.value) == nil; got != tc.before {
			t.Errorf("v2.8.0 ISO8601Time(%q) ok = %v, want %v", tc.value, got, tc.before)
		}
		if got := ISO8601Time(tc.value) == nil; got != tc.after {
			t.Errorf("ISO8601Time(%q) = %v, want ok = %v", tc.value, ISO8601Time(tc.value), tc.after)
		}
	}
}

func TestISO8601TimeMessages(t *testing.T) {
	for value, want := range map[string]string{
		"":                          "time cannot be empty",
		"2026-11-19T08:30:00":       `must be an RFC 3339 time such as "2026-11-19T08:30:00Z": it has no time zone (add Z or an offset such as +02:00)`,
		"2026-11-19":                `must be an RFC 3339 time such as "2026-11-19T08:30:00Z": it ends before the seconds (YYYY-MM-DDThh:mm:ss)`,
		"2026-11-19T8:30:00Z":       `must be an RFC 3339 time such as "2026-11-19T08:30:00Z": expected YYYY-MM-DDThh:mm:ss, found ':' at offset 12`,
		"2026-13-19T08:30:00Z":      `must be an RFC 3339 time such as "2026-11-19T08:30:00Z": iso8601: invalid time "2026-13-19T08:30:00Z" at offset 5: month 13 is out of range 01-12`,
		"2026-11-19T08:30:00+24:00": `must be an RFC 3339 time such as "2026-11-19T08:30:00Z": iso8601: invalid time "2026-11-19T08:30:00+24:00" at offset 20: offset hour 24 is out of range 00-23`,
		"2026-11-19T24:00:00Z":      `must be an RFC 3339 time such as "2026-11-19T08:30:00Z": hour 24 is out of range 00-23`,
		"2016-12-31T23:59:60Z":      `must be an RFC 3339 time such as "2026-11-19T08:30:00Z": a leap second (second 60) is not supported`,
	} {
		if err := ISO8601Time(value); err == nil || err.Error() != want {
			t.Errorf("ISO8601Time(%q) = %v, want %q", value, err, want)
		}
	}
}

// goLeniency reports whether v has one of the four non-RFC 3339 forms that
// time.Parse(time.RFC3339, v) accepts.
func goLeniency(v string) bool {
	if len(v) > 12 && v[12] == ':' || strings.Contains(v, ",") {
		return true
	}
	if n := len(v); n >= 6 && (v[n-6] == '+' || v[n-6] == '-') {
		return v[n-5:n-3] == "24" || v[n-2:] == "60"
	}
	return false
}

// checkTime compares ISO8601Time with v2.8.0's on v: anything accepted now
// was accepted before and reads as the same instant with time.Parse and
// iso8601.ParseTime, and anything accepted before but not now is one of
// time.Parse's leniencies.
func checkTime(t *testing.T, v string) {
	t.Helper()
	before, after := legacyISO8601Time(v) == nil, ISO8601Time(v) == nil
	switch {
	case after && !before:
		t.Errorf("ISO8601Time(%q) accepts what v2.8.0 rejected", v)
	case before && !after && !goLeniency(v):
		t.Errorf("ISO8601Time(%q) = %v, but v2.8.0 accepted it", v, ISO8601Time(v))
	case after:
		a, err1 := time.Parse(time.RFC3339, v)
		b, err2 := iso8601.ParseTime(v, nil)
		if err1 != nil || err2 != nil || !a.Equal(b) {
			t.Errorf("%q: time.Parse = %v, %v; iso8601.ParseTime = %v, %v", v, a, err1, b, err2)
		}
	}
}

// Every combination of these fields, sampled, and with each field varied
// alone.
func TestISO8601TimeMatchesLegacy(t *testing.T) {
	fields := [][]string{
		{"2026", "2024", "2100", "2000", "0000", "9999", "202", "20261", "2O26"},
		{"-"},
		{"01", "02", "04", "11", "12", "00", "13", "1", "1a"},
		{"-"},
		{"01", "19", "28", "29", "30", "31", "00", "32", "9"},
		{"T", "t", " ", "", "TT", "Z"},
		{"00", "08", "23", "24", "25", "8", "0", "99"},
		{":"},
		{"00", "30", "59", "60", "5", ""},
		{":", "", "::"},
		{"00", "59", "60", "61", "5", ""},
		{"", ".5", ",5", ".", ",", ".123456789", ".1234567891234", "..5", ".5,5", ".0"},
		{"Z", "z", "", " Z", "+00:00", "-00:00", "+02:00", "-05:30", "+23:59", "+24:00", "+23:60", "+24:60", "+25:00", "-24:00", "+0200", "+02", "+2:00", "+02:0", "+02:00:00", "Z0", "ZZ"},
	}
	valid := []int{0, 0, 3, 0, 1, 0, 1, 0, 1, 0, 0, 0, 0} // indexes of a valid value per field
	build := func(pick []int) string {
		var b strings.Builder
		for i, f := range fields {
			b.WriteString(f[pick[i]])
		}
		return b.String()
	}
	// Each field varied alone, and each pair of fields.
	for i := range fields {
		for j := range fields {
			for a := range fields[i] {
				for b := range fields[j] {
					pick := append([]int(nil), valid...)
					pick[i], pick[j] = a, b
					checkTime(t, build(pick))
				}
			}
		}
	}
	// Random combinations of every field.
	r := rand.New(rand.NewPCG(1, 2))
	pick := make([]int, len(fields))
	for range 200000 {
		for i, f := range fields {
			pick[i] = r.IntN(len(f))
		}
		checkTime(t, build(pick))
	}
}

func FuzzISO8601TimeMatchesLegacy(f *testing.F) {
	for _, s := range []string{"2026-11-19T08:30:00Z", "2026-11-19T8:30:00,5+24:00", "2016-12-31T23:59:60Z", "2026-11-19T24:00:00-00:00", "20261119T083000Z"} {
		f.Add(s)
	}
	f.Fuzz(checkTime)
}

// The accepted set of ISO8601Duration is unchanged.
func TestISO8601Duration(t *testing.T) {
	valid := map[string]time.Duration{
		"PT0S":                         0,
		"PT1M30S":                      90 * time.Second,
		"PT1.5S":                       1500 * time.Millisecond,
		"PT1,5S":                       1500 * time.Millisecond,
		"-PT2H":                        -2 * time.Hour,
		"+PT2H":                        2 * time.Hour,
		"PT26H":                        26 * time.Hour,
		"P1D":                          24 * time.Hour,
		"P1DT12H":                      36 * time.Hour,
		"P0D":                          0,
		"-P0D":                         0,
		"PT1H1M1S":                     time.Hour + time.Minute + time.Second,
		"PT1H30S":                      time.Hour + 30*time.Second,
		"P1DT0.5S":                     24*time.Hour + 500*time.Millisecond,
		"PT90M":                        90 * time.Minute,
		"PT0001H":                      time.Hour,
		"PT0.000000001S":               time.Nanosecond,
		"PT0.0000000019S":              time.Nanosecond, // digits beyond nanoseconds are dropped
		"PT2562047H47M16.854775807S":   math.MaxInt64,
		"-PT2562047H47M16.854775808S":  math.MinInt64,
		"P106751DT23H47M16.854775807S": math.MaxInt64,
	}
	for v, want := range valid {
		if err := ISO8601Duration(v); err != nil {
			t.Errorf("ISO8601Duration(%q) = %v", v, err)
		}
		if got, err := parseISODuration(v); err != nil || got != want {
			t.Errorf("parseISODuration(%q) = %v, %v; want %v", v, got, err, want)
		}
		if got, err := legacyParseISODuration(v); err != nil || got != want {
			t.Errorf("v2.8.0 parseISODuration(%q) = %v, %v; want %v", v, got, err, want)
		}
	}
	for _, v := range []string{
		"", "P", "PT", "-P", "+P", "+", "P1DT", "1H", "T1H", "pt1h", "PT1h",
		"P1Y", "P1M", "P1W", "P1Y2M", "P0Y1D", "P0W", "P1W2D", // calendar units, even zero
		"PT1.5M", "PT1.5H", "P1.5D", "P1,5D", "P0.5W", // fraction only on seconds
		"PT1S1M", "PT1H1H", "PT1M1H", "P1D1D", "PT1HT1M", // order and repetition
		"PT1", "PT.5S", "PT1.S", "PT1..5S", "PT1.5,5S", "PT-1S", "P-1D", "PT 1S", "PT1S ",
		"--PT1S", "+-PT1S", "-+PT1S", "++PT1S", "+ PT1S",
		"PT2562047H47M16.854775808S", "P106752D", "PT99999999999999999999S", "P106751DT23H47M16.854775808S",
	} {
		if err := ISO8601Duration(v); err == nil {
			t.Errorf("ISO8601Duration(%q) = nil", v)
		}
		if _, err := legacyParseISODuration(v); v != "" && err == nil {
			t.Errorf("v2.8.0 parseISODuration(%q) = nil", v)
		}
	}
}

func TestISO8601DurationMessages(t *testing.T) {
	for value, want := range map[string]string{
		"":         "duration cannot be empty",
		"P1M":      `must be an ISO 8601 duration such as "PT1M30S": years, months and weeks are not supported, since their length depends on the calendar`,
		"PT1.5M":   `must be an ISO 8601 duration such as "PT1M30S": only seconds may have a fraction`,
		"P1X":      `must be an ISO 8601 duration such as "PT1M30S": iso8601: invalid duration "P1X" at offset 2: expected designator Y, M, W or D, found 'X'`,
		"+PT1X":    `must be an ISO 8601 duration such as "PT1M30S": iso8601: invalid duration "+PT1X" at offset 4: expected designator H, M or S, found 'X'`,
		"+-PT1S":   `must be an ISO 8601 duration such as "PT1M30S": "+" must be followed by "P"`,
		"P106752D": `must be an ISO 8601 duration such as "PT1M30S": it is out of range`,
	} {
		if err := ISO8601Duration(value); err == nil || err.Error() != want {
			t.Errorf("ISO8601Duration(%q) = %v, want %q", value, err, want)
		}
	}
}

// checkDuration compares ISO8601Duration with v2.8.0's on v: both accept
// or both reject, and accepted values are equal.
func checkDuration(t *testing.T, v string) {
	t.Helper()
	before, errBefore := legacyParseISODuration(v)
	after, errAfter := parseISODuration(v)
	if (errBefore == nil) != (errAfter == nil) || before != after {
		t.Errorf("%q: parseISODuration = %v, %v; v2.8.0 = %v, %v", v, after, errAfter, before, errBefore)
	}
	if (ISO8601Duration(v) == nil) != (errAfter == nil) {
		t.Errorf("%q: ISO8601Duration and parseISODuration disagree", v)
	}
}

// Every string of up to five characters over the duration alphabet.
func TestISO8601DurationMatchesLegacyExhaustively(t *testing.T) {
	const alphabet = "PTYMWDHS019.,+-"
	buf := make([]byte, 0, 5)
	var walk func()
	walk = func() {
		checkDuration(t, string(buf))
		if len(buf) == cap(buf) || t.Failed() {
			return
		}
		for i := range len(alphabet) {
			buf = append(buf, alphabet[i])
			walk()
			buf = buf[:len(buf)-1]
		}
	}
	walk()
}

// Random durations built from components, including long numbers near
// the limits of time.Duration.
func TestISO8601DurationMatchesLegacy(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	number := func() string {
		switch r.IntN(6) {
		case 0:
			return ""
		case 1:
			return strings.Repeat("0", r.IntN(30)) + "1"
		case 2:
			return []string{"106751", "106752", "2562047", "2562048", "153722867", "153722868", "9223372036", "9223372037", "18446744073709551616"}[r.IntN(9)]
		}
		var b strings.Builder
		for range 1 + r.IntN(20) {
			b.WriteByte(byte('0' + r.IntN(10)))
		}
		return b.String()
	}
	fraction := func() string {
		switch r.IntN(5) {
		case 0:
			return "." + number()
		case 1:
			return "," + number()
		case 2:
			return []string{".", "..5", ".5.5", ".854775807", ".854775808", ".8547758079"}[r.IntN(6)]
		}
		return ""
	}
	for range 300000 {
		var b strings.Builder
		b.WriteString([]string{"", "", "", "-", "+", "--", "+-", " "}[r.IntN(8)])
		b.WriteString([]string{"P", "P", "P", "p", ""}[r.IntN(5)])
		for range r.IntN(6) {
			if r.IntN(4) == 0 {
				b.WriteString([]string{"T", "T", "t", "TT"}[r.IntN(4)])
			}
			b.WriteString(number())
			b.WriteString(fraction())
			b.WriteString(string("YMWDHMSX"[r.IntN(8)]))
		}
		checkDuration(t, b.String())
		if t.Failed() {
			return
		}
	}
}

func FuzzISO8601DurationMatchesLegacy(f *testing.F) {
	for _, s := range []string{"PT1M30S", "+P1DT0.5S", "-PT2562047H47M16.854775808S", "P1W", "PT1.5M", "P106751DT23H47M16.854775807S"} {
		f.Add(s)
	}
	f.Fuzz(checkDuration)
}
