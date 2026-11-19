package jq

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// maxDepth bounds the nesting of input values, as jq's parser does.
const maxDepth = 10000

// decode parses one JSON value, keeping object keys in order and number
// literals as written. A key that appears twice keeps its first position
// and its last value, as in jq. Invalid UTF-8 and unpaired surrogate
// escapes become U+FFFD.
func decode(data []byte) (any, error) {
	d := &decoder{s: data}
	d.space()
	if d.i >= len(d.s) {
		return nil, fmt.Errorf("no JSON value in the input")
	}
	v, err := d.value(0)
	if err != nil {
		return nil, err
	}
	d.space()
	if d.i < len(d.s) {
		return nil, d.errorf("unexpected data after the JSON value")
	}
	return v, nil
}

type decoder struct {
	s []byte
	i int
}

func (d *decoder) errorf(format string, args ...any) error {
	return fmt.Errorf("invalid JSON input at offset %d: %s", d.i, fmt.Sprintf(format, args...))
}

func (d *decoder) space() {
	for d.i < len(d.s) {
		switch d.s[d.i] {
		case ' ', '\t', '\n', '\r':
			d.i++
		default:
			return
		}
	}
}

func (d *decoder) value(depth int) (any, error) {
	if depth > maxDepth {
		return nil, d.errorf("nesting deeper than %d", maxDepth)
	}
	if d.i >= len(d.s) {
		return nil, d.errorf("unexpected end of input")
	}
	switch c := d.s[d.i]; {
	case c == '{':
		d.i++
		obj := newObject(0)
		d.space()
		if d.i < len(d.s) && d.s[d.i] == '}' {
			d.i++
			return obj, nil
		}
		for {
			d.space()
			if d.i >= len(d.s) || d.s[d.i] != '"' {
				return nil, d.errorf("expected an object key")
			}
			key, err := d.str()
			if err != nil {
				return nil, err
			}
			d.space()
			if d.i >= len(d.s) || d.s[d.i] != ':' {
				return nil, d.errorf("expected ':'")
			}
			d.i++
			d.space()
			v, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			obj.set(key, v)
			d.space()
			if d.i < len(d.s) && d.s[d.i] == ',' {
				d.i++
				continue
			}
			if d.i < len(d.s) && d.s[d.i] == '}' {
				d.i++
				return obj, nil
			}
			return nil, d.errorf("expected ',' or '}'")
		}
	case c == '[':
		d.i++
		arr := []any{}
		d.space()
		if d.i < len(d.s) && d.s[d.i] == ']' {
			d.i++
			return arr, nil
		}
		for {
			d.space()
			v, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
			d.space()
			if d.i < len(d.s) && d.s[d.i] == ',' {
				d.i++
				continue
			}
			if d.i < len(d.s) && d.s[d.i] == ']' {
				d.i++
				return arr, nil
			}
			return nil, d.errorf("expected ',' or ']'")
		}
	case c == '"':
		return d.str()
	case c == '-' || isDigit(c):
		start := d.i
		if c == '-' {
			d.i++
		}
		for d.i < len(d.s) && (isDigit(d.s[d.i]) || strings.IndexByte(".eE+-", d.s[d.i]) >= 0) {
			d.i++
		}
		text := string(d.s[start:d.i])
		if !validJSONNumber(text) {
			return nil, d.errorf("invalid number %q", text)
		}
		dec, ok := parseDecimal(text)
		if !ok {
			return nil, d.errorf("number %q is out of range", text)
		}
		return literalNumber(dec), nil
	default:
		for _, lit := range []struct {
			text string
			v    any
		}{{"true", true}, {"false", false}, {"null", nil}} {
			if strings.HasPrefix(string(d.s[d.i:]), lit.text) {
				d.i += len(lit.text)
				return lit.v, nil
			}
		}
	}
	return nil, d.errorf("unexpected character %q", d.s[d.i])
}

// validJSONNumber reports whether s is a number in the JSON grammar
// (RFC 8259 section 6).
func validJSONNumber(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	switch {
	case i < len(s) && s[i] == '0':
		i++
	case i < len(s) && s[i] >= '1' && s[i] <= '9':
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	default:
		return false
	}
	if i < len(s) && s[i] == '.' {
		i++
		if i >= len(s) || !isDigit(s[i]) {
			return false
		}
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if i >= len(s) || !isDigit(s[i]) {
			return false
		}
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	return i == len(s)
}

// str parses a JSON string starting at the opening quote.
func (d *decoder) str() (string, error) {
	d.i++ // opening quote
	var b strings.Builder
	for {
		if d.i >= len(d.s) {
			return "", d.errorf("unterminated string")
		}
		c := d.s[d.i]
		switch {
		case c == '"':
			d.i++
			return b.String(), nil
		case c == '\\':
			d.i++
			if d.i >= len(d.s) {
				return "", d.errorf("unterminated string")
			}
			e := d.s[d.i]
			d.i++
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
				r, ok := d.hex4()
				if !ok {
					return "", d.errorf("invalid \\u escape")
				}
				if utf16.IsSurrogate(r) {
					r2 := rune(-1)
					if d.i+1 < len(d.s) && d.s[d.i] == '\\' && d.s[d.i+1] == 'u' {
						save := d.i
						d.i += 2
						var ok2 bool
						r2, ok2 = d.hex4()
						if !ok2 {
							return "", d.errorf("invalid \\u escape")
						}
						if utf16.DecodeRune(r, r2) == utf8.RuneError {
							d.i = save
							r2 = -1
						}
					}
					if r2 < 0 {
						r = utf8.RuneError
					} else {
						r = utf16.DecodeRune(r, r2)
					}
				}
				b.WriteRune(r)
			default:
				return "", d.errorf("invalid escape \\%c", e)
			}
		case c < 0x20:
			return "", d.errorf("control character in string")
		case c < utf8.RuneSelf:
			b.WriteByte(c)
			d.i++
		default:
			r, size := utf8.DecodeRune(d.s[d.i:])
			if r == utf8.RuneError && size == 1 {
				b.WriteString("�")
			} else {
				b.Write(d.s[d.i : d.i+size])
			}
			d.i += size
		}
	}
}

func (d *decoder) hex4() (rune, bool) {
	if d.i+4 > len(d.s) {
		return 0, false
	}
	n, err := strconv.ParseUint(string(d.s[d.i:d.i+4]), 16, 32)
	if err != nil {
		return 0, false
	}
	d.i += 4
	return rune(n), true
}
