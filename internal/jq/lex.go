package jq

import (
	"fmt"
	"strings"
)

type tokenKind int

const (
	tEOF    tokenKind = iota
	tDot              // .
	tRec              // ..
	tField            // .name (text is the name)
	tIdent            // a name or a keyword
	tNumber           // a number literal (text)
	tString           // a string literal (parts)
	tFormat           // @name (text is the name)
	tVar              // $name (text is the name)
	tOp               // punctuation and operators (text)
)

type token struct {
	kind  tokenKind
	text  string
	parts []strPart // tString
	pos   int
}

// strPart is a piece of a string literal: literal text, or an
// interpolated filter (\(...)).
type strPart struct {
	text string
	expr node
}

// keywords are jq's reserved words.
var keywords = map[string]bool{
	"__loc__": true, "and": true, "as": true, "catch": true, "def": true,
	"elif": true, "else": true, "end": true, "foreach": true, "if": true,
	"import": true, "include": true, "label": true, "or": true,
	"reduce": true, "then": true, "try": true,
}

// operators lists the multi-character operators, longest first.
var operators = []string{
	"?//", "//=", "|=", "+=", "-=", "*=", "/=", "%=", "==", "!=", "<=", ">=", "//",
}

type lexer struct {
	src   string
	pos   int
	base  int // offset of src in the whole filter, for messages
	depth int // nesting of src in the whole filter
}

func (l *lexer) errorf(pos int, format string, args ...any) error {
	return &SyntaxError{Offset: l.base + pos, Msg: fmt.Sprintf(format, args...)}
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentChar(c byte) bool { return isIdentStart(c) || isDigit(c) }

// skip skips whitespace and comments (# to the end of the line).
func (l *lexer) skip() {
	for l.pos < len(l.src) {
		switch c := l.src[l.pos]; c {
		case ' ', '\t', '\n', '\r':
			l.pos++
		case '#':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		default:
			return
		}
	}
}

func (l *lexer) next() (token, error) {
	l.skip()
	start := l.pos
	if l.pos >= len(l.src) {
		return token{kind: tEOF, pos: start}, nil
	}
	s := l.src[l.pos:]
	c := s[0]
	switch {
	case c == '.':
		switch {
		case len(s) > 1 && s[1] == '.':
			l.pos += 2
			return token{kind: tRec, text: "..", pos: start}, nil
		case len(s) > 1 && isIdentStart(s[1]):
			n := 2
			for n < len(s) && isIdentChar(s[n]) {
				n++
			}
			l.pos += n
			return token{kind: tField, text: s[1:n], pos: start}, nil
		case len(s) > 1 && isDigit(s[1]):
			return l.number()
		}
		l.pos++
		return token{kind: tDot, text: ".", pos: start}, nil
	case isDigit(c):
		return l.number()
	case isIdentStart(c):
		n := 1
		for n < len(s) && isIdentChar(s[n]) {
			n++
		}
		// Module-qualified names (mod::name) are part of the name.
		for n+2 < len(s) && s[n] == ':' && s[n+1] == ':' && isIdentStart(s[n+2]) {
			n += 3
			for n < len(s) && isIdentChar(s[n]) {
				n++
			}
		}
		l.pos += n
		return token{kind: tIdent, text: s[:n], pos: start}, nil
	case c == '"':
		parts, err := l.str()
		if err != nil {
			return token{}, err
		}
		return token{kind: tString, parts: parts, pos: start}, nil
	case c == '@' || c == '$':
		n := 1
		for n < len(s) && isIdentChar(s[n]) {
			n++
		}
		if n == 1 || !isIdentStart(s[1]) {
			return token{}, l.errorf(start, "unexpected %q", string(c))
		}
		l.pos += n
		kind := tFormat
		if c == '$' {
			kind = tVar
		}
		return token{kind: kind, text: s[1:n], pos: start}, nil
	}
	for _, op := range operators {
		if strings.HasPrefix(s, op) {
			l.pos += len(op)
			return token{kind: tOp, text: op, pos: start}, nil
		}
	}
	if strings.IndexByte("|,:;?()[]{}<>=+-*/%", c) >= 0 {
		l.pos++
		return token{kind: tOp, text: string(c), pos: start}, nil
	}
	return token{}, l.errorf(start, "unexpected character %q", rune(c))
}

// number lexes a number literal: digits with an optional fraction, or a
// fraction alone, then an optional exponent, as jq's lexer does.
func (l *lexer) number() (token, error) {
	start := l.pos
	s := l.src
	i := l.pos
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && isDigit(s[j]) {
			for j < len(s) && isDigit(s[j]) {
				j++
			}
			i = j
		}
	}
	l.pos = i
	return token{kind: tNumber, text: s[start:i], pos: start}, nil
}

// str lexes a string literal, parsing each interpolation \(...) as a
// filter.
func (l *lexer) str() ([]strPart, error) {
	start := l.pos
	l.pos++ // opening quote
	var parts []strPart
	var b strings.Builder
	for {
		if l.pos >= len(l.src) {
			return nil, l.errorf(start, "unterminated string")
		}
		c := l.src[l.pos]
		switch c {
		case '"':
			l.pos++
			if b.Len() > 0 || len(parts) == 0 {
				parts = append(parts, strPart{text: b.String()})
			}
			return parts, nil
		case '\\':
			if l.pos+1 >= len(l.src) {
				return nil, l.errorf(start, "unterminated string")
			}
			e := l.src[l.pos+1]
			l.pos += 2
			switch e {
			case '"', '\\', '/':
				b.WriteByte(e)
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				r, err := l.unicodeEscape(l.pos - 2)
				if err != nil {
					return nil, err
				}
				b.WriteRune(r)
			case '(':
				end, err := l.matchParen(l.pos, l.depth)
				if err != nil {
					return nil, err
				}
				expr, err := parseSub(l.src[l.pos:end], l.base+l.pos, l.depth+1)
				if err != nil {
					return nil, err
				}
				if b.Len() > 0 {
					parts = append(parts, strPart{text: b.String()})
					b.Reset()
				}
				parts = append(parts, strPart{expr: expr})
				l.pos = end + 1
			default:
				return nil, l.errorf(l.pos-2, "invalid escape \\%c in a string", e)
			}
		default:
			b.WriteByte(c)
			l.pos++
		}
	}
}

// unicodeEscape reads the XXXX of \uXXXX (and a second \uXXXX for a
// surrogate pair) at l.pos. at is the offset of the backslash.
func (l *lexer) unicodeEscape(at int) (rune, error) {
	read := func() (rune, bool) {
		if l.pos+4 > len(l.src) {
			return 0, false
		}
		var r rune
		for _, c := range []byte(l.src[l.pos : l.pos+4]) {
			var d byte
			switch {
			case isDigit(c):
				d = c - '0'
			case c >= 'a' && c <= 'f':
				d = c - 'a' + 10
			case c >= 'A' && c <= 'F':
				d = c - 'A' + 10
			default:
				return 0, false
			}
			r = r<<4 | rune(d)
		}
		l.pos += 4
		return r, true
	}
	r, ok := read()
	if !ok {
		return 0, l.errorf(at, "invalid \\u escape in a string")
	}
	switch {
	case r >= 0xd800 && r < 0xdc00:
		if strings.HasPrefix(l.src[l.pos:], "\\u") {
			l.pos += 2
			lo, ok := read()
			if ok && lo >= 0xdc00 && lo < 0xe000 {
				return (r-0xd800)<<10 + (lo - 0xdc00) + 0x10000, nil
			}
		}
		return 0, l.errorf(at, "invalid \\uXXXX\\uXXXX surrogate pair escape")
	case r >= 0xdc00 && r < 0xe000:
		return 0, l.errorf(at, "invalid \\uXXXX\\uXXXX surrogate pair escape")
	}
	return r, nil
}

// matchParen returns the offset of the ")" that closes the "(" just
// before pos, skipping strings (and their interpolations) and comments.
// nesting counts the enclosing interpolations, to bound the recursion.
func (l *lexer) matchParen(pos, nesting int) (int, error) {
	if nesting > maxNesting {
		return 0, l.errorf(pos, "the filter nests more than %d levels deep", maxNesting)
	}
	depth := 1
	s := l.src
	for i := pos; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, nil
			}
		case '#':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case '"':
			end, err := l.skipString(i, nesting)
			if err != nil {
				return 0, err
			}
			i = end
		}
	}
	return 0, l.errorf(pos-2, "unterminated string interpolation")
}

// skipString returns the offset of the closing quote of the string that
// starts at pos.
func (l *lexer) skipString(pos, nesting int) (int, error) {
	s := l.src
	for i := pos + 1; i < len(s); i++ {
		switch s[i] {
		case '"':
			return i, nil
		case '\\':
			if i+1 < len(s) && s[i+1] == '(' {
				end, err := l.matchParen(i+2, nesting+1)
				if err != nil {
					return 0, err
				}
				i = end
				continue
			}
			i++
		}
	}
	return 0, l.errorf(pos, "unterminated string")
}
