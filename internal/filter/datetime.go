package filter

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// parseDateTime reads a date and time in the forms gcloud's
// times.ParseDateTime (dateutil) is known to read the same way, in its
// extended ISO 8601 format with ASCII digits:
//
//	YYYY-MM-DD
//	YYYY-MM-DDTHH:MM[:SS[.F...]][Z|±HH:MM|±HHMM]
//
// with T or a space between the date and the time. A value without a
// UTC offset is in loc (gcloud uses the local time zone), and a local
// time that a daylight saving change skips or repeats is an error, as is
// anything else (dateutil reads many more forms, which clix does not
// accept rather than read differently). Fractions of a second are
// truncated to microseconds, the precision of Python's datetime.
func parseDateTime(s string, loc *time.Location) (time.Time, error) {
	p := dtParser{s: s}
	year := p.digits(4)
	p.expect('-')
	month := p.digits(2)
	p.expect('-')
	day := p.digits(2)
	hour, minute, second, micro := 0, 0, 0, 0
	zone := loc
	if !p.end() {
		if !p.accept('T') && !p.accept(' ') {
			p.fail()
		}
		hour = p.digits(2)
		p.expect(':')
		minute = p.digits(2)
		if p.accept(':') {
			second = p.digits(2)
			if p.accept('.') {
				start := p.i
				for !p.end() && isDigit(p.s[p.i]) {
					p.i++
				}
				frac := p.s[start:p.i]
				if frac == "" {
					p.fail()
				}
				for len(frac) < 6 {
					frac += "0"
				}
				micro, _ = strconv.Atoi(frac[:6])
			}
		}
		if !p.end() {
			switch {
			case p.accept('Z'):
				zone = time.UTC
			case p.peek('+') || p.peek('-'):
				sign := 1
				if p.s[p.i] == '-' {
					sign = -1
				}
				p.i++
				oh := p.digits(2)
				p.accept(':')
				om := p.digits(2)
				if oh > 23 || om > 59 {
					p.fail()
				}
				zone = time.FixedZone("", sign*(oh*3600+om*60))
			default:
				p.fail()
			}
		}
	}
	if !p.end() {
		p.fail()
	}
	if p.err != nil {
		return time.Time{}, p.err
	}
	if year < 1 || month < 1 || month > 12 || day < 1 || day > daysIn(year, month) ||
		hour > 23 || minute > 59 || second > 59 {
		return time.Time{}, fmt.Errorf("%q is out of range", s)
	}
	t := time.Date(year, time.Month(month), day, hour, minute, second, micro*1000, zone)
	if zone == loc {
		if err := checkLocal(t, year, month, day, hour, minute, second); err != nil {
			return time.Time{}, fmt.Errorf("%q: %w", s, err)
		}
	}
	return t, nil
}

// checkLocal rejects a local time that does not exist (skipped by a
// daylight saving change) or exists twice (repeated by one).
func checkLocal(t time.Time, year, month, day, hour, minute, second int) error {
	if t.Year() != year || int(t.Month()) != month || t.Day() != day || t.Hour() != hour || t.Minute() != minute || t.Second() != second {
		return errors.New("the local time does not exist")
	}
	for d := -3 * time.Hour; d <= 3*time.Hour; d += 15 * time.Minute {
		if d == 0 {
			continue
		}
		u := t.Add(d)
		if u.Year() == year && int(u.Month()) == month && u.Day() == day && u.Hour() == hour && u.Minute() == minute && u.Second() == second {
			return errors.New("the local time is ambiguous")
		}
	}
	return nil
}

func daysIn(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

type dtParser struct {
	s   string
	i   int
	err error
}

func (p *dtParser) fail() {
	if p.err == nil {
		p.err = fmt.Errorf("%q is not YYYY-MM-DD, YYYY-MM-DDTHH:MM[:SS[.F]] or the same with Z or an offset such as +02:00", p.s)
	}
	p.i = len(p.s)
}

func (p *dtParser) end() bool { return p.i >= len(p.s) }

func (p *dtParser) peek(c byte) bool { return !p.end() && p.s[p.i] == c }

func (p *dtParser) accept(c byte) bool {
	if p.peek(c) {
		p.i++
		return true
	}
	return false
}

func (p *dtParser) expect(c byte) {
	if !p.accept(c) {
		p.fail()
	}
}

func (p *dtParser) digits(n int) int {
	if p.i+n > len(p.s) {
		p.fail()
		return 0
	}
	v := 0
	for k := 0; k < n; k++ {
		c := p.s[p.i+k]
		if !isDigit(c) {
			p.fail()
			return 0
		}
		v = v*10 + int(c-'0')
	}
	p.i += n
	return v
}
