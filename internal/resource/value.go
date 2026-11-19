package resource

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// A value is one of:
//
//   - nil (JSON null, Python None)
//   - bool
//   - string
//   - *big.Int (a JSON number without a fraction or exponent, a Python int)
//   - float64 (any other JSON number, a Python float)
//   - []any (a JSON array, a Python list)
//   - *Object (a JSON object, a Python dict)
//
// Values are never modified once decoded, so they may be shared.

// Object is a JSON object as Python's json module reads it into a dict:
// keys in the order they first appear, and for a key that appears more
// than once, its last value.
type Object struct {
	keys []string
	vals map[string]any
}

// NewObject returns an empty object.
func NewObject() *Object {
	return &Object{vals: map[string]any{}}
}

// Set adds key with value v, or replaces its value, keeping its position.
func (o *Object) Set(key string, v any) {
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = v
}

// Get returns the value of key.
func (o *Object) Get(key string) (any, bool) {
	v, ok := o.vals[key]
	return v, ok
}

// Keys returns the keys in order. The caller must not modify the slice.
func (o *Object) Keys() []string { return o.keys }

// Len returns the number of keys.
func (o *Object) Len() int { return len(o.keys) }

// maxDepth bounds the nesting of decoded values.
const maxDepth = 10000

// Decode parses one JSON value as Python's json.loads does (with the
// default strict=True): a number without a fraction or exponent is an
// int (*big.Int), any other number a float64 (one too large is ±Inf, as
// Python's float() makes it), and a key that appears twice keeps its
// first position and its last value. Unpaired surrogate escapes, which a
// Python string can hold, become U+FFFD.
func Decode(data []byte) (any, error) {
	return decode(data, false)
}

// decodePython is Decode with the extensions json.loads also accepts:
// NaN, Infinity and -Infinity.
func decodePython(data []byte) (any, error) {
	return decode(data, true)
}

func decode(data []byte, extensions bool) (any, error) {
	d := &decoder{s: data, ext: extensions}
	d.space()
	v, err := d.value(0)
	if err != nil {
		return nil, err
	}
	d.space()
	if d.i < len(d.s) {
		return nil, d.errorf("extra data")
	}
	return v, nil
}

type decoder struct {
	s   []byte
	i   int
	ext bool
}

func (d *decoder) errorf(format string, args ...any) error {
	return fmt.Errorf("invalid JSON at offset %d: %s", d.i, fmt.Sprintf(format, args...))
}

// space skips JSON whitespace, the only whitespace json.loads accepts.
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

func (d *decoder) literal(text string) bool {
	if strings.HasPrefix(string(d.s[d.i:min(len(d.s), d.i+len(text))]), text) {
		d.i += len(text)
		return true
	}
	return false
}

func (d *decoder) value(depth int) (any, error) {
	if depth > maxDepth {
		return nil, d.errorf("nesting deeper than %d", maxDepth)
	}
	if d.i >= len(d.s) {
		return nil, d.errorf("expecting value")
	}
	switch c := d.s[d.i]; {
	case c == '{':
		d.i++
		obj := NewObject()
		d.space()
		if d.i < len(d.s) && d.s[d.i] == '}' {
			d.i++
			return obj, nil
		}
		for {
			d.space()
			if d.i >= len(d.s) || d.s[d.i] != '"' {
				return nil, d.errorf("expecting property name enclosed in double quotes")
			}
			key, err := d.str()
			if err != nil {
				return nil, err
			}
			d.space()
			if d.i >= len(d.s) || d.s[d.i] != ':' {
				return nil, d.errorf("expecting ':' delimiter")
			}
			d.i++
			d.space()
			v, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			obj.Set(key, v)
			d.space()
			if d.i < len(d.s) && d.s[d.i] == ',' {
				d.i++
				continue
			}
			if d.i < len(d.s) && d.s[d.i] == '}' {
				d.i++
				return obj, nil
			}
			return nil, d.errorf("expecting ',' delimiter")
		}
	case c == '[':
		d.i++
		list := []any{}
		d.space()
		if d.i < len(d.s) && d.s[d.i] == ']' {
			d.i++
			return list, nil
		}
		for {
			d.space()
			v, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
			d.space()
			if d.i < len(d.s) && d.s[d.i] == ',' {
				d.i++
				continue
			}
			if d.i < len(d.s) && d.s[d.i] == ']' {
				d.i++
				return list, nil
			}
			return nil, d.errorf("expecting ',' delimiter")
		}
	case c == '"':
		return d.str()
	case c == '-' || isASCIIDigit(c):
		if d.ext && d.literal("-Infinity") {
			return negInf, nil
		}
		return d.number()
	}
	switch {
	case d.literal("null"):
		return nil, nil
	case d.literal("true"):
		return true, nil
	case d.literal("false"):
		return false, nil
	case d.ext && d.literal("NaN"):
		return nan, nil
	case d.ext && d.literal("Infinity"):
		return posInf, nil
	}
	return nil, d.errorf("expecting value")
}

// number reads a JSON number (RFC 8259 section 6).
func (d *decoder) number() (any, error) {
	start := d.i
	if d.s[d.i] == '-' {
		d.i++
	}
	switch {
	case d.i < len(d.s) && d.s[d.i] == '0':
		d.i++
	case d.i < len(d.s) && d.s[d.i] >= '1' && d.s[d.i] <= '9':
		for d.i < len(d.s) && isASCIIDigit(d.s[d.i]) {
			d.i++
		}
	default:
		return nil, d.errorf("expecting value")
	}
	isFloat := false
	if d.i+1 < len(d.s) && d.s[d.i] == '.' && isASCIIDigit(d.s[d.i+1]) {
		isFloat = true
		d.i++
		for d.i < len(d.s) && isASCIIDigit(d.s[d.i]) {
			d.i++
		}
	}
	if d.i < len(d.s) && (d.s[d.i] == 'e' || d.s[d.i] == 'E') {
		j := d.i + 1
		if j < len(d.s) && (d.s[j] == '+' || d.s[j] == '-') {
			j++
		}
		if j < len(d.s) && isASCIIDigit(d.s[j]) {
			isFloat = true
			d.i = j
			for d.i < len(d.s) && isASCIIDigit(d.s[d.i]) {
				d.i++
			}
		}
	}
	text := string(d.s[start:d.i])
	if !isFloat {
		n, ok := new(big.Int).SetString(text, 10)
		if !ok {
			return nil, d.errorf("invalid number %q", text)
		}
		return n, nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil && !isRangeError(err) {
		return nil, d.errorf("invalid number %q", text)
	}
	return f, nil
}

// str reads a JSON string. Control characters must be escaped, as
// json.loads requires by default.
func (d *decoder) str() (string, error) {
	d.i++ // the opening quote
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
		case c < 0x20:
			return "", d.errorf("invalid control character")
		case c == '\\':
			if d.i+1 >= len(d.s) {
				return "", d.errorf("unterminated string")
			}
			e := d.s[d.i+1]
			d.i += 2
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
					return "", d.errorf("invalid \\uXXXX escape")
				}
				if utf16.IsSurrogate(r) {
					if r < 0xdc00 && d.i+1 < len(d.s) && d.s[d.i] == '\\' && d.s[d.i+1] == 'u' {
						save := d.i
						d.i += 2
						if r2, ok := d.hex4(); ok && r2 >= 0xdc00 && r2 <= 0xdfff {
							r = utf16.DecodeRune(r, r2)
						} else {
							d.i = save
							r = utf8.RuneError
						}
					} else {
						r = utf8.RuneError
					}
				}
				b.WriteRune(r)
			default:
				return "", d.errorf("invalid \\escape")
			}
		default:
			r, size := utf8.DecodeRune(d.s[d.i:])
			b.WriteRune(r) // invalid UTF-8 becomes U+FFFD
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

func isASCIIDigit(c byte) bool { return '0' <= c && c <= '9' }

func isRangeError(err error) bool {
	ne, ok := err.(*strconv.NumError)
	return ok && ne.Err == strconv.ErrRange
}
