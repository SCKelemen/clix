package jq

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// formatNumber formats n as jq 1.8 prints it: a literal in its canonical
// decimal form, a computed number like jq's jvp_dtoa_fmt.
func formatNumber(n number) string {
	if n.lit != nil {
		return n.lit.String()
	}
	return formatDouble(n.f)
}

// formatDouble formats f the way jq's jvp_dtoa_fmt does (David Gay's
// g_fmt with jq's changes): the shortest digits that read back as f, in
// exponential notation when the decimal point would be more than 15
// places past the digits or 4 or more places before them, with a signed
// exponent of at least two digits. Infinities print as the largest
// finite values and NaN as null, as in jq.
func formatDouble(f float64) string {
	if math.IsNaN(f) {
		return "null"
	}
	f = clampInf(f)
	if f == 0 {
		if math.Signbit(f) {
			return "-0"
		}
		return "0"
	}
	s := strconv.FormatFloat(math.Abs(f), 'e', -1, 64) // d.ddde±XX
	mant, exp, _ := strings.Cut(s, "e")
	digits := strings.Replace(mant, ".", "", 1)
	e, _ := strconv.Atoi(exp)
	decpt := e + 1
	nd := len(digits)
	var b strings.Builder
	if f < 0 {
		b.WriteByte('-')
	}
	switch {
	case decpt <= -4 || decpt > nd+15:
		b.WriteByte(digits[0])
		if nd > 1 {
			b.WriteByte('.')
			b.WriteString(digits[1:])
		}
		b.WriteByte('e')
		x := decpt - 1
		if x < 0 {
			b.WriteByte('-')
			x = -x
		} else {
			b.WriteByte('+')
		}
		if x < 10 {
			b.WriteByte('0')
		}
		b.WriteString(strconv.Itoa(x))
	case decpt <= 0:
		b.WriteString("0.")
		b.WriteString(strings.Repeat("0", -decpt))
		b.WriteString(digits)
	case nd <= decpt:
		b.WriteString(digits)
		b.WriteString(strings.Repeat("0", decpt-nd))
	default:
		b.WriteString(digits[:decpt])
		b.WriteByte('.')
		b.WriteString(digits[decpt:])
	}
	return b.String()
}

// appendString appends s as a JSON string with jq's escaping: '"' and '\'
// escaped, \b \t \n \f \r for those controls, \u00XX (lowercase hex) for
// the other C0 controls and DEL, and everything else as is.
func appendString(b []byte, s string) []byte {
	b = append(b, '"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' && c != 0x7f {
			if c < utf8.RuneSelf {
				i++
				continue
			}
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				// Invalid UTF-8 cannot come from the parsers; write U+FFFD
				// as jq would have replaced it on input.
				b = append(b, s[start:i]...)
				b = append(b, "�"...)
				i++
				start = i
				continue
			}
			i += size
			continue
		}
		b = append(b, s[start:i]...)
		switch c {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		case '\b':
			b = append(b, '\\', 'b')
		case '\t':
			b = append(b, '\\', 't')
		case '\n':
			b = append(b, '\\', 'n')
		case '\f':
			b = append(b, '\\', 'f')
		case '\r':
			b = append(b, '\\', 'r')
		default:
			const hex = "0123456789abcdef"
			b = append(b, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xf])
		}
		i++
		start = i
	}
	b = append(b, s[start:]...)
	return append(b, '"')
}

// appendJSON appends v as JSON. With pretty set it uses jq's default
// layout (two-space indent, ": " between keys and values); otherwise the
// compact form tojson produces.
func appendJSON(b []byte, v any, pretty bool, depth int) []byte {
	switch x := v.(type) {
	case nil:
		return append(b, "null"...)
	case bool:
		if x {
			return append(b, "true"...)
		}
		return append(b, "false"...)
	case number:
		return append(b, formatNumber(x)...)
	case string:
		return appendString(b, x)
	case []any:
		if len(x) == 0 {
			return append(b, "[]"...)
		}
		b = append(b, '[')
		for i, e := range x {
			if i > 0 {
				b = append(b, ',')
			}
			if pretty {
				b = appendIndent(b, depth+1)
			}
			b = appendJSON(b, e, pretty, depth+1)
		}
		if pretty {
			b = appendIndent(b, depth)
		}
		return append(b, ']')
	case *object:
		if len(x.keys) == 0 {
			return append(b, "{}"...)
		}
		b = append(b, '{')
		for i, k := range x.keys {
			if i > 0 {
				b = append(b, ',')
			}
			if pretty {
				b = appendIndent(b, depth+1)
			}
			b = appendString(b, k)
			b = append(b, ':')
			if pretty {
				b = append(b, ' ')
			}
			b = appendJSON(b, x.vals[k], pretty, depth+1)
		}
		if pretty {
			b = appendIndent(b, depth)
		}
		return append(b, '}')
	}
	panic("jq: unknown value type")
}

func appendIndent(b []byte, depth int) []byte {
	b = append(b, '\n')
	for i := 0; i < depth; i++ {
		b = append(b, ' ', ' ')
	}
	return b
}

// compactString returns v as compact JSON, the text tojson produces.
func compactString(v any) string {
	return string(appendJSON(nil, v, false, 0))
}

// toString is jq's tostring: a string as is, anything else as compact
// JSON.
func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return compactString(v)
}
