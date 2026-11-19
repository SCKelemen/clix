package validation

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ISO8601Time validates a timestamp in RFC 3339, the profile of ISO 8601
// used on the internet, such as "2026-11-19T08:30:00Z" or
// "2026-11-19T10:30:00.5+02:00". A time zone (Z or an offset) is required.
// https://datatracker.ietf.org/doc/html/rfc3339#section-5.6
//
// TODO: switch to the iso8601 package (github.com/SCKelemen/clix/v2/iso8601)
// once it lands.
func ISO8601Time(value string) error {
	if value == "" {
		return errors.New("time cannot be empty")
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return errors.New(`must be an RFC 3339 time such as "2026-11-19T08:30:00Z"`)
	}
	return nil
}

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
// TODO: switch to the iso8601 package (github.com/SCKelemen/clix/v2/iso8601)
// once it lands.
func ISO8601Duration(value string) error {
	if value == "" {
		return errors.New("duration cannot be empty")
	}
	if _, err := parseISODuration(value); err != nil {
		return fmt.Errorf(`must be an ISO 8601 duration such as "PT1M30S": %w`, err)
	}
	return nil
}

// parseISODuration parses the durations ISO8601Duration accepts.
func parseISODuration(value string) (time.Duration, error) {
	s := value
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if s == "" || s[0] != 'P' {
		return 0, errors.New(`it must start with "P"`)
	}
	date, clock, hasT := strings.Cut(s[1:], "T")

	var total uint64
	parts := 0
	add := func(n, unit uint64) error {
		if n > math.MaxUint64/unit || total > math.MaxUint64-n*unit {
			return errors.New("it is out of range")
		}
		total += n * unit
		parts++
		return nil
	}

	if date != "" {
		digits, ok := strings.CutSuffix(date, "D")
		n, err := parseDigits(digits)
		if !ok || err != nil {
			return 0, fmt.Errorf("%q is not a number of days (years, months and weeks are not supported)", date)
		}
		if err := add(n, uint64(24*time.Hour)); err != nil {
			return 0, err
		}
	}
	if hasT {
		if clock == "" {
			return 0, errors.New(`"T" must be followed by hours, minutes or seconds`)
		}
		units := []struct {
			designator byte
			size       time.Duration
		}{{'H', time.Hour}, {'M', time.Minute}, {'S', time.Second}}
		next := 0
		for clock != "" {
			end := strings.IndexFunc(clock, func(r rune) bool { return (r < '0' || r > '9') && r != '.' && r != ',' })
			if end < 0 {
				return 0, fmt.Errorf("%q has no designator (H, M or S)", clock)
			}
			num, designator := clock[:end], clock[end]
			clock = clock[end+1:]
			i := next
			for i < len(units) && units[i].designator != designator {
				i++
			}
			if i == len(units) {
				return 0, fmt.Errorf("unexpected %q (hours, minutes and seconds go in that order, once each)", string(designator))
			}
			next = i + 1
			whole, frac, hasFrac := strings.Cut(strings.Replace(num, ",", ".", 1), ".")
			n, err := parseDigits(whole)
			if err != nil {
				return 0, fmt.Errorf("%q is not a number", num)
			}
			if err := add(n, uint64(units[i].size)); err != nil {
				return 0, err
			}
			if hasFrac {
				if designator != 'S' {
					return 0, errors.New("only seconds may have a fraction")
				}
				ns, err := parseFraction(frac)
				if err != nil {
					return 0, fmt.Errorf("%q is not a number", num)
				}
				parts--
				if err := add(ns, 1); err != nil {
					return 0, err
				}
			}
		}
	}
	if parts == 0 {
		return 0, errors.New("it has no days, hours, minutes or seconds")
	}

	limit := uint64(math.MaxInt64)
	if neg {
		limit++
	}
	if total > limit {
		return 0, errors.New("it is out of range")
	}
	if neg {
		return -time.Duration(total-1) - 1, nil // total may be 1<<63
	}
	return time.Duration(total), nil
}

// parseDigits parses a non-empty run of ASCII digits.
func parseDigits(s string) (uint64, error) {
	if s == "" || strings.TrimLeft(s, "0123456789") != "" {
		return 0, errors.New("not digits")
	}
	return strconv.ParseUint(s, 10, 64)
}

// parseFraction turns the digits after a decimal sign into nanoseconds,
// ignoring digits beyond the ninth.
func parseFraction(s string) (uint64, error) {
	if s == "" || strings.TrimLeft(s, "0123456789") != "" {
		return 0, errors.New("not digits")
	}
	if len(s) > 9 {
		s = s[:9]
	}
	ns, err := strconv.ParseUint(s, 10, 64)
	for i := len(s); i < 9; i++ {
		ns *= 10
	}
	return ns, err
}
