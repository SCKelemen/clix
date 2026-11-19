package validation

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SCKelemen/clix/v2/iso8601"
)

// rfc3339Example starts every ISO8601Time error.
const rfc3339Example = `must be an RFC 3339 time such as "2026-11-19T08:30:00Z"`

// ISO8601Time validates a timestamp in RFC 3339, the profile of ISO 8601
// used on the internet, such as "2026-11-19T08:30:00Z" or
// "2026-11-19T10:30:00.5+02:00". A time zone (Z or an offset) is required.
// https://datatracker.ietf.org/doc/html/rfc3339#section-5.6
//
// It accepts exactly the RFC 3339 date-time
//
//	YYYY-MM-DDThh:mm:ss[.f...](Z|+hh:mm|-hh:mm)
//
// with every field two digits (the year four) and in range, an uppercase
// "T" and "Z", and a fraction of a second after a "." only. Every value it
// accepts reads the same with time.Parse(time.RFC3339, value) and with
// iso8601.ParseTime(value, nil). It does not accept what RFC 3339 allows
// but time.Parse rejects (a lowercase "t" or "z", a space instead of "T",
// a leap second such as 23:59:60), nor the other ISO 8601 forms that
// iso8601.ParseTime reads (the basic format, week and ordinal dates, a
// time without an offset). Up to v2.8.0 it also accepted four forms that
// are not RFC 3339 but that time.Parse lets through
// (https://go.dev/issue/54580): a one-digit hour, a "," before the
// fraction, and an offset of 24 hours or of 60 minutes.
func ISO8601Time(value string) error {
	if value == "" {
		return errors.New("time cannot be empty")
	}
	if err := rfc3339Syntax(value); err != nil {
		return fmt.Errorf("%s: %w", rfc3339Example, err)
	}
	// The syntax is right, so iso8601.ParseTime checks the ranges of the
	// fields: month 13, February 30, an offset hour of 24, and so on.
	if _, err := iso8601.ParseTime(value, nil); err != nil {
		return fmt.Errorf("%s: %w", rfc3339Example, err)
	}
	// iso8601.ParseTime also reads 24:00:00, the end of the day, and leap
	// seconds; RFC 3339 §5.6 has hours 00-23, and time.Time cannot
	// represent a leap second.
	if value[11:13] == "24" {
		return fmt.Errorf("%s: hour 24 is out of range 00-23", rfc3339Example)
	}
	if value[17:19] == "60" {
		return fmt.Errorf("%s: a leap second (second 60) is not supported", rfc3339Example)
	}
	return nil
}

// rfc3339Syntax checks that s has the syntax of an RFC 3339 date-time
// (section 5.6) with an uppercase "T" and "Z", leaving the ranges of the
// fields to iso8601.ParseTime.
func rfc3339Syntax(s string) error {
	const layout = "dddd-dd-ddTdd:dd:dd" // d is a digit
	for i := range len(layout) {
		if i >= len(s) {
			return errors.New("it ends before the seconds (YYYY-MM-DDThh:mm:ss)")
		}
		if c := s[i]; layout[i] == 'd' && !isDigit(c) || layout[i] != 'd' && c != layout[i] {
			return fmt.Errorf("expected YYYY-MM-DDThh:mm:ss, found %q at offset %d", c, i)
		}
	}
	rest := s[len(layout):]
	if strings.HasPrefix(rest, ".") {
		n := 1
		for n < len(rest) && isDigit(rest[n]) {
			n++
		}
		if n == 1 {
			return errors.New(`expected a digit after "."`)
		}
		rest = rest[n:]
	}
	switch {
	case rest == "Z":
		return nil
	case rest == "":
		return errors.New("it has no time zone (add Z or an offset such as +02:00)")
	case len(rest) == 6 && (rest[0] == '+' || rest[0] == '-') && isDigit(rest[1]) && isDigit(rest[2]) && rest[3] == ':' && isDigit(rest[4]) && isDigit(rest[5]):
		return nil
	}
	return fmt.Errorf("expected Z or an offset such as +02:00, found %q", rest)
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// ISO8601Duration validates an ISO 8601 duration of days, hours, minutes
// and seconds: "PT1M30S", "P1DT12H", "PT0.5S", "-PT2H". The forms are
// PnDTnHnMnS, with any of the parts left out (but at least one present,
// and at least one after T when T is present), an optional leading sign,
// and an optional decimal fraction (with "." or ",") on the seconds.
// Designators are upper case. Years, months and weeks are not accepted,
// since their length depends on the calendar; a day counts as 24 hours.
// The duration must fit in a time.Duration (about 292 years).
//
// ISO 8601-1:2019 section 5.5.2.4; the sign follows ISO 8601-2:2019
// section 4.4.1.9.
//
// These are the durations iso8601.ParseDuration reads that have only
// days, hours, minutes and seconds, a fraction only on the seconds, and an
// exact length that fits in a time.Duration (iso8601.Duration.Exact), plus
// the same with a leading "+", which ISO8601Duration has always accepted.
func ISO8601Duration(value string) error {
	if value == "" {
		return errors.New("duration cannot be empty")
	}
	if _, err := parseISODuration(value); err != nil {
		return fmt.Errorf(`must be an ISO 8601 duration such as "PT1M30S": %w`, err)
	}
	return nil
}

// parseISODuration parses the durations ISO8601Duration accepts with
// iso8601.ParseDuration, and returns them as a time.Duration.
func parseISODuration(value string) (time.Duration, error) {
	s := value
	plus := strings.HasPrefix(s, "+")
	if plus {
		// ISO 8601-2 has only a minus sign; "+" is ISO8601Duration's own.
		s = s[1:]
		if !strings.HasPrefix(s, "P") {
			return 0, errors.New(`"+" must be followed by "P"`)
		}
	}
	d, err := iso8601.ParseDuration(s)
	if err != nil {
		var pe *iso8601.ParseError
		if plus && errors.As(err, &pe) {
			// Point at the offending byte of what was given.
			pe.Input, pe.Offset = value, pe.Offset+1
		}
		return 0, err
	}
	// The date part may hold Y, M (months), W and D designators.
	date, _, _ := strings.Cut(s, "T")
	if strings.ContainsAny(date, "YMW") {
		return 0, errors.New("years, months and weeks are not supported, since their length depends on the calendar")
	}
	// iso8601.ParseDuration allows a fraction on the last component only,
	// which is the seconds exactly when the duration ends in S.
	if strings.ContainsAny(s, ".,") && !strings.HasSuffix(s, "S") {
		return 0, errors.New("only seconds may have a fraction")
	}
	exact, ok := d.Exact()
	if !ok {
		return 0, errors.New("it is out of range")
	}
	return exact, nil
}
