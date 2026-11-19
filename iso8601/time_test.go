package iso8601

import (
	"errors"
	"testing"
	"time"
	_ "time/tzdata" // the DST tests must not depend on the host's zoneinfo
)

func mustLoad(t testing.TB, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

func utc(y int, m time.Month, d, hh, mm, ss, ns int) time.Time {
	return time.Date(y, m, d, hh, mm, ss, ns, time.UTC)
}

func TestParseTimeAccepted(t *testing.T) {
	plus4 := time.FixedZone("", 4*3600)
	stockholm := mustLoad(t, "Europe/Stockholm")
	nov19 := utc(2026, time.November, 19, 0, 0, 0, 0)

	tests := []struct {
		in   string
		loc  *time.Location
		want time.Time
	}{
		// ISO 8601's running example, 12 April 1985, in each date form.
		{"1985-04-12", time.UTC, utc(1985, 4, 12, 0, 0, 0, 0)},
		{"19850412", time.UTC, utc(1985, 4, 12, 0, 0, 0, 0)},
		{"1985-102", time.UTC, utc(1985, 4, 12, 0, 0, 0, 0)},
		{"1985102", time.UTC, utc(1985, 4, 12, 0, 0, 0, 0)},
		{"1985-W15-5", time.UTC, utc(1985, 4, 12, 0, 0, 0, 0)},
		{"1985W155", time.UTC, utc(1985, 4, 12, 0, 0, 0, 0)},
		{"1985-04-12T10:15:30", time.UTC, utc(1985, 4, 12, 10, 15, 30, 0)},
		{"19850412T101530", time.UTC, utc(1985, 4, 12, 10, 15, 30, 0)},
		{"1985-04-12T10:15", time.UTC, utc(1985, 4, 12, 10, 15, 0, 0)},
		{"19850412T1015", time.UTC, utc(1985, 4, 12, 10, 15, 0, 0)},
		{"1985-04-12T23:20:50,5", time.UTC, utc(1985, 4, 12, 23, 20, 50, 5e8)},
		{"19850412T232050,5", time.UTC, utc(1985, 4, 12, 23, 20, 50, 5e8)},
		{"1985-04-12T23:20:50.5", time.UTC, utc(1985, 4, 12, 23, 20, 50, 5e8)},
		{"1985-04-12T10:15:30Z", nil, utc(1985, 4, 12, 10, 15, 30, 0)},
		{"19850412T101530Z", nil, utc(1985, 4, 12, 10, 15, 30, 0)},
		{"1985-04-12T10:15:30+04:00", nil, time.Date(1985, 4, 12, 10, 15, 30, 0, plus4)},
		{"1985-04-12T10:15:30+04", nil, time.Date(1985, 4, 12, 10, 15, 30, 0, plus4)},
		{"19850412T101530+0400", nil, time.Date(1985, 4, 12, 10, 15, 30, 0, plus4)},
		{"19850412T101530+04", nil, time.Date(1985, 4, 12, 10, 15, 30, 0, plus4)},
		{"1985-04-12T10:15:30-05:30", nil, time.Date(1985, 4, 12, 10, 15, 30, 0, time.FixedZone("", -(5*3600+30*60)))},

		// The same day in every form.
		{"2026-11-19", time.UTC, nov19},
		{"20261119", time.UTC, nov19},
		{"2026-W47-4", time.UTC, nov19},
		{"2026W474", time.UTC, nov19},
		{"2026-323", time.UTC, nov19},
		{"2026323", time.UTC, nov19},
		{"2026-323T10:00Z", nil, nov19.Add(10 * time.Hour)},
		{"2026W474T1000Z", nil, nov19.Add(10 * time.Hour)},

		// Week dates: week 1 can start in the previous year, and a year
		// that starts on a Thursday (or a leap year that starts on a
		// Wednesday) has 53 weeks.
		{"2026-W01-1", time.UTC, utc(2025, 12, 29, 0, 0, 0, 0)},
		{"2026-W53-7", time.UTC, utc(2027, 1, 3, 0, 0, 0, 0)},
		{"2020-W53-5", time.UTC, utc(2021, 1, 1, 0, 0, 0, 0)},
		{"2021-W01-1", time.UTC, utc(2021, 1, 4, 0, 0, 0, 0)},
		{"2015-W53-7", time.UTC, utc(2016, 1, 3, 0, 0, 0, 0)},
		{"2009-W01-1", time.UTC, utc(2008, 12, 29, 0, 0, 0, 0)},
		{"2008-W01-1", time.UTC, utc(2007, 12, 31, 0, 0, 0, 0)},

		// Leap years.
		{"2024-02-29", time.UTC, utc(2024, 2, 29, 0, 0, 0, 0)},
		{"2024-060", time.UTC, utc(2024, 2, 29, 0, 0, 0, 0)},
		{"2024-366", time.UTC, utc(2024, 12, 31, 0, 0, 0, 0)},
		{"2000-02-29", time.UTC, utc(2000, 2, 29, 0, 0, 0, 0)},
		{"0000-01-01", time.UTC, utc(0, 1, 1, 0, 0, 0, 0)},
		{"0000-W01-1", time.UTC, utc(0, 1, 3, 0, 0, 0, 0)},
		{"9999-W52-5", time.UTC, utc(9999, 12, 31, 0, 0, 0, 0)},
		{"9999-12-31T23:59:59.999999999Z", nil, utc(9999, 12, 31, 23, 59, 59, 999999999)},

		// 24:00 is the end of the day.
		{"2026-11-19T24:00Z", nil, utc(2026, 11, 20, 0, 0, 0, 0)},
		{"2026-11-19T24:00:00Z", nil, utc(2026, 11, 20, 0, 0, 0, 0)},
		{"20261231T240000,000Z", nil, utc(2027, 1, 1, 0, 0, 0, 0)},
		{"2026-12-31T24:00+01:00", nil, time.Date(2027, 1, 1, 0, 0, 0, 0, time.FixedZone("", 3600))},
		{"2026-11-19T24:00", stockholm, time.Date(2026, 11, 20, 0, 0, 0, 0, stockholm)},

		// Lenience permitted by RFC 3339 §5.6.
		{"2026-11-19t10:00:00z", nil, nov19.Add(10 * time.Hour)},
		{"2026-11-19 10:00:00Z", nil, nov19.Add(10 * time.Hour)},
		{"20261119 1000Z", nil, nov19.Add(10 * time.Hour)},
		{"2026-11-19T10:00:00-00:00", nil, nov19.Add(10 * time.Hour)},
		{"2026-11-19T10:00:00+00:00", nil, nov19.Add(10 * time.Hour)},

		// Fractions beyond nanoseconds are truncated.
		{"2026-11-19T10:00:00.1234567899Z", nil, nov19.Add(10*time.Hour + 123456789)},
		{"2026-11-19T10:00:00.000000001Z", nil, nov19.Add(10*time.Hour + 1)},

		// Local times resolve in loc.
		{"2026-07-01T12:00", stockholm, time.Date(2026, 7, 1, 12, 0, 0, 0, stockholm)},
		{"2026-01-01", stockholm, time.Date(2026, 1, 1, 0, 0, 0, 0, stockholm)},

		// A leap second at the end of a month, in UTC and with an offset.
		{"2016-12-31T23:59:60Z", nil, utc(2016, 12, 31, 23, 59, 59, 0)},
		{"2015-06-30T23:59:60.25Z", nil, utc(2015, 6, 30, 23, 59, 59, 25e7)},
		{"2017-01-01T00:59:60+01:00", nil, time.Date(2017, 1, 1, 0, 59, 59, 0, time.FixedZone("", 3600))},
		{"2016-12-31T23:59:60", time.UTC, utc(2016, 12, 31, 23, 59, 59, 0)},
	}
	for _, tt := range tests {
		got, err := ParseTime(tt.in, tt.loc)
		if err != nil {
			t.Errorf("ParseTime(%q): %v", tt.in, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("ParseTime(%q) = %v, want %v", tt.in, got, tt.want)
		}
		_, gotOff := got.Zone()
		_, wantOff := tt.want.Zone()
		if gotOff != wantOff {
			t.Errorf("ParseTime(%q) offset = %d, want %d", tt.in, gotOff, wantOff)
		}
	}
}

func TestParseTimeLocation(t *testing.T) {
	stockholm := mustLoad(t, "Europe/Stockholm")
	// An offset wins over loc, and a zero offset is time.UTC.
	got, err := ParseTime("2026-11-19T10:00:00Z", stockholm)
	if err != nil || got.Location() != time.UTC {
		t.Errorf("Z: got %v (%v), %v; want time.UTC", got, got.Location(), err)
	}
	got, err = ParseTime("2026-11-19T10:00:00+00:00", stockholm)
	if err != nil || got.Location() != time.UTC {
		t.Errorf("+00:00: got %v (%v), %v; want time.UTC", got, got.Location(), err)
	}
	got, err = ParseTime("2026-11-19T10:00:00+01:00", stockholm)
	if err != nil || got.Location() == stockholm {
		t.Errorf("+01:00: got %v (%v), %v; want a fixed zone, not loc", got, got.Location(), err)
	}
	got, err = ParseTime("2026-11-19T10:00:00", stockholm)
	if err != nil || got.Location() != stockholm {
		t.Errorf("local: got %v (%v), %v; want loc", got, got.Location(), err)
	}
}

// TestParseTimeTransitions covers local times and dates that a change in
// the UTC offset repeats or skips. time.Date resolves these one way west
// of UTC and the other way east of it; ParseTime follows RFC 5545 §3.3.5,
// and a date alone or 24:00 is the first instant of the day.
func TestParseTimeTransitions(t *testing.T) {
	santiago := mustLoad(t, "America/Santiago")
	havana := mustLoad(t, "America/Havana")
	amman := mustLoad(t, "Asia/Amman")
	azores := mustLoad(t, "Atlantic/Azores")
	apia := mustLoad(t, "Pacific/Apia")
	newYork := mustLoad(t, "America/New_York")
	berlin := mustLoad(t, "Europe/Berlin")
	tests := []struct {
		in   string
		loc  *time.Location
		want string
	}{
		// Midnight skipped west of UTC: the day starts as the gap ends.
		{"2026-09-06", santiago, "2026-09-06T01:00:00-03:00"},
		{"2026-09-05T24:00", santiago, "2026-09-06T01:00:00-03:00"},
		{"2026-03-08", havana, "2026-03-08T01:00:00-04:00"},
		// Midnight repeated east and west of UTC: the first occurrence.
		{"1995-09-15", amman, "1995-09-15T00:00:00+03:00"},
		{"1995-09-14T24:00", amman, "1995-09-15T00:00:00+03:00"},
		{"2026-11-01", havana, "2026-11-01T00:00:00-04:00"},
		{"1990-09-30", azores, "1990-09-30T00:00:00Z"},
		// A whole day skipped: the first instant after it.
		{"2011-12-30", apia, "2011-12-31T00:00:00+14:00"},
		{"2011-12-29T24:00", apia, "2011-12-31T00:00:00+14:00"},
		// Times of day repeated or skipped (RFC 5545 §3.3.5's own example
		// is New York's skipped 02:30).
		{"2026-03-08T02:30", newYork, "2026-03-08T03:30:00-04:00"},
		{"2026-11-01T01:30", newYork, "2026-11-01T01:30:00-04:00"},
		{"2026-03-29T02:30", berlin, "2026-03-29T03:30:00+02:00"},
		{"2026-10-25T02:30", berlin, "2026-10-25T02:30:00+02:00"},
		{"2026-09-06T00:30", santiago, "2026-09-06T01:30:00-03:00"},
	}
	for _, tt := range tests {
		got, err := ParseTime(tt.in, tt.loc)
		if err != nil {
			t.Errorf("ParseTime(%q, %v): %v", tt.in, tt.loc, err)
			continue
		}
		if s := FormatTime(got); s != tt.want {
			t.Errorf("ParseTime(%q, %v) = %s, want %s", tt.in, tt.loc, s, tt.want)
		}
	}

	// The end of a day comes after every time of that day.
	before, err1 := ParseTime("2026-09-05T23:30", santiago)
	end, err2 := ParseTime("2026-09-05T24:00", santiago)
	if err1 != nil || err2 != nil || !before.Before(end) {
		t.Errorf("Santiago 23:30 = %v, 24:00 = %v (%v, %v); want 23:30 first", before, end, err1, err2)
	}
}

// TestParseTimeRFC3339Examples covers RFC 3339 §5.8.
// https://datatracker.ietf.org/doc/html/rfc3339#section-5.8
func TestParseTimeRFC3339Examples(t *testing.T) {
	tests := []struct {
		in   string
		want time.Time
	}{
		{"1985-04-12T23:20:50.52Z", utc(1985, 4, 12, 23, 20, 50, 52e7)},
		{"1996-12-19T16:39:57-08:00", utc(1996, 12, 20, 0, 39, 57, 0)},
		// The leap second inserted at the end of 1990, clamped to :59.
		{"1990-12-31T23:59:60Z", utc(1990, 12, 31, 23, 59, 59, 0)},
		{"1990-12-31T15:59:60-08:00", utc(1990, 12, 31, 23, 59, 59, 0)},
		// Noon in the Netherlands, whose offset was +00:19:32.13, with
		// the closest offset RFC 3339 can write.
		{"1937-01-01T12:00:27.87+00:20", utc(1937, 1, 1, 11, 40, 27, 87e7)},
	}
	for _, tt := range tests {
		got, err := ParseTime(tt.in, nil)
		if err != nil {
			t.Errorf("ParseTime(%q): %v", tt.in, err)
			continue
		}
		if !got.Equal(tt.want) {
			t.Errorf("ParseTime(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseTimeRejected(t *testing.T) {
	tests := []struct {
		in     string
		loc    *time.Location
		offset int
		msg    string
	}{
		{"", time.UTC, 0, `expected 4-digit year, found end of input`},
		{"26-11-19", time.UTC, 2, `expected 4-digit year, found '-'`},
		{" 2026-11-19", time.UTC, 0, `expected 4-digit year, found ' '`},
		{"+002026-11-19", time.UTC, 0, `expected 4-digit year, found '+'`},
		{"２０２６-11-19", time.UTC, 0, `expected 4-digit year, found '２'`},
		{"2026", time.UTC, 4, `expected '-', 'W' or digit after year, found end of input`},
		{"2026/11/19", time.UTC, 4, `expected '-', 'W' or digit after year, found '/'`},
		{"2026-11", time.UTC, 7, `expected '-' before day of month, found end of input`},
		{"202611", time.UTC, 4, `expected MMDD, DDD or Www after year in basic format`},
		{"2026-1119", time.UTC, 5, `expected month (MM-DD) or day of year (DDD) after "2026-"`},
		{"2026-11-1", time.UTC, 9, `expected 2-digit day, found end of input`},
		{"2026-13-01", time.UTC, 5, `month 13 is out of range 01-12`},
		{"2026-00-10", time.UTC, 5, `month 00 is out of range 01-12`},
		{"2026-11-00", time.UTC, 8, `day 00 is out of range 01-30 for 2026-11`},
		{"2026-02-29", time.UTC, 8, `day 29 is out of range 01-28 for 2026-02`},
		{"1900-02-29", time.UTC, 8, `day 29 is out of range 01-28 for 1900-02`},
		{"2026-04-31", time.UTC, 8, `day 31 is out of range 01-30 for 2026-04`},
		{"2026-000", time.UTC, 5, `day of year 000 is out of range 001-365 for 2026`},
		{"2026-366", time.UTC, 5, `day of year 366 is out of range 001-365 for 2026`},
		{"2026-W00-1", time.UTC, 6, `week 00 is out of range 01-53 for 2026`},
		{"2021-W53-1", time.UTC, 6, `week 53 is out of range 01-52 for 2021`},
		{"2026-W47-0", time.UTC, 9, `day of week 0 is out of range 1-7 (Monday-Sunday)`},
		{"2026-W47-8", time.UTC, 9, `day of week 8 is out of range 1-7 (Monday-Sunday)`},
		{"2026-W47", time.UTC, 8, `expected '-' before day of week, found end of input`},
		{"2026W47", time.UTC, 7, `expected 1-digit day of week, found end of input`},
		{"2026-W474", time.UTC, 8, `expected '-' before day of week, found '4'`},
		{"2026W47-4", time.UTC, 7, `extended-format day of week in a basic-format date`},
		{"2026-w47-4", time.UTC, 5, `expected month (MM-DD) or day of year (DDD) after "2026-"`},

		{"2026-11-19Z", time.UTC, 10, `expected 'T' or end of input after date, found 'Z'`},
		{"2026-11-19X10:00", time.UTC, 10, `expected 'T' or end of input after date, found 'X'`},
		{"2026-11-19T", time.UTC, 11, `expected 2-digit hour, found end of input`},
		{"2026-11-19T10", time.UTC, 13, `expected minutes after hour, found end of input`},
		{"2026-11-19T10Z", time.UTC, 13, `expected minutes after hour, found 'Z'`},
		{"2026-11-19T1030", time.UTC, 13, `basic-format time in an extended-format value (expected ':')`},
		{"2026-11-19T10:3000", time.UTC, 16, `basic-format seconds in an extended-format value (expected ':')`},
		{"20261119T10:30", time.UTC, 11, `extended-format time in a basic-format value`},
		{"20261119T1030:00", time.UTC, 13, `extended-format seconds in a basic-format value`},
		{"2026-11-19T10:30:00+0100", nil, 22, `basic-format offset in an extended-format value (expected ':')`},
		{"20261119T103000+01:00", nil, 18, `extended-format offset in a basic-format value`},
		{"2026-11-19T10:30.5", time.UTC, 16, `a decimal fraction is only supported on seconds`},
		{"2026-11-19T10,5", time.UTC, 13, `expected minutes after hour, found ','`},
		{"2026-11-19T10:00:00.Z", nil, 20, `expected digit after decimal sign, found 'Z'`},
		{"2026-11-19T25:00Z", nil, 11, `hour 25 is out of range 00-24`},
		{"2026-11-19T24:01Z", nil, 11, `hour 24 is only valid as 24:00:00, the end of the day`},
		{"2026-11-19T24:00:01Z", nil, 11, `hour 24 is only valid as 24:00:00, the end of the day`},
		{"2026-11-19T24:00:00.5Z", nil, 11, `hour 24 is only valid as 24:00:00, the end of the day`},
		{"2026-11-19T10:60Z", nil, 14, `minute 60 is out of range 00-59`},
		{"2026-11-19T10:00:61Z", nil, 17, `second 61 is out of range 00-60`},
		{"2026-11-19T10:59:60Z", nil, 17, `leap second is only valid at 23:59:60 UTC on the last day of a month`},
		{"2026-11-19T23:59:60Z", nil, 17, `leap second is only valid at 23:59:60 UTC on the last day of a month`},
		{"2016-12-31T23:59:60+01:00", nil, 17, `leap second is only valid at 23:59:60 UTC on the last day of a month`},
		{"2026-11-19T10:00:00+24:00", nil, 20, `offset hour 24 is out of range 00-23`},
		{"2026-11-19T10:00:00+01:60", nil, 23, `offset minute 60 is out of range 00-59`},
		{"2026-11-19T10:00:00+1", nil, 21, `expected 2-digit offset hour, found end of input`},
		{"2026-11-19T10:00:00+01:", nil, 23, `expected 2-digit offset minute, found end of input`},
		{"2026-11-19T10:00:00 ", time.UTC, 19, `unexpected ' ' after time`},
		{"2026-11-19T10:00:00ZZ", nil, 20, `unexpected 'Z' after time`},
		{"2026-11-19T10:00:00+01:00[Europe/Stockholm]", nil, 25, `unexpected '[' after time`},
		{"2026-11-19T10:00:00 UTC", nil, 19, `unexpected ' ' after time`},

		{"2026-11-19", nil, 10, `a date without a UTC offset needs a location, and loc is nil`},
		{"2026-11-19T10:00:00", nil, 19, `a time without a UTC offset needs a location, and loc is nil`},
		{"9999-12-31T24:00Z", nil, 0, `year 10000 is outside the range 0000-9999`},
		{"9999-W52-6", time.UTC, 0, `year 10000 is outside the range 0000-9999`},
		{"9999-12-31T23:00-01:00", nil, -1, ""}, // accepted: formats in its own zone
	}
	for _, tt := range tests {
		got, err := ParseTime(tt.in, tt.loc)
		if tt.offset < 0 {
			if err != nil {
				t.Errorf("ParseTime(%q): %v", tt.in, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("ParseTime(%q) = %v, want error %q", tt.in, got, tt.msg)
			continue
		}
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("ParseTime(%q) error %T is not a *ParseError", tt.in, err)
			continue
		}
		if pe.Input != tt.in || pe.Offset != tt.offset || pe.Msg != tt.msg {
			t.Errorf("ParseTime(%q):\n got offset %d: %s\nwant offset %d: %s", tt.in, pe.Offset, pe.Msg, tt.offset, tt.msg)
		}
	}
}

func TestParseErrorMessage(t *testing.T) {
	_, err := ParseTime("2026-13-01\x1b[31m", time.UTC)
	want := `iso8601: invalid time "2026-13-01\x1b[31m" at offset 5: month 13 is out of range 01-12`
	if err == nil || err.Error() != want {
		t.Errorf("error = %v\nwant    %s", err, want)
	}
	_, err = ParseTime("2026-11-19T10:30:00+0100", nil)
	want = `iso8601: invalid time "2026-11-19T10:30:00+0100" at offset 22: basic-format offset in an extended-format value (expected ':')`
	if err == nil || err.Error() != want {
		t.Errorf("error = %v\nwant    %s", err, want)
	}
}

func TestFormatTime(t *testing.T) {
	newYork := mustLoad(t, "America/New_York")
	tests := []struct {
		in   time.Time
		want string
	}{
		{utc(2026, 11, 19, 10, 30, 0, 0), "2026-11-19T10:30:00Z"},
		{utc(2026, 11, 19, 10, 30, 0, 5e8), "2026-11-19T10:30:00.5Z"},
		{utc(2026, 11, 19, 10, 30, 0, 123456789), "2026-11-19T10:30:00.123456789Z"},
		{utc(2026, 11, 19, 10, 30, 0, 1000), "2026-11-19T10:30:00.000001Z"},
		{time.Date(2026, 11, 19, 10, 30, 0, 0, time.FixedZone("IST", 5*3600+1800)), "2026-11-19T10:30:00+05:30"},
		{time.Date(2026, 11, 19, 10, 30, 0, 0, time.FixedZone("", -8*3600)), "2026-11-19T10:30:00-08:00"},
		{time.Date(2026, 11, 19, 10, 30, 0, 0, time.FixedZone("", 0)), "2026-11-19T10:30:00Z"},
		// Offsets RFC 3339 cannot write are converted to UTC. Until 1883,
		// New York kept local mean time, 4:56:02 behind UTC.
		{time.Date(1850, 1, 1, 12, 0, 0, 0, newYork), "1850-01-01T16:56:02Z"},
		{time.Date(2026, 11, 19, 10, 30, 0, 0, time.FixedZone("", 90)), "2026-11-19T10:28:30Z"},
		{time.Date(2026, 11, 19, 10, 30, 0, 0, time.FixedZone("", 25*3600)), "2026-11-18T09:30:00Z"},
	}
	for _, tt := range tests {
		if got := FormatTime(tt.in); got != tt.want {
			t.Errorf("FormatTime(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTimeRoundTrip(t *testing.T) {
	newYork := mustLoad(t, "America/New_York")
	stockholm := mustLoad(t, "Europe/Stockholm")
	times := []time.Time{
		utc(2026, 11, 19, 10, 30, 0, 0),
		utc(2026, 11, 19, 10, 30, 0, 1),
		utc(0, 1, 1, 0, 0, 0, 0),
		utc(9999, 12, 31, 23, 59, 59, 999999999),
		time.Date(2026, 3, 8, 3, 0, 0, 0, newYork),
		time.Date(2026, 10, 25, 2, 30, 0, 0, stockholm),
		time.Date(1850, 1, 1, 12, 0, 27, 870000000, newYork),
		time.Date(2026, 11, 19, 10, 30, 0, 0, time.FixedZone("", -(9*3600+30*60))),
	}
	for _, want := range times {
		s := FormatTime(want)
		got, err := ParseTime(s, nil)
		if err != nil {
			t.Errorf("ParseTime(FormatTime(%v) = %q): %v", want, s, err)
			continue
		}
		if !got.Equal(want) {
			t.Errorf("ParseTime(%q) = %v, want %v", s, got, want)
		}
		if again := FormatTime(got); again != s {
			t.Errorf("FormatTime(ParseTime(%q)) = %q", s, again)
		}
	}
}
