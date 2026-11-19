package iso8601

import (
	"time"
)

// ParseTime parses an ISO 8601 date or date and time of day. The accepted
// forms are listed in the package documentation; in short:
//
//	2026-11-19  20261119  2026-323  2026323  2026-W47-4  2026W474
//	2026-11-19T10:30  2026-11-19T10:30:00.5Z  20261119T103000,5+0100
//
// A value with a UTC offset ("Z", "±hh", "±hh:mm" or "±hhmm") is returned
// in [time.UTC] when the offset is zero and in a [time.FixedZone] otherwise;
// loc is not consulted and may be nil. A value without an offset, which
// includes every date without a time, is a local time and is interpreted in
// loc, which must then be non-nil.
//
// A local time that a transition in loc (such as a daylight saving change)
// repeats or skips is resolved as RFC 5545 §3.3.5 specifies: a repeated
// time means its first occurrence, and a skipped time is read with the UTC
// offset in effect before the gap, so America/New_York's skipped
// 2026-03-08T02:30 is 03:30 EDT. [time.Date] leaves both choices
// unspecified.
//
// A date alone means the first instant of that day in loc. That is 00:00,
// unless loc skips midnight (America/Santiago starts daylight saving time
// by skipping 2026-09-06T00:00 to 01:00), when the day starts as the gap
// ends; if loc skips the whole date (Pacific/Apia skipped 2011-12-30), it
// is the first instant after it.
//
// The hour 24 is accepted only as 24:00, 24:00:00 or 24:00:00 with an
// all-zero fraction, and means the end of the day: the first instant of
// the next day, as above.
//
// A leap second (second 60) is clamped to second 59 of the same minute,
// keeping any fraction, because [time.Time] cannot represent it. It is
// accepted only where a leap second can occur, at 23:59:60 UTC on the last
// day of a month; anywhere else it is an error.
//
// The result's calendar year, as [FormatTime] would write it, must lie in
// 0000 through 9999, so that every result can be formatted and parsed
// again.
//
// The error, if any, is a [*ParseError] that names the input and the byte
// offset of the problem.
func ParseTime(s string, loc *time.Location) (time.Time, error) {
	p := &scanner{s: s, kind: "time"}

	year, month, day, extended, err := p.date()
	if err != nil {
		return time.Time{}, err
	}

	if p.pos == len(s) {
		if loc == nil {
			return time.Time{}, p.errorf(p.pos, "a date without a UTC offset needs a location, and loc is nil")
		}
		return checkRange(p, startOfDay(year, month, day, loc))
	}

	// ISO 8601-1:2019 separates the date and the time with "T" (§5.4).
	// RFC 3339 §5.6 permits lowercase "t" and notes that applications may
	// use a space instead, "for the sake of readability".
	switch c := p.peek(); c {
	case 'T', 't', ' ':
		p.pos++
	default:
		return time.Time{}, p.errorf(p.pos, "expected 'T' or end of input after date, found %s", p.found(p.pos))
	}

	tod, err := p.timeOfDay(extended)
	if err != nil {
		return time.Time{}, err
	}

	zone, hasOffset, err := p.offset(extended)
	if err != nil {
		return time.Time{}, err
	}
	if p.pos != len(s) {
		return time.Time{}, p.errorf(p.pos, "unexpected %s after time", p.found(p.pos))
	}
	if !hasOffset {
		if loc == nil {
			return time.Time{}, p.errorf(p.pos, "a time without a UTC offset needs a location, and loc is nil")
		}
		zone = loc
	}

	if tod.hour == 24 {
		return checkRange(p, startOfDay(year, month, day+1, zone))
	}

	sec := tod.sec
	if sec == 60 {
		sec = 59
	}
	t, _ := localTime(time.Date(year, month, day, tod.hour, tod.min, sec, tod.nsec, time.UTC), zone)
	if tod.sec == 60 {
		// ITU-R TF.460-6 §2.1 places a leap second at the end of a UTC
		// month, so it is valid only at 23:59:60 UTC on a month's last day.
		u := t.UTC()
		if u.Hour() != 23 || u.Minute() != 59 || u.Day() != daysIn(u.Month(), u.Year()) {
			return time.Time{}, p.errorf(tod.secPos, "leap second is only valid at 23:59:60 UTC on the last day of a month")
		}
	}
	return checkRange(p, t)
}

// localTime returns the instant at which the wall clock in loc reads the
// date and time of day in w's UTC fields. A reading that occurs twice,
// because the clocks went back, is resolved to its first occurrence, and
// one that does not occur, because they went forward, is read with the
// UTC offset in effect before the gap (RFC 5545 §3.3.5,
// https://datatracker.ietf.org/doc/html/rfc5545#section-3.3.5). For a
// skipped reading, gapEnd is the transition that ends the gap; otherwise it
// is the zero Time.
//
// time.Date leaves both cases unspecified, and in practice resolves them
// one way east of UTC and the other way west of it.
func localTime(w time.Time, loc *time.Location) (t, gapEnd time.Time) {
	guess := time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), w.Nanosecond(), loc)

	// Every instant that reads w lies in the zone period of guess or in a
	// neighboring one. Probe one instant in each.
	var probes [3]time.Time
	probes[0] = guess
	n := 1
	start, end := guess.ZoneBounds()
	if !start.IsZero() {
		probes[n] = start.Add(-time.Nanosecond)
		n++
	}
	if !end.IsZero() {
		probes[n] = end
		n++
	}

	found := false
	for _, p := range probes[:n] {
		c, s, e := reading(w, p)
		if (s.IsZero() || !c.Before(s)) && (e.IsZero() || c.Before(e)) && (!found || c.Before(t)) {
			t, found = c, true
		}
	}
	if found {
		return t.In(loc), time.Time{}
	}

	// No period shows w, so it falls in a gap: it is past the end of one
	// period, read with that period's offset, and before the start of the
	// next, read with the next one's.
	for _, p := range probes[:n] {
		c, _, e := reading(w, p)
		if e.IsZero() || c.Before(e) {
			continue
		}
		if next, _, _ := reading(w, e); next.Before(e) {
			return c.In(loc), e
		}
	}
	return guess, time.Time{} // unreachable with consistent zone data
}

// reading returns the instant at which a clock showing the UTC offset in
// effect at p reads w's UTC fields, and the bounds of p's zone period.
func reading(w, p time.Time) (c, start, end time.Time) {
	_, off := p.Zone()
	start, end = p.ZoneBounds()
	return w.Add(-time.Duration(off) * time.Second), start, end
}

// startOfDay returns the first instant of the date year-month-day in loc:
// local midnight, or, if loc skips midnight, the end of that gap.
func startOfDay(year int, month time.Month, day int, loc *time.Location) time.Time {
	t, gapEnd := localTime(time.Date(year, month, day, 0, 0, 0, 0, time.UTC), loc)
	if !gapEnd.IsZero() {
		return gapEnd
	}
	return t
}

// FormatTime formats t in the RFC 3339 §5.6 internet profile of ISO 8601:
// "2006-01-02T15:04:05Z07:00" with a fractional second, using "." and
// without trailing zeros, only when the nanoseconds are non-zero. It is
// equivalent to t.Format([time.RFC3339Nano]) with one exception: RFC 3339
// offsets have whole minutes and fewer than 24 hours, so a t whose offset
// has a seconds part (such as local mean time in historical zone data) or
// is 24 hours or more is written in UTC instead, preserving the instant.
//
// RFC 3339 has four-digit years. For a year outside 0000 through 9999 the
// result follows [time.Time.Format] and is not valid RFC 3339.
func FormatTime(t time.Time) string {
	return formatZone(t).Format(time.RFC3339Nano)
}

// formatZone returns t in the location FormatTime writes it in.
func formatZone(t time.Time) time.Time {
	_, off := t.Zone()
	if off%60 != 0 || off <= -24*3600 || off >= 24*3600 {
		return t.UTC()
	}
	return t
}

// checkRange rejects a result whose year FormatTime could not write as four
// digits, which normalization (24:00, week dates, offsets) can produce.
func checkRange(p *scanner, t time.Time) (time.Time, error) {
	if y := formatZone(t).Year(); y < 0 || y > 9999 {
		return time.Time{}, p.errorf(0, "year %d is outside the range 0000-9999", y)
	}
	return t, nil
}

// date parses a calendar, ordinal or week date (ISO 8601-1:2019 §5.2.2,
// §5.2.3 and §5.2.4) with a four-digit year, and reports whether it used the
// extended format.
func (p *scanner) date() (year int, month time.Month, day int, extended bool, err error) {
	year, err = p.fixed(4, "year")
	if err != nil {
		return
	}
	switch c := p.peek(); {
	case c == '-':
		extended = true
		p.pos++
		if p.peek() == 'W' {
			p.pos++
			month, day, year, err = p.weekDate(year, true)
			return
		}
		switch p.digitRun() {
		case 2:
			month, day, err = p.calendarDate(year, true)
		case 3:
			month, day, err = p.ordinalDate(year)
		default:
			err = p.errorf(p.pos, "expected month (MM-DD) or day of year (DDD) after \"%04d-\"", year)
		}
	case c == 'W':
		p.pos++
		month, day, year, err = p.weekDate(year, false)
	case isDigit(c):
		switch p.digitRun() {
		case 4:
			month, day, err = p.calendarDate(year, false)
		case 3:
			month, day, err = p.ordinalDate(year)
		default:
			err = p.errorf(p.pos, "expected MMDD, DDD or Www after year in basic format")
		}
	default:
		err = p.errorf(p.pos, "expected '-', 'W' or digit after year, found %s", p.found(p.pos))
	}
	return
}

func (p *scanner) calendarDate(year int, extended bool) (time.Month, int, error) {
	at := p.pos
	m, err := p.fixed(2, "month")
	if err != nil {
		return 0, 0, err
	}
	if m < 1 || m > 12 {
		return 0, 0, p.errorf(at, "month %02d is out of range 01-12", m)
	}
	if extended {
		if p.peek() != '-' {
			return 0, 0, p.errorf(p.pos, "expected '-' before day of month, found %s", p.found(p.pos))
		}
		p.pos++
	}
	at = p.pos
	d, err := p.fixed(2, "day")
	if err != nil {
		return 0, 0, err
	}
	if n := daysIn(time.Month(m), year); d < 1 || d > n {
		return 0, 0, p.errorf(at, "day %02d is out of range 01-%02d for %04d-%02d", d, n, year, m)
	}
	return time.Month(m), d, nil
}

func (p *scanner) ordinalDate(year int) (time.Month, int, error) {
	at := p.pos
	d, err := p.fixed(3, "day of year")
	if err != nil {
		return 0, 0, err
	}
	n := 365
	if isLeap(year) {
		n = 366
	}
	if d < 1 || d > n {
		return 0, 0, p.errorf(at, "day of year %03d is out of range 001-%03d for %04d", d, n, year)
	}
	// time.Date normalizes January 32 and beyond into later months.
	t := time.Date(year, time.January, d, 0, 0, 0, 0, time.UTC)
	return t.Month(), t.Day(), nil
}

// weekDate parses the part of a week date after "W". Week 1 of a year is
// the week, starting on Monday, that contains the year's first Thursday, so
// it can start in the previous year and the last week can end in the next.
// The calendar year of the result is returned with its month and day.
func (p *scanner) weekDate(year int, extended bool) (time.Month, int, int, error) {
	at := p.pos
	w, err := p.fixed(2, "week")
	if err != nil {
		return 0, 0, 0, err
	}
	if n := weeksIn(year); w < 1 || w > n {
		return 0, 0, 0, p.errorf(at, "week %02d is out of range 01-%02d for %04d", w, n, year)
	}
	if extended {
		if p.peek() != '-' {
			return 0, 0, 0, p.errorf(p.pos, "expected '-' before day of week, found %s", p.found(p.pos))
		}
		p.pos++
	} else if p.peek() == '-' {
		return 0, 0, 0, p.errorf(p.pos, "extended-format day of week in a basic-format date")
	}
	at = p.pos
	wd, err := p.fixed(1, "day of week")
	if err != nil {
		return 0, 0, 0, err
	}
	if wd < 1 || wd > 7 {
		return 0, 0, 0, p.errorf(at, "day of week %d is out of range 1-7 (Monday-Sunday)", wd)
	}
	t := week1Monday(year).AddDate(0, 0, (w-1)*7+wd-1)
	return t.Month(), t.Day(), t.Year(), nil
}

// week1Monday returns the Monday that starts ISO week 1 of year: the Monday
// on or before January 4, which is always in week 1.
func week1Monday(year int) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	back := (int(jan4.Weekday()) + 6) % 7 // days since Monday
	return jan4.AddDate(0, 0, -back)
}

// weeksIn returns 52 or 53, the number of ISO weeks in year. December 28
// is always in the last week.
func weeksIn(year int) int {
	_, w := time.Date(year, time.December, 28, 0, 0, 0, 0, time.UTC).ISOWeek()
	return w
}

func daysIn(m time.Month, year int) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func isLeap(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

type timeOfDay struct {
	hour, min, sec, nsec int
	secPos               int
}

// timeOfDay parses hh:mm[:ss[.f]] or hhmm[ss[,f]] (ISO 8601-1:2019 §5.3),
// in the same format as the date. Reduced precision (hh alone, §5.3.1.3)
// and fractions of hours or minutes are not supported.
func (p *scanner) timeOfDay(extended bool) (timeOfDay, error) {
	var tod timeOfDay
	hourPos := p.pos
	h, err := p.fixed(2, "hour")
	if err != nil {
		return tod, err
	}
	tod.hour = h

	switch c := p.peek(); {
	case extended && c == ':':
		p.pos++
	case extended && isDigit(c):
		return tod, p.errorf(p.pos, "basic-format time in an extended-format value (expected ':')")
	case !extended && c == ':':
		return tod, p.errorf(p.pos, "extended-format time in a basic-format value")
	case !extended && isDigit(c):
	default:
		return tod, p.errorf(p.pos, "expected minutes after hour, found %s", p.found(p.pos))
	}
	minPos := p.pos
	if tod.min, err = p.fixed(2, "minute"); err != nil {
		return tod, err
	}

	hasSec := false
	switch c := p.peek(); {
	case extended && c == ':':
		p.pos++
		hasSec = true
	case extended && isDigit(c):
		return tod, p.errorf(p.pos, "basic-format seconds in an extended-format value (expected ':')")
	case !extended && c == ':':
		return tod, p.errorf(p.pos, "extended-format seconds in a basic-format value")
	case !extended && isDigit(c):
		hasSec = true
	}
	if hasSec {
		tod.secPos = p.pos
		if tod.sec, err = p.fixed(2, "second"); err != nil {
			return tod, err
		}
	}

	// ISO 8601-1:2019 §3.1.3.9 allows a comma or a full stop as the
	// decimal sign.
	fracZero := true
	if c := p.peek(); c == '.' || c == ',' {
		if !hasSec {
			return tod, p.errorf(p.pos, "a decimal fraction is only supported on seconds")
		}
		p.pos++
		n := p.digitRun()
		if n == 0 {
			return tod, p.errorf(p.pos, "expected digit after decimal sign, found %s", p.found(p.pos))
		}
		// Digits past the ninth are below a nanosecond and are truncated,
		// as time.Parse does.
		for i := range 9 {
			tod.nsec *= 10
			if i < n {
				tod.nsec += int(p.s[p.pos+i] - '0')
			}
		}
		for i := range n {
			if p.s[p.pos+i] != '0' {
				fracZero = false
			}
		}
		p.pos += n
	}

	switch {
	case tod.hour == 24:
		// ISO 8601-1:2019/Amd 1:2022 restored 24:00:00 as the end of a
		// calendar day; RFC 3339 Appendix A allows time-hour 00-24.
		if tod.min != 0 || tod.sec != 0 || !fracZero {
			return tod, p.errorf(hourPos, "hour 24 is only valid as 24:00:00, the end of the day")
		}
	case tod.hour > 24:
		return tod, p.errorf(hourPos, "hour %02d is out of range 00-24", tod.hour)
	case tod.min > 59:
		return tod, p.errorf(minPos, "minute %02d is out of range 00-59", tod.min)
	case tod.sec > 60:
		return tod, p.errorf(tod.secPos, "second %02d is out of range 00-60", tod.sec)
	}
	return tod, nil
}

// offset parses an optional UTC designator "Z" (ISO 8601-1:2019 §5.3.3) or
// numeric offset ±hh, ±hh:mm or ±hhmm (§5.3.4), in the same format as the
// rest of the value. RFC 3339 §5.6 permits lowercase "z". An offset of
// -00:00, which RFC 3339 §4.3 uses for "UTC, local offset unknown", is
// returned as UTC because time.Time cannot record the distinction.
func (p *scanner) offset(extended bool) (*time.Location, bool, error) {
	var sign int
	switch p.peek() {
	case 'Z', 'z':
		p.pos++
		return time.UTC, true, nil
	case '+':
		sign = 1
	case '-':
		sign = -1
	default:
		return nil, false, nil
	}
	p.pos++

	hourPos := p.pos
	h, err := p.fixed(2, "offset hour")
	if err != nil {
		return nil, false, err
	}
	if h > 23 {
		return nil, false, p.errorf(hourPos, "offset hour %02d is out of range 00-23", h)
	}
	m := 0
	switch c := p.peek(); {
	case extended && c == ':':
		p.pos++
	case extended && isDigit(c):
		return nil, false, p.errorf(p.pos, "basic-format offset in an extended-format value (expected ':')")
	case !extended && c == ':':
		return nil, false, p.errorf(p.pos, "extended-format offset in a basic-format value")
	case !extended && isDigit(c):
	default:
		// ±hh: hours only, valid in both formats.
		return zoneFor(sign * h * 3600), true, nil
	}
	minPos := p.pos
	if m, err = p.fixed(2, "offset minute"); err != nil {
		return nil, false, err
	}
	if m > 59 {
		return nil, false, p.errorf(minPos, "offset minute %02d is out of range 00-59", m)
	}
	return zoneFor(sign * (h*3600 + m*60)), true, nil
}

func zoneFor(offset int) *time.Location {
	if offset == 0 {
		return time.UTC
	}
	return time.FixedZone("", offset)
}
