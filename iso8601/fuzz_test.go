package iso8601

import (
	"errors"
	"testing"
	"time"
)

var timeSeeds = []string{
	"2026-11-19", "20261119", "2026-W47-4", "2026W474", "2026-323", "2026323",
	"2026-11-19T10:30", "20261119T1030", "2026-11-19T10:30:00.5Z",
	"20261119T103000,5+0100", "2026-11-19T10:30:00+01:00", "2026-11-19T10:30:00-05",
	"2026-11-19 10:30:00Z", "2026-11-19t10:30:00z", "2026-11-19T24:00Z",
	"1990-12-31T23:59:60Z", "1990-12-31T15:59:60-08:00", "1937-01-01T12:00:27.87+00:20",
	"2026-W53-7", "2020-W53-5", "9999-12-31T23:59:59.999999999Z", "0000-01-01T00:00+01:00",
	"2026-11-19T10:30:00+0100", "2026-13-01", "", "2026-W00-1",
	"1850-01-01T00:00", "2026-03-08T02:30", "2026-10-25T02:30",
}

// FuzzParseTime checks that ParseTime never panics, that every error is a
// *ParseError with an offset inside the input, and that a parsed time
// formats to a string that parses back to the same instant and offset.
func FuzzParseTime(f *testing.F) {
	for _, s := range timeSeeds {
		f.Add(s)
	}
	// A location with daylight saving gaps and historical local mean time,
	// so local inputs exercise time.Date normalization.
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, loc := range []*time.Location{time.UTC, newYork} {
			got, err := ParseTime(s, loc)
			if err != nil {
				var pe *ParseError
				if !errors.As(err, &pe) {
					t.Fatalf("ParseTime(%q): error %T is not a *ParseError", s, err)
				}
				if pe.Input != s || pe.Offset < 0 || pe.Offset > len(s) {
					t.Fatalf("ParseTime(%q): bad error %#v", s, pe)
				}
				continue
			}
			out := FormatTime(got)
			again, err := ParseTime(out, nil)
			if err != nil {
				t.Fatalf("ParseTime(%q) = %v, but ParseTime(FormatTime = %q): %v", s, got, out, err)
			}
			if !again.Equal(got) {
				t.Fatalf("ParseTime(%q) = %v; reparsed %q = %v", s, got, out, again)
			}
			_, off1 := formatZone(got).Zone()
			_, off2 := again.Zone()
			if off1 != off2 {
				t.Fatalf("ParseTime(%q): offset %d, reparsed %q has offset %d", s, off1, out, off2)
			}
			if out2 := FormatTime(again); out2 != out {
				t.Fatalf("FormatTime not stable: %q then %q", out, out2)
			}
		}
	})
}

var durationSeeds = []string{
	"P1Y2M3DT4H5M6.5S", "P2W", "P1W2D", "PT36H", "PT0S", "-P1D", "-PT1M30S",
	"P1.5D", "P0,25W", "PT0.3333333333333333333333H", "PT1H30S",
	"PT2562047H47M16.854775807S", "-PT2562047H47M16.854775808S",
	"P9223372036854775807Y", "-P9223372036854775808D", "P99999999999999999999Y",
	"P", "PT", "P1D2Y", "P1.5DT1H", "P1.5Y",
}

// FuzzParseDuration checks that ParseDuration never panics, that every
// error is a *ParseError with an offset inside the input, and that a parsed
// duration's String parses back to the same Duration and is stable.
func FuzzParseDuration(f *testing.F) {
	for _, s := range durationSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		d, err := ParseDuration(s)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("ParseDuration(%q): error %T is not a *ParseError", s, err)
			}
			if pe.Input != s || pe.Offset < 0 || pe.Offset > len(s) {
				t.Fatalf("ParseDuration(%q): bad error %#v", s, pe)
			}
			return
		}
		if pos, neg := d.signs(); pos && neg {
			t.Fatalf("ParseDuration(%q) = %#v has mixed signs", s, d)
		}
		out := d.String()
		again, err := ParseDuration(out)
		if err != nil {
			t.Fatalf("ParseDuration(%q) = %#v, but ParseDuration(String = %q): %v", s, d, out, err)
		}
		if again != d {
			t.Fatalf("ParseDuration(%q) = %#v; reparsed %q = %#v", s, d, out, again)
		}
		if text, err := d.MarshalText(); err != nil || string(text) != out {
			t.Fatalf("MarshalText(%#v) = %q, %v; want %q", d, text, err, out)
		}
		if out2 := again.String(); out2 != out {
			t.Fatalf("String not stable: %q then %q", out, out2)
		}
	})
}

func BenchmarkParseTime(b *testing.B) {
	for _, s := range []string{
		"2026-11-19T10:30:00Z",
		"2026-11-19T10:30:00.123456789+01:00",
		"20261119T103000,5+0100",
		"2026-W47-4T10:30:00Z",
		"2026-11-19",
	} {
		b.Run(s, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := ParseTime(s, time.UTC); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkTimeParseRFC3339 is a baseline: the standard library's parser
// on an input both accept.
func BenchmarkTimeParseRFC3339(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := time.Parse(time.RFC3339Nano, "2026-11-19T10:30:00.123456789+01:00"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormatTime(b *testing.B) {
	t := time.Date(2026, 11, 19, 10, 30, 0, 123456789, time.FixedZone("", 3600))
	b.ReportAllocs()
	for b.Loop() {
		_ = FormatTime(t)
	}
}

func BenchmarkParseDuration(b *testing.B) {
	for _, s := range []string{
		"PT1M30S",
		"P1Y2M3DT4H5M6.5S",
		"-P1W2DT0.3333333333333333333333H",
	} {
		b.Run(s, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := ParseDuration(s); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkDurationString(b *testing.B) {
	d := Duration{Years: 1, Months: 2, Days: 3, Time: 4*time.Hour + 5*time.Minute + 6500*time.Millisecond}
	b.ReportAllocs()
	for b.Loop() {
		_ = d.String()
	}
}
