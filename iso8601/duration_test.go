package iso8601

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	maxIntStr    = "9223372036854775807" // math.MaxInt64
	minIntMagStr = "9223372036854775808" // -math.MinInt64
)

func TestParseDurationAccepted(t *testing.T) {
	h, m, s := time.Hour, time.Minute, time.Second
	tests := []struct {
		in   string
		want Duration
	}{
		// ISO 8601's examples.
		{"P2Y10M15DT10H30M20S", Duration{Years: 2, Months: 10, Days: 15, Time: 10*h + 30*m + 20*s}},
		{"P6W", Duration{Weeks: 6}},

		{"P1Y2M3DT4H5M6.5S", Duration{Years: 1, Months: 2, Days: 3, Time: 4*h + 5*m + 6500*time.Millisecond}},
		{"P1Y", Duration{Years: 1}},
		{"P1M", Duration{Months: 1}},
		{"P1W", Duration{Weeks: 1}},
		{"P1D", Duration{Days: 1}},
		{"PT1H", Duration{Time: h}},
		{"PT1M", Duration{Time: m}},
		{"PT1S", Duration{Time: s}},
		{"PT36H", Duration{Time: 36 * h}},
		{"PT90M", Duration{Time: 90 * m}},
		{"P0001Y", Duration{Years: 1}},

		// Zero, written several ways.
		{"PT0S", Duration{}},
		{"P0D", Duration{}},
		{"PT0H0M0S", Duration{}},
		{"-PT0S", Duration{}},

		// Zero components may be omitted, as ISO 8601 allows (RFC 3339's
		// ABNF does not).
		{"P1YT1S", Duration{Years: 1, Time: s}},
		{"PT1H30S", Duration{Time: h + 30*s}},
		{"P1Y1D", Duration{Years: 1, Days: 1}},

		// Weeks combined with other components (ISO 8601-2:2019).
		{"P1W2D", Duration{Weeks: 1, Days: 2}},
		{"P1Y2W", Duration{Years: 1, Weeks: 2}},
		{"P1Y2M3W4DT5H6M7S", Duration{Years: 1, Months: 2, Weeks: 3, Days: 4, Time: 5*h + 6*m + 7*s}},

		// A fraction on the smallest component, with "." or ",".
		{"PT1.5S", Duration{Time: 1500 * time.Millisecond}},
		{"PT1,5S", Duration{Time: 1500 * time.Millisecond}},
		{"PT0.5H", Duration{Time: 30 * m}},
		{"PT1.5M", Duration{Time: 90 * s}},
		{"PT1H0.5M", Duration{Time: h + 30*s}},
		{"P1DT1.5H", Duration{Days: 1, Time: 90 * m}},
		{"PT0.000000001S", Duration{Time: 1}},
		{"PT0.0000000019S", Duration{Time: 1}},
		{"PT0.9999999999S", Duration{Time: 999999999}},
		{"PT0.3333333333333333333333H", Duration{Time: 1199999999999}},
		// Digits past the 19th still count: each of these is just above a
		// whole number of nanoseconds.
		{"PT0.3333333333333333333334H", Duration{Time: 20 * m}},
		{"P0.142857142857142857142858W", Duration{Days: 1}},
		{"P0.00000000000001157407407407407407407408D", Duration{Time: 1}},
		{"PT0." + strings.Repeat("9", 100) + "S", Duration{Time: 999999999}},
		{"P1.5D", Duration{Days: 1, Time: 12 * h}},
		{"P0.5D", Duration{Time: 12 * h}},
		{"P1.5W", Duration{Weeks: 1, Days: 3, Time: 12 * h}},
		{"P0,25W", Duration{Days: 1, Time: 18 * h}},
		{"P0.1W", Duration{Time: 16*h + 48*m}},

		// Negative durations.
		{"-P1D", Duration{Days: -1}},
		{"-PT1M30S", Duration{Time: -90 * s}},
		{"-P1Y2M3W4DT5H6M7.5S", Duration{Years: -1, Months: -2, Weeks: -3, Days: -4, Time: -(5*h + 6*m + 7500*time.Millisecond)}},
		{"-P1.5D", Duration{Days: -1, Time: -12 * h}},

		// The limits of time.Duration.
		{"PT2562047H47M16.854775807S", Duration{Time: math.MaxInt64}},
		{"PT9223372036.854775807S", Duration{Time: math.MaxInt64}},
		{"-PT2562047H47M16.854775808S", Duration{Time: math.MinInt64}},
		{"-PT" + minIntMagStr[:10] + "." + minIntMagStr[10:] + "S", Duration{Time: math.MinInt64}},
	}
	if strconv.IntSize == 64 {
		tests = append(tests,
			struct {
				in   string
				want Duration
			}{"P" + maxIntStr + "Y", Duration{Years: math.MaxInt}},
			struct {
				in   string
				want Duration
			}{"-P" + minIntMagStr + "D", Duration{Days: math.MinInt}},
		)
	}
	for _, tt := range tests {
		got, err := ParseDuration(tt.in)
		if err != nil {
			t.Errorf("ParseDuration(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDuration(%q) = %#v, want %#v", tt.in, got, tt.want)
		}
	}
}

func TestParseDurationRejected(t *testing.T) {
	tests := []struct {
		in     string
		offset int
		msg    string
	}{
		{"", 0, `expected 'P', found end of input`},
		{"1D", 0, `expected 'P', found '1'`},
		{"p1d", 0, `expected 'P', found 'p'`},
		{" P1D", 0, `expected 'P', found ' '`},
		{"+P1D", 0, `expected 'P', found '+'`},
		{"-", 1, `expected 'P', found end of input`},
		{"--P1D", 1, `expected 'P', found '-'`},
		{"P", 1, `a duration needs at least one component`},
		{"-P", 2, `a duration needs at least one component`},
		{"PT", 2, `expected a number of hours, minutes or seconds after 'T', found end of input`},
		{"P1DT", 4, `expected a number of hours, minutes or seconds after 'T', found end of input`},
		{"P1Y2MT", 6, `expected a number of hours, minutes or seconds after 'T', found end of input`},
		{"PTT1H", 2, `expected a number of hours, minutes or seconds after 'T', found 'T'`},
		{"P1DT1HT1M", 6, `duplicate time designator 'T'`},
		{"P1", 2, `expected a designator (Y, M, W, D, H or S) after 1`},
		{"PT1.5", 5, `expected a designator (Y, M, W, D, H or S) after 1.5`},
		{"P1Y2M3DT4H5M6S7", 15, `expected a designator (Y, M, W, D, H or S) after 7`},
		{"PY", 1, `expected a number, found 'Y'`},
		{"P-1D", 1, `expected a number, found '-'`},
		{"PT.5S", 2, `expected a number of hours, minutes or seconds after 'T', found '.'`},
		{"P.5D", 1, `expected a number, found '.'`},
		{"P１D", 1, `expected a number, found '１'`},
		{"P1D ", 3, `expected a number, found ' '`},

		// Units out of order or repeated.
		{"P1D2Y", 4, `years designator 'Y' must come before days designator 'D'`},
		{"P1M1Y", 4, `years designator 'Y' must come before months designator 'M'`},
		{"P1D1W", 4, `weeks designator 'W' must come before days designator 'D'`},
		{"PT1M1H", 5, `hours designator 'H' must come before minutes designator 'M'`},
		{"PT1S1M", 5, `minutes designator 'M' must come before seconds designator 'S'`},
		{"P1Y1Y", 4, `duplicate years designator 'Y'`},
		{"P1W1W", 4, `duplicate weeks designator 'W'`},
		{"PT1H1H", 5, `duplicate hours designator 'H'`},

		// Units on the wrong side of 'T', or unknown.
		{"P1H", 2, `designator 'H' must follow the time designator 'T'`},
		{"P1S", 2, `designator 'S' must follow the time designator 'T'`},
		{"PT1D", 3, `designator 'D' must come before the time designator 'T'`},
		{"PT1Y", 3, `designator 'Y' must come before the time designator 'T'`},
		{"PT1W", 3, `designator 'W' must come before the time designator 'T'`},
		{"P1X", 2, `expected designator Y, M, W or D, found 'X'`},
		{"P1d", 2, `expected designator Y, M, W or D, found 'd'`},
		{"PT1X", 3, `expected designator H, M or S, found 'X'`},
		{"P1.5.5D", 4, `expected designator Y, M, W or D, found '.'`},

		// Fractions.
		{"P1.5DT1H", 2, `only the smallest component present may have a decimal fraction`},
		{"PT1.5H30M", 3, `only the smallest component present may have a decimal fraction`},
		{"P0.5W1D", 2, `only the smallest component present may have a decimal fraction`},
		{"P1.5Y", 2, `a fraction of years is not supported, because their length varies`},
		{"P1,5M", 2, `a fraction of months is not supported, because their length varies`},
		{"PT1.S", 4, `expected digit after decimal sign, found 'S'`},

		// Range.
		{"P99999999999999999999Y", 1, `years value 99999999999999999999 is out of range`},
		{"PT2562047H47M16.854775808S", 13, `hours, minutes and seconds exceed the range of time.Duration`},
		{"PT9223372037S", 2, `hours, minutes and seconds exceed the range of time.Duration`},
		{"PT2562048H", 2, `hours, minutes and seconds exceed the range of time.Duration`},
		{"-PT2562047H47M16.854775809S", 14, `hours, minutes and seconds exceed the range of time.Duration`},
	}
	if strconv.IntSize == 64 {
		tests = append(tests, struct {
			in     string
			offset int
			msg    string
		}{"P" + minIntMagStr + "D", 1, "days value " + minIntMagStr + " is out of range"})
	}
	for _, tt := range tests {
		got, err := ParseDuration(tt.in)
		if err == nil {
			t.Errorf("ParseDuration(%q) = %v, want error %q", tt.in, got, tt.msg)
			continue
		}
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("ParseDuration(%q) error %T is not a *ParseError", tt.in, err)
			continue
		}
		if pe.Input != tt.in || pe.Offset != tt.offset || pe.Msg != tt.msg {
			t.Errorf("ParseDuration(%q):\n got offset %d: %s\nwant offset %d: %s", tt.in, pe.Offset, pe.Msg, tt.offset, tt.msg)
		}
	}

	_, err := ParseDuration("P1D2Y")
	want := `iso8601: invalid duration "P1D2Y" at offset 4: years designator 'Y' must come before days designator 'D'`
	if err == nil || err.Error() != want {
		t.Errorf("error = %v\nwant    %s", err, want)
	}
}

var stringTests = []struct {
	d    Duration
	want string
}{
	{Duration{}, "PT0S"},
	{Duration{Years: 1, Months: 2, Days: 3, Time: 4*time.Hour + 5*time.Minute + 6500*time.Millisecond}, "P1Y2M3DT4H5M6.5S"},
	{Duration{Years: 2, Months: 10, Days: 15, Time: 10*time.Hour + 30*time.Minute + 20*time.Second}, "P2Y10M15DT10H30M20S"},
	{Duration{Weeks: 2}, "P2W"},
	{Duration{Weeks: 1, Days: 2}, "P1W2D"},
	{Duration{Years: 1, Time: time.Second}, "P1YT1S"},
	{Duration{Time: 90 * time.Minute}, "PT1H30M"},
	{Duration{Time: 36 * time.Hour}, "PT36H"},
	{Duration{Time: 90 * time.Second}, "PT1M30S"},
	{Duration{Time: time.Hour + 30*time.Second}, "PT1H30S"},
	{Duration{Time: 1}, "PT0.000000001S"},
	{Duration{Time: 1500 * time.Millisecond}, "PT1.5S"},
	{Duration{Time: 120 * time.Microsecond}, "PT0.00012S"},
	{Duration{Time: time.Hour + 1}, "PT1H0.000000001S"},
	{Duration{Days: -1, Time: -2 * time.Hour}, "-P1DT2H"},
	{Duration{Years: -1, Months: -2, Weeks: -3, Days: -4, Time: -7500 * time.Millisecond}, "-P1Y2M3W4DT7.5S"},
	{Duration{Time: math.MaxInt64}, "PT2562047H47M16.854775807S"},
	{Duration{Time: math.MinInt64}, "-PT2562047H47M16.854775808S"},
	{Duration{Days: math.MaxInt32}, "P2147483647D"},
	{Duration{Days: math.MinInt32}, "-P2147483648D"},
}

func TestDurationString(t *testing.T) {
	for _, tt := range stringTests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("%#v.String() = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestDurationStringMixedSigns(t *testing.T) {
	tests := []struct {
		d    Duration
		want string
	}{
		{Duration{Days: 1, Time: -time.Hour}, "P1DT-1H"},
		{Duration{Days: 1, Time: -90 * time.Minute}, "P1DT-1H-30M"},
		{Duration{Years: -1, Months: 1}, "P-1Y1M"},
		{Duration{Months: 8, Days: -1}, "P8M-1D"},
	}
	for _, tt := range tests {
		if got := tt.d.String(); got != tt.want {
			t.Errorf("%#v.String() = %q, want %q", tt.d, got, tt.want)
		}
		if _, err := tt.d.MarshalText(); !errors.Is(err, errMixedSigns) {
			t.Errorf("%#v.MarshalText() error = %v, want errMixedSigns", tt.d, err)
		}
		if _, err := ParseDuration(tt.d.String()); err == nil {
			t.Errorf("ParseDuration(%q) succeeded, want error", tt.d.String())
		}
	}
}

func TestDurationRoundTrip(t *testing.T) {
	ds := []Duration{
		{Years: math.MaxInt},
		{Years: math.MinInt, Months: math.MinInt, Weeks: math.MinInt, Days: math.MinInt, Time: math.MinInt64},
		{Years: math.MaxInt, Months: math.MaxInt, Weeks: math.MaxInt, Days: math.MaxInt, Time: math.MaxInt64},
	}
	for _, tt := range stringTests {
		ds = append(ds, tt.d)
	}
	for _, d := range ds {
		s := d.String()
		got, err := ParseDuration(s)
		if err != nil {
			t.Errorf("ParseDuration(%q): %v", s, err)
			continue
		}
		if got != d {
			t.Errorf("ParseDuration(%q) = %#v, want %#v", s, got, d)
		}
	}
}

func TestDurationExact(t *testing.T) {
	tests := []struct {
		d    Duration
		want time.Duration
		ok   bool
	}{
		{Duration{}, 0, true},
		{Duration{Time: 90 * time.Minute}, 90 * time.Minute, true},
		{Duration{Days: 1}, 24 * time.Hour, true},
		{Duration{Weeks: 1}, 7 * 24 * time.Hour, true},
		{Duration{Weeks: 1, Days: 2, Time: time.Hour}, 9*24*time.Hour + time.Hour, true},
		{Duration{Days: -1, Time: -time.Hour}, -25 * time.Hour, true},
		{Duration{Days: 1, Time: -time.Hour}, 23 * time.Hour, true},
		{Duration{Days: 106751}, 106751 * 24 * time.Hour, true},
		{Duration{Years: 1}, 0, false},
		{Duration{Months: -1, Days: 1}, 0, false},
		{Duration{Days: 106752}, 0, false},
		{Duration{Weeks: math.MaxInt32}, 0, false},
		{Duration{Days: 1, Time: math.MaxInt64}, 0, false},
		{Duration{Days: -1, Time: math.MinInt64}, 0, false},
	}
	for _, tt := range tests {
		got, ok := tt.d.Exact()
		if got != tt.want || ok != tt.ok {
			t.Errorf("%v.Exact() = %v, %v; want %v, %v", tt.d, got, ok, tt.want, tt.ok)
		}
	}
}

func TestDurationAddToDST(t *testing.T) {
	newYork := mustLoad(t, "America/New_York")
	stockholm := mustLoad(t, "Europe/Stockholm")
	tests := []struct {
		name    string
		start   time.Time
		d       string
		want    time.Time
		elapsed time.Duration
	}{
		// New York springs forward on 2026-03-08 and falls back on
		// 2026-11-01.
		{"NY spring P1D", time.Date(2026, 3, 7, 12, 0, 0, 0, newYork), "P1D",
			time.Date(2026, 3, 8, 12, 0, 0, 0, newYork), 23 * time.Hour},
		{"NY spring PT24H", time.Date(2026, 3, 7, 12, 0, 0, 0, newYork), "PT24H",
			time.Date(2026, 3, 8, 13, 0, 0, 0, newYork), 24 * time.Hour},
		{"NY spring P1DT1H", time.Date(2026, 3, 7, 12, 0, 0, 0, newYork), "P1DT1H",
			time.Date(2026, 3, 8, 13, 0, 0, 0, newYork), 24 * time.Hour},
		{"NY spring P1W", time.Date(2026, 3, 5, 12, 0, 0, 0, newYork), "P1W",
			time.Date(2026, 3, 12, 12, 0, 0, 0, newYork), 167 * time.Hour},
		{"NY spring -P1D", time.Date(2026, 3, 8, 12, 0, 0, 0, newYork), "-P1D",
			time.Date(2026, 3, 7, 12, 0, 0, 0, newYork), -23 * time.Hour},
		{"NY fall P1D", time.Date(2026, 10, 31, 12, 0, 0, 0, newYork), "P1D",
			time.Date(2026, 11, 1, 12, 0, 0, 0, newYork), 25 * time.Hour},
		{"NY fall PT24H", time.Date(2026, 10, 31, 12, 0, 0, 0, newYork), "PT24H",
			time.Date(2026, 11, 1, 11, 0, 0, 0, newYork), 24 * time.Hour},

		// Stockholm springs forward on 2026-03-29 and falls back on
		// 2026-10-25.
		{"Stockholm spring P1D", time.Date(2026, 3, 28, 12, 0, 0, 0, stockholm), "P1D",
			time.Date(2026, 3, 29, 12, 0, 0, 0, stockholm), 23 * time.Hour},
		{"Stockholm spring PT24H", time.Date(2026, 3, 28, 12, 0, 0, 0, stockholm), "PT24H",
			time.Date(2026, 3, 29, 13, 0, 0, 0, stockholm), 24 * time.Hour},
		{"Stockholm spring P1M", time.Date(2026, 3, 15, 12, 0, 0, 0, stockholm), "P1M",
			time.Date(2026, 4, 15, 12, 0, 0, 0, stockholm), 31*24*time.Hour - time.Hour},
		{"Stockholm fall P1D", time.Date(2026, 10, 24, 12, 0, 0, 0, stockholm), "P1D",
			time.Date(2026, 10, 25, 12, 0, 0, 0, stockholm), 25 * time.Hour},
		{"Stockholm fall PT24H", time.Date(2026, 10, 24, 12, 0, 0, 0, stockholm), "PT24H",
			time.Date(2026, 10, 25, 11, 0, 0, 0, stockholm), 24 * time.Hour},
		{"Stockholm fall -P1DT1H", time.Date(2026, 10, 25, 12, 0, 0, 0, stockholm), "-P1DT1H",
			time.Date(2026, 10, 24, 11, 0, 0, 0, stockholm), -26 * time.Hour},
	}
	for _, tt := range tests {
		d, err := ParseDuration(tt.d)
		if err != nil {
			t.Fatalf("%s: ParseDuration(%q): %v", tt.name, tt.d, err)
		}
		got := d.AddTo(tt.start)
		if !got.Equal(tt.want) {
			t.Errorf("%s: %s + %s = %s, want %s", tt.name, FormatTime(tt.start), tt.d, FormatTime(got), FormatTime(tt.want))
		}
		if e := got.Sub(tt.start); e != tt.elapsed {
			t.Errorf("%s: elapsed %v, want %v", tt.name, e, tt.elapsed)
		}
	}
}

// TestDurationAddToTransitions covers calendar steps that land on a wall
// clock time repeated or skipped by a change in the UTC offset, where
// time.AddDate's choice is unspecified (and differs east and west of UTC).
func TestDurationAddToTransitions(t *testing.T) {
	newYork := mustLoad(t, "America/New_York")
	berlin := mustLoad(t, "Europe/Berlin")
	santiago := mustLoad(t, "America/Santiago")
	apia := mustLoad(t, "Pacific/Apia")
	// The second 01:30 on 2026-11-01 in New York, after the clocks go back.
	secondOneThirty := time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC).In(newYork)
	tests := []struct {
		name    string
		start   time.Time
		d       string
		want    string
		elapsed time.Duration
	}{
		{"NY skipped", time.Date(2026, 3, 7, 2, 30, 0, 0, newYork), "P1D", "2026-03-08T03:30:00-04:00", 24 * time.Hour},
		{"NY repeated", time.Date(2026, 10, 31, 1, 30, 0, 0, newYork), "P1D", "2026-11-01T01:30:00-04:00", 24 * time.Hour},
		{"Berlin skipped", time.Date(2026, 3, 28, 2, 30, 0, 0, berlin), "P1D", "2026-03-29T03:30:00+02:00", 24 * time.Hour},
		{"Berlin repeated", time.Date(2026, 10, 24, 2, 30, 0, 0, berlin), "P1D", "2026-10-25T02:30:00+02:00", 24 * time.Hour},
		{"Santiago skipped midnight", time.Date(2026, 9, 5, 0, 0, 0, 0, santiago), "P1D", "2026-09-06T01:00:00-03:00", 24 * time.Hour},
		{"Apia skipped day", time.Date(2011, 12, 29, 12, 0, 0, 0, apia), "P1D", "2011-12-31T12:00:00+14:00", 24 * time.Hour},
		{"second 01:30 PT0S", secondOneThirty, "PT0S", "2026-11-01T01:30:00-05:00", 0},
		{"second 01:30 PT1H", secondOneThirty, "PT1H", "2026-11-01T02:30:00-05:00", time.Hour},
		{"second 01:30 P1D", secondOneThirty, "P1D", "2026-11-02T01:30:00-05:00", 24 * time.Hour},
	}
	for _, tt := range tests {
		d, err := ParseDuration(tt.d)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		got := d.AddTo(tt.start)
		if s := FormatTime(got); s != tt.want {
			t.Errorf("%s: %s + %s = %s, want %s", tt.name, FormatTime(tt.start), tt.d, s, tt.want)
		}
		if e := got.Sub(tt.start); e != tt.elapsed {
			t.Errorf("%s: elapsed %v, want %v", tt.name, e, tt.elapsed)
		}
	}
}

func TestDurationAddToCalendar(t *testing.T) {
	tests := []struct {
		start string
		d     string
		want  string
	}{
		// Leap days.
		{"2024-02-29", "P1Y", "2025-03-01"},
		{"2024-02-29", "P4Y", "2028-02-29"},
		{"2024-02-29", "-P1Y", "2023-03-01"},
		{"2024-02-29", "P1Y1D", "2025-03-02"},
		{"2023-02-28", "P1Y", "2024-02-28"},
		{"2024-02-28", "P1D", "2024-02-29"},
		// End-of-month normalization, as with time.AddDate.
		{"2026-01-31", "P1M", "2026-03-03"},
		{"2024-01-31", "P1M", "2024-03-02"},
		{"2026-03-31", "-P1M", "2026-03-03"},
		{"2026-08-31", "P1M", "2026-10-01"},
		// Weeks are seven days.
		{"2026-11-19", "P2W", "2026-12-03"},
		{"2026-12-28", "P1W2D", "2027-01-06"},
	}
	for _, tt := range tests {
		start, err := ParseTime(tt.start, time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		want, err := ParseTime(tt.want, time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		d, err := ParseDuration(tt.d)
		if err != nil {
			t.Fatal(err)
		}
		if got := d.AddTo(start); !got.Equal(want) {
			t.Errorf("%s + %s = %s, want %s", tt.start, tt.d, FormatTime(got), FormatTime(want))
		}
	}
}

func TestFromStd(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "PT0S"},
		{90 * time.Second, "PT1M30S"},
		{-1500 * time.Millisecond, "-PT1.5S"},
		{36 * time.Hour, "PT36H"},
		{time.Nanosecond, "PT0.000000001S"},
		{math.MinInt64, "-PT2562047H47M16.854775808S"},
	}
	for _, tt := range tests {
		if got := FormatDuration(tt.d); got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
		d := FromStd(tt.d)
		if d != (Duration{Time: tt.d}) {
			t.Errorf("FromStd(%v) = %#v", tt.d, d)
		}
		if exact, ok := d.Exact(); !ok || exact != tt.d {
			t.Errorf("FromStd(%v).Exact() = %v, %v", tt.d, exact, ok)
		}
	}
}

func TestDurationText(t *testing.T) {
	type config struct {
		TTL Duration `json:"ttl"`
	}
	b, err := json.Marshal(config{TTL: Duration{Days: 1, Time: 2 * time.Hour}})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"ttl":"P1DT2H"}` {
		t.Errorf("json.Marshal = %s", b)
	}

	var c config
	if err := json.Unmarshal([]byte(`{"ttl":"-PT1M30S"}`), &c); err != nil {
		t.Fatal(err)
	}
	if c.TTL != (Duration{Time: -90 * time.Second}) {
		t.Errorf("json.Unmarshal: TTL = %#v", c.TTL)
	}

	c.TTL = Duration{Weeks: 1}
	err = json.Unmarshal([]byte(`{"ttl":"P1D2Y"}`), &c)
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Offset != 4 {
		t.Errorf("json.Unmarshal(invalid) error = %v, want a *ParseError at offset 4", err)
	}
	if c.TTL != (Duration{Weeks: 1}) {
		t.Errorf("failed UnmarshalText changed the value to %v", c.TTL)
	}

	if _, err := json.Marshal(config{TTL: Duration{Days: 1, Time: -time.Hour}}); !errors.Is(err, errMixedSigns) {
		t.Errorf("json.Marshal(mixed signs) error = %v, want errMixedSigns", err)
	}

	b, err = Duration{Years: 1}.AppendText([]byte("ttl="))
	if err != nil || string(b) != "ttl=P1Y" {
		t.Errorf("AppendText = %q, %v", b, err)
	}
}

func TestDurationFlagTextVar(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var ttl Duration
	fs.TextVar(&ttl, "ttl", Duration{Days: 1}, "time to live")
	if got := fs.Lookup("ttl").DefValue; got != "P1D" {
		t.Errorf("DefValue = %q, want P1D", got)
	}
	if err := fs.Parse([]string{"-ttl", "PT90M"}); err != nil {
		t.Fatal(err)
	}
	if ttl != (Duration{Time: 90 * time.Minute}) {
		t.Errorf("ttl = %#v", ttl)
	}
	err := fs.Parse([]string{"-ttl", "P1H"})
	if err == nil || !strings.Contains(err.Error(), `designator 'H' must follow the time designator 'T'`) {
		t.Errorf("Parse(-ttl P1H) error = %v", err)
	}
}
