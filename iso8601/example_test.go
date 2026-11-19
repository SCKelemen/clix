package iso8601_test

import (
	"flag"
	"fmt"
	"time"

	"github.com/SCKelemen/clix/v2/iso8601"
)

func ExampleParseTime() {
	for _, s := range []string{
		"2026-11-19T10:30:00Z",
		"20261119T103000,5+0100",
		"2026-W47-4",
		"2026-323",
		"2026-11-19T24:00Z",
	} {
		t, err := iso8601.ParseTime(s, time.UTC)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(iso8601.FormatTime(t))
	}

	_, err := iso8601.ParseTime("2026-11-19T10:30", nil)
	fmt.Println(err)
	// Output:
	// 2026-11-19T10:30:00Z
	// 2026-11-19T10:30:00.5+01:00
	// 2026-11-19T00:00:00Z
	// 2026-11-19T00:00:00Z
	// 2026-11-20T00:00:00Z
	// iso8601: invalid time "2026-11-19T10:30" at offset 16: a time without a UTC offset needs a location, and loc is nil
}

func ExampleParseDuration() {
	d, err := iso8601.ParseDuration("P1Y2M3DT4H5M6,5S")
	if err != nil {
		panic(err)
	}
	fmt.Println(d.Years, d.Months, d.Days, d.Time)
	fmt.Println(d)

	_, err = iso8601.ParseDuration("PT1D")
	fmt.Println(err)
	// Output:
	// 1 2 3 4h5m6.5s
	// P1Y2M3DT4H5M6.5S
	// iso8601: invalid duration "PT1D" at offset 3: designator 'D' must come before the time designator 'T'
}

func ExampleDuration_AddTo() {
	stockholm, err := time.LoadLocation("Europe/Stockholm")
	if err != nil {
		panic(err)
	}
	// Daylight saving time starts in Stockholm on 2026-03-29.
	start := time.Date(2026, 3, 28, 12, 0, 0, 0, stockholm)
	for _, s := range []string{"P1D", "PT24H"} {
		d, _ := iso8601.ParseDuration(s)
		end := d.AddTo(start)
		fmt.Println(s, iso8601.FormatTime(end), end.Sub(start))
	}
	// Output:
	// P1D 2026-03-29T12:00:00+02:00 23h0m0s
	// PT24H 2026-03-29T13:00:00+02:00 24h0m0s
}

func ExampleDuration_Exact() {
	week, _ := iso8601.ParseDuration("P1W")
	fmt.Println(week.Exact())

	month, _ := iso8601.ParseDuration("P1M")
	fmt.Println(month.Exact())
	// Output:
	// 168h0m0s true
	// 0s false
}

func ExampleFormatDuration() {
	fmt.Println(iso8601.FormatDuration(90 * time.Second))
	fmt.Println(iso8601.FormatDuration(36 * time.Hour))
	fmt.Println(iso8601.FormatDuration(-1500 * time.Millisecond))
	fmt.Println(iso8601.FormatDuration(0))
	// Output:
	// PT1M30S
	// PT36H
	// -PT1.5S
	// PT0S
}

// Duration implements encoding.TextUnmarshaler, so it works with
// flag.TextVar.
func ExampleDuration_flag() {
	fs := flag.NewFlagSet("example", flag.ContinueOnError)
	var ttl iso8601.Duration
	fs.TextVar(&ttl, "ttl", iso8601.Duration{Days: 1}, "time to live (ISO 8601)")
	if err := fs.Parse([]string{"-ttl", "P1W2D"}); err != nil {
		panic(err)
	}
	fmt.Println(ttl, ttl.Weeks, ttl.Days)
	// Output:
	// P1W2D 1 2
}
