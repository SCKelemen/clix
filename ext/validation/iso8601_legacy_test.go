package validation

// The implementations of ISO8601Time and ISO8601Duration before they used
// the iso8601 package (v2.8.0), kept to test that the accepted sets are
// unchanged, or changed exactly as documented.

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// legacyISO8601Time is ISO8601Time as of v2.8.0.
func legacyISO8601Time(value string) error {
	if value == "" {
		return errors.New("time cannot be empty")
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return errors.New(`must be an RFC 3339 time such as "2026-11-19T08:30:00Z"`)
	}
	return nil
}

// legacyParseISODuration is parseISODuration as of v2.8.0.
func legacyParseISODuration(value string) (time.Duration, error) {
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
		n, err := legacyParseDigits(digits)
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
			n, err := legacyParseDigits(whole)
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
				ns, err := legacyParseFraction(frac)
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

// legacyParseDigits parses a non-empty run of ASCII digits.
func legacyParseDigits(s string) (uint64, error) {
	if s == "" || strings.TrimLeft(s, "0123456789") != "" {
		return 0, errors.New("not digits")
	}
	return strconv.ParseUint(s, 10, 64)
}

// legacyParseFraction turns the digits after a decimal sign into nanoseconds,
// ignoring digits beyond the ninth.
func legacyParseFraction(s string) (uint64, error) {
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
