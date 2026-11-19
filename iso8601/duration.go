package iso8601

import (
	"errors"
	"math"
	"math/bits"
	"strconv"
	"time"
)

// Duration is an ISO 8601 duration (ISO 8601-1:2019 §5.5.2), such as
// P1Y2M3DT4H5M6.5S. Years, months, weeks and days are nominal: their length
// depends on where in the calendar they are applied, so they are kept as
// separate fields and only [Duration.AddTo] can resolve them. Hours,
// minutes and seconds are exact and are kept together in Time.
//
// A Duration's fields share one sign: a negative duration such as -P1DT2H
// has every non-zero field negative ({Days: -1, Time: -2 * time.Hour}).
// This mirrors ISO 8601-2:2019 §4.4.1.9, where a leading minus sign reverses
// the whole duration and every component is written as positive. Signed
// fields, rather than unsigned magnitudes and a separate sign flag, mean
// there is a single zero value, == compares durations, [FromStd] and
// [Duration.Exact] carry the sign of a [time.Duration] through unchanged,
// and code that reads d.Days directly cannot forget the sign.
//
// [ParseDuration] only produces durations whose fields share a sign. A
// Duration built by hand with mixed signs, such as {Days: 1, Time:
// -time.Hour}, still works with [Duration.AddTo] and [Duration.Exact], but
// it has no ISO 8601 representation: [Duration.MarshalText] rejects it and
// [Duration.String] writes each negative field with its own minus sign
// ("P1DT-1H"), which ParseDuration rejects.
//
// The zero Duration is the empty duration, written PT0S. Duration
// implements [encoding.TextMarshaler] and [encoding.TextUnmarshaler], so it
// reads and writes ISO 8601 text in JSON, YAML and other encoders that use
// those interfaces, and works with [flag.TextVar].
type Duration struct {
	// Years is the number of calendar years.
	Years int
	// Months is the number of calendar months.
	Months int
	// Weeks is the number of weeks, each seven calendar days.
	Weeks int
	// Days is the number of calendar days.
	Days int
	// Time is the exact part: hours, minutes and seconds.
	Time time.Duration
}

// Unit lengths in seconds, for converting fractions.
const (
	secondsPerMinute = 60
	secondsPerHour   = 60 * secondsPerMinute
	secondsPerDay    = 24 * secondsPerHour
	secondsPerWeek   = 7 * secondsPerDay
)

// Designator positions, in the order ISO 8601 requires.
const (
	unitYear = iota
	unitMonth
	unitWeek
	unitDay
	unitHour
	unitMinute
	unitSecond
)

var unitNames = [...]string{"years", "months", "weeks", "days", "hours", "minutes", "seconds"}
var unitDesignators = [...]byte{'Y', 'M', 'W', 'D', 'H', 'M', 'S'}

// ParseDuration parses an ISO 8601 duration in the format with designators
// (ISO 8601-1:2019 §5.5.2):
//
//	P[n]Y[n]M[n]W[n]D[T[n]H[n]M[n]S]
//
// for example P1Y2M3DT4H5M6.5S, P2W, PT36H or -P1D. The rules are:
//
//   - Components appear in the order above, each at most once. A zero
//     component may be omitted (P1YT1S), as ISO 8601 allows; the duration
//     ABNF in RFC 3339 Appendix A is stricter and would reject this.
//   - At least one component is required, and a "T" must be followed by at
//     least one of H, M or S. "P" and "PT" alone are errors.
//   - ISO 8601-1:2019 gives the week form PnW as an alternative to the
//     others, not something to combine with them. Combining weeks with other
//     components (P1W2D, P1Y2WT3H) is an ISO 8601-2:2019 extension, and this
//     function accepts it, so a [Duration] with both Weeks and Days set can
//     be parsed back from its String.
//   - The last (smallest) component present may have a decimal fraction,
//     with "." or "," as the decimal sign (ISO 8601-1:2019 §3.1.3.9). A
//     fraction of a week or day is converted at seven days per week and 24
//     hours per day: P1.5D is {Days: 1, Time: 12 * time.Hour}. A fraction of
//     a year or month is an error, because those have no fixed length.
//     Fractions are truncated to whole nanoseconds.
//   - A leading "-" negates the whole duration (ISO 8601-2:2019 §4.4.1.9).
//     Signs on individual components are not accepted.
//   - Designators are uppercase. The alternative format (§5.5.2.4, such as
//     P0001-02-03T04:05:06) is not supported.
//
// Each date component must fit in an int, and the hours, minutes and
// seconds together must fit in a [time.Duration] (about 292 years).
//
// The error, if any, is a [*ParseError] that names the input and the byte
// offset of the problem.
func ParseDuration(s string) (Duration, error) {
	p := &scanner{s: s, kind: "duration"}
	neg := false
	if p.peek() == '-' {
		neg = true
		p.pos++
	}
	if p.peek() != 'P' {
		return Duration{}, p.errorf(p.pos, "expected 'P', found %s", p.found(p.pos))
	}
	p.pos++
	if p.pos == len(s) {
		return Duration{}, p.errorf(p.pos, "a duration needs at least one component")
	}

	// Magnitudes are accumulated unsigned so that the most negative int and
	// time.Duration, whose magnitudes exceed the largest positive values,
	// can be parsed back from String.
	limit := uint64(math.MaxInt)
	timeLimit := uint64(math.MaxInt64)
	if neg {
		limit++
		timeLimit++
	}

	var date [4]uint64 // years, months, weeks, days
	var exact uint64   // nanoseconds
	inTime := false
	last := -1
	fracPos := -1

	for p.pos < len(s) {
		if p.s[p.pos] == 'T' {
			if inTime {
				return Duration{}, p.errorf(p.pos, "duplicate time designator 'T'")
			}
			if fracPos >= 0 {
				return Duration{}, p.errorf(fracPos, "only the smallest component present may have a decimal fraction")
			}
			inTime = true
			p.pos++
			if p.pos == len(s) || !isDigit(p.s[p.pos]) {
				return Duration{}, p.errorf(p.pos, "expected a number of hours, minutes or seconds after 'T', found %s", p.found(p.pos))
			}
			continue
		}

		numPos := p.pos
		intDigits := p.digitRun()
		if intDigits == 0 {
			return Duration{}, p.errorf(p.pos, "expected a number, found %s", p.found(p.pos))
		}
		if fracPos >= 0 {
			return Duration{}, p.errorf(fracPos, "only the smallest component present may have a decimal fraction")
		}
		intPart := s[p.pos : p.pos+intDigits]
		p.pos += intDigits
		var frac string
		if c := p.peek(); c == '.' || c == ',' {
			fracPos = p.pos
			p.pos++
			n := p.digitRun()
			if n == 0 {
				return Duration{}, p.errorf(p.pos, "expected digit after decimal sign, found %s", p.found(p.pos))
			}
			frac = s[p.pos : p.pos+n]
			p.pos += n
		}

		unitPos := p.pos
		if unitPos == len(s) {
			return Duration{}, p.errorf(unitPos, "expected a designator (Y, M, W, D, H or S) after %s", s[numPos:unitPos])
		}
		u := unitIndex(s[unitPos], inTime)
		if u < 0 {
			return Duration{}, p.unitError(unitPos, inTime)
		}
		if u == last {
			return Duration{}, p.errorf(unitPos, "duplicate %s designator '%c'", unitNames[u], unitDesignators[u])
		}
		if u < last {
			return Duration{}, p.errorf(unitPos, "%s designator '%c' must come before %s designator '%c'",
				unitNames[u], unitDesignators[u], unitNames[last], unitDesignators[last])
		}
		last = u
		p.pos++

		switch u {
		case unitYear, unitMonth, unitWeek, unitDay:
			if frac != "" && (u == unitYear || u == unitMonth) {
				return Duration{}, p.errorf(fracPos, "a fraction of %s is not supported, because their length varies", unitNames[u])
			}
			v, ok := parseUint(intPart, limit)
			if !ok {
				return Duration{}, p.errorf(numPos, "%s value %s is out of range", unitNames[u], intPart)
			}
			date[u] = v
			switch {
			case frac != "" && u == unitDay:
				// The fraction of a day becomes exact time.
				exact = fracNanos(frac, secondsPerDay)
			case frac != "" && u == unitWeek:
				// The fraction of a week becomes whole days and exact
				// time. Nothing can follow a fraction, so Days is unset.
				ns := fracNanos(frac, secondsPerWeek)
				date[unitDay] = ns / (secondsPerDay * 1e9)
				exact = ns % (secondsPerDay * 1e9)
			}
		default:
			unit := uint64(secondsPerHour)
			switch u {
			case unitMinute:
				unit = secondsPerMinute
			case unitSecond:
				unit = 1
			}
			ns, ok := parseUint(intPart, timeLimit/(unit*1e9))
			ns *= unit * 1e9
			if ok && frac != "" {
				ns, ok = addLimit(ns, fracNanos(frac, unit), timeLimit)
			}
			if ok {
				exact, ok = addLimit(exact, ns, timeLimit)
			}
			if !ok {
				return Duration{}, p.errorf(numPos, "hours, minutes and seconds exceed the range of time.Duration")
			}
		}
	}
	d := Duration{
		Years:  int(date[unitYear]),
		Months: int(date[unitMonth]),
		Weeks:  int(date[unitWeek]),
		Days:   int(date[unitDay]),
		Time:   time.Duration(exact),
	}
	if neg {
		// Negating converts each magnitude back to a signed value; the
		// conversion of limit+1 wraps to the minimum, which is its negation.
		d = Duration{
			Years:  -int(date[unitYear]),
			Months: -int(date[unitMonth]),
			Weeks:  -int(date[unitWeek]),
			Days:   -int(date[unitDay]),
			Time:   -time.Duration(exact),
		}
	}
	return d, nil
}

// unitIndex returns the position of designator c in the date part or the
// time part, or -1 if c is not valid there.
func unitIndex(c byte, inTime bool) int {
	if inTime {
		switch c {
		case 'H':
			return unitHour
		case 'M':
			return unitMinute
		case 'S':
			return unitSecond
		}
		return -1
	}
	switch c {
	case 'Y':
		return unitYear
	case 'M':
		return unitMonth
	case 'W':
		return unitWeek
	case 'D':
		return unitDay
	}
	return -1
}

func (p *scanner) unitError(pos int, inTime bool) *ParseError {
	switch c := p.s[pos]; {
	case !inTime && (c == 'H' || c == 'S'):
		return p.errorf(pos, "designator '%c' must follow the time designator 'T'", c)
	case inTime && (c == 'Y' || c == 'W' || c == 'D'):
		return p.errorf(pos, "designator '%c' must come before the time designator 'T'", c)
	case inTime:
		return p.errorf(pos, "expected designator H, M or S, found %s", p.found(pos))
	default:
		return p.errorf(pos, "expected designator Y, M, W or D, found %s", p.found(pos))
	}
}

// parseUint parses ASCII digits, reporting false if the value exceeds limit.
func parseUint(digits string, limit uint64) (uint64, bool) {
	var v uint64
	for i := range len(digits) {
		d := uint64(digits[i] - '0')
		if v > (limit-d)/10 {
			return 0, false
		}
		v = v*10 + d
	}
	return v, true
}

func addLimit(a, b, limit uint64) (uint64, bool) {
	if b > limit || a > limit-b {
		return 0, false
	}
	return a + b, true
}

var pow10 = [...]uint64{1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10,
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19}

// fracNanos returns floor(0.frac × unitSeconds) in nanoseconds, computed
// exactly from up to 19 fraction digits. Later digits add less than a
// thousandth of a nanosecond even for a week, and are ignored.
func fracNanos(frac string, unitSeconds uint64) uint64 {
	frac = frac[:min(len(frac), 19)]
	var n uint64
	for i := range len(frac) {
		n = n*10 + uint64(frac[i]-'0')
	}
	// n < 10^k and unit < 2^50, so hi < 10^k and Div64 cannot overflow.
	hi, lo := bits.Mul64(n, unitSeconds*1e9)
	q, _ := bits.Div64(hi, lo, pow10[len(frac)])
	return q
}

// String returns d in the ISO 8601 format with designators, with zero
// components omitted, for example P1Y2M3DT4H5M6.5S or -P2W. The zero
// duration is PT0S.
//
// The output is canonical: Time is split into hours, minutes and seconds,
// with hours never carried into days, because a day is a calendar unit
// (90 minutes is PT1H30M; 36 hours is PT36H). Seconds have a fraction,
// written with "." and without trailing zeros, only when it is non-zero.
// Weeks are written alongside other components when both are set, using
// the ISO 8601-2:2019 extension described at [ParseDuration].
//
// For every d whose fields share a sign, ParseDuration(d.String()) returns
// d. A d with mixed signs is written with a minus sign on each negative
// field, which ParseDuration does not accept; see [Duration].
func (d Duration) String() string {
	return string(d.appendText(make([]byte, 0, 32)))
}

// AppendText implements [encoding.TextAppender]. It appends the result of
// [Duration.String] to b, or returns an error if d's fields have mixed
// signs.
func (d Duration) AppendText(b []byte) ([]byte, error) {
	if pos, neg := d.signs(); pos && neg {
		return b, errMixedSigns
	}
	return d.appendText(b), nil
}

// MarshalText implements [encoding.TextMarshaler]. It returns the result of
// [Duration.String], or an error if d's fields have mixed signs, which no
// ISO 8601 duration can represent.
func (d Duration) MarshalText() ([]byte, error) {
	return d.AppendText(nil)
}

// UnmarshalText implements [encoding.TextUnmarshaler] by calling
// [ParseDuration]. On error, d is left unchanged.
func (d *Duration) UnmarshalText(text []byte) error {
	v, err := ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = v
	return nil
}

var errMixedSigns = errors.New("iso8601: duration fields have mixed signs, which ISO 8601 cannot represent")

// signs reports whether any field is positive and whether any is negative.
func (d Duration) signs() (pos, neg bool) {
	for _, v := range [...]int64{int64(d.Years), int64(d.Months), int64(d.Weeks), int64(d.Days), int64(d.Time)} {
		pos = pos || v > 0
		neg = neg || v < 0
	}
	return pos, neg
}

func (d Duration) appendText(b []byte) []byte {
	pos, neg := d.signs()
	if !pos && !neg {
		return append(b, "PT0S"...)
	}
	// With a uniform sign, write one leading minus and magnitudes. With
	// mixed signs, write each negative field with its own minus.
	perField := pos && neg
	if neg && !pos {
		b = append(b, '-')
	}
	b = append(b, 'P')
	appendField := func(v int64, unit byte) {
		if v == 0 {
			return
		}
		if perField && v < 0 {
			b = append(b, '-')
		}
		b = strconv.AppendUint(b, magnitude(v), 10)
		b = append(b, unit)
	}
	appendField(int64(d.Years), 'Y')
	appendField(int64(d.Months), 'M')
	appendField(int64(d.Weeks), 'W')
	appendField(int64(d.Days), 'D')
	if d.Time == 0 {
		return b
	}

	b = append(b, 'T')
	ns := magnitude(int64(d.Time))
	h := ns / uint64(time.Hour)
	ns -= h * uint64(time.Hour)
	m := ns / uint64(time.Minute)
	ns -= m * uint64(time.Minute)
	s := ns / uint64(time.Second)
	ns -= s * uint64(time.Second)
	minus := perField && d.Time < 0 // each time component gets its own sign
	appendTime := func(v uint64, unit byte, frac uint64) {
		if minus {
			b = append(b, '-')
		}
		b = strconv.AppendUint(b, v, 10)
		if frac != 0 {
			b = appendFraction(b, frac)
		}
		b = append(b, unit)
	}
	if h != 0 {
		appendTime(h, 'H', 0)
	}
	if m != 0 {
		appendTime(m, 'M', 0)
	}
	if s != 0 || ns != 0 {
		appendTime(s, 'S', ns)
	}
	return b
}

// appendFraction appends "." and the nine-digit nanosecond fraction ns,
// without trailing zeros.
func appendFraction(b []byte, ns uint64) []byte {
	var buf [10]byte
	buf[0] = '.'
	for i := 9; i >= 1; i-- {
		buf[i] = byte('0' + ns%10)
		ns /= 10
	}
	n := 10
	for buf[n-1] == '0' {
		n--
	}
	return append(b, buf[:n]...)
}

// magnitude returns |v|, which for the minimum int64 does not fit in int64.
func magnitude(v int64) uint64 {
	if v < 0 {
		return -uint64(v)
	}
	return uint64(v)
}

// Exact returns d as an exact [time.Duration] and true, or 0 and false if
// d has years or months, whose length depends on the calendar, or if the
// total overflows a time.Duration.
//
// Weeks and days count as 7×24 and 24 hours. That ignores daylight saving
// time and other changes in a location's UTC offset, across which a
// calendar day is 23 or 25 hours long; use [Duration.AddTo] to step through
// the calendar instead.
func (d Duration) Exact() (time.Duration, bool) {
	if d.Years != 0 || d.Months != 0 {
		return 0, false
	}
	weeks, ok1 := scale(int64(d.Weeks), int64(7*24*time.Hour))
	days, ok2 := scale(int64(d.Days), int64(24*time.Hour))
	sum, ok3 := add64(weeks, days)
	total, ok4 := add64(sum, int64(d.Time))
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return 0, false
	}
	return time.Duration(total), true
}

// scale returns n×unit for a positive unit, reporting false on overflow.
func scale(n, unit int64) (int64, bool) {
	if n > math.MaxInt64/unit || n < math.MinInt64/unit {
		return 0, false
	}
	return n * unit, true
}

// add64 returns a+b, reporting false on overflow.
func add64(a, b int64) (int64, bool) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, false
	}
	return a + b, true
}

// AddTo returns t plus d. The calendar part is applied first, in t's
// location, with a single call t.AddDate(d.Years, d.Months, 7*d.Weeks+d.Days),
// and the exact part is then added with t.Add(d.Time). So P1D keeps the
// wall clock time across a daylight saving change (23 or 25 elapsed hours),
// while PT24H is always 24 elapsed hours.
//
// Like [time.Time.AddDate], AddTo normalizes a day past the end of a month
// into the next month instead of clamping it: 2026-01-31 plus P1M is
// 2026-03-03 (February 31 is March 3), and 2024-02-29 plus P1Y is
// 2025-03-01. A wall clock time that the calendar step lands in a daylight
// saving gap or overlap resolves as [time.Date] resolves it.
//
// Subtracting is adding the negated duration, but because months and days
// vary in length, t plus d minus d need not equal t.
//
// Fields too large for time.Time's range give unspecified results, as with
// time.AddDate.
func (d Duration) AddTo(t time.Time) time.Time {
	return t.AddDate(d.Years, d.Months, 7*d.Weeks+d.Days).Add(d.Time)
}

// FromStd returns d as a Duration with only the exact Time part set. It
// does not carry hours into days, because a day is a calendar unit.
func FromStd(d time.Duration) Duration {
	return Duration{Time: d}
}

// FormatDuration formats a [time.Duration] as an ISO 8601 duration, for
// example PT1M30S for 90 seconds, PT36H for 36 hours and PT0S for zero. It
// is FromStd(d).String().
func FormatDuration(d time.Duration) string {
	return FromStd(d).String()
}
