package iso8601

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

// ParseError reports an input that [ParseTime] or [ParseDuration] rejected.
// It records the whole input and the byte offset at which the problem was
// found, so a caller can point at the offending character.
type ParseError struct {
	// Input is the complete string that was being parsed.
	Input string
	// Offset is the zero-based byte offset in Input where the problem was
	// found. It equals len(Input) when the input ended too early.
	Offset int
	// Msg describes the problem, for example "month 13 is out of range 01-12".
	Msg string

	kind string // "time" or "duration"
}

// Error returns a message of the form
//
//	iso8601: invalid time "2026-13-01" at offset 5: month 13 is out of range 01-12
//
// The input is quoted with [strconv.Quote], so control characters in it
// cannot reach a terminal unescaped.
func (e *ParseError) Error() string {
	return "iso8601: invalid " + e.kind + " " + strconv.Quote(e.Input) +
		" at offset " + strconv.Itoa(e.Offset) + ": " + e.Msg
}

// scanner is a cursor over an input string. Every accessor works on bytes;
// only error messages decode runes, to describe what was found.
type scanner struct {
	s    string
	pos  int
	kind string
}

func (p *scanner) errorf(pos int, format string, args ...any) *ParseError {
	return &ParseError{Input: p.s, Offset: pos, Msg: fmt.Sprintf(format, args...), kind: p.kind}
}

// peek returns the byte at the cursor, or 0 at the end of the input.
func (p *scanner) peek() byte {
	if p.pos < len(p.s) {
		return p.s[p.pos]
	}
	return 0
}

// found describes the character at pos for an error message.
func (p *scanner) found(pos int) string {
	if pos >= len(p.s) {
		return "end of input"
	}
	r, _ := utf8.DecodeRuneInString(p.s[pos:])
	return strconv.QuoteRune(r)
}

// digitRun returns the number of consecutive ASCII digits at the cursor,
// without consuming them.
func (p *scanner) digitRun() int {
	n := 0
	for p.pos+n < len(p.s) && isDigit(p.s[p.pos+n]) {
		n++
	}
	return n
}

// fixed consumes exactly n ASCII digits and returns their value.
func (p *scanner) fixed(n int, what string) (int, error) {
	v := 0
	for i := range n {
		at := p.pos + i
		if at >= len(p.s) || !isDigit(p.s[at]) {
			return 0, p.errorf(at, "expected %d-digit %s, found %s", n, what, p.found(at))
		}
		v = v*10 + int(p.s[at]-'0')
	}
	p.pos += n
	return v, nil
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }
