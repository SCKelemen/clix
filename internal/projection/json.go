package projection

import (
	"fmt"
	"math"
	"math/big"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

// JSON returns v as compact JSON with object keys sorted, as Python's
// json.dumps(v, sort_keys=True, ensure_ascii=False,
// separators=(",", ":")) writes it, for clix's JSON and YAML encoders to
// format. A float that JSON cannot hold (infinite or NaN) is an error.
func JSON(v any) ([]byte, error) {
	var err error
	b := appendJSON(nil, v, ",", ":", &err)
	return b, err
}

// appendCanonical is json.dumps(v, sort_keys=True, ensure_ascii=False),
// with Python's default separators, for tests.
func appendCanonical(b []byte, v any) []byte {
	var err error
	return appendJSON(b, v, ", ", ": ", &err)
}

func appendJSON(b []byte, v any, comma, colon string, err *error) []byte {
	switch v := v.(type) {
	case nil:
		return append(b, "null"...)
	case bool:
		return strconv.AppendBool(b, v)
	case *big.Int:
		return append(b, v.String()...)
	case float64:
		switch {
		case math.IsInf(v, 1):
			*err = fmt.Errorf("the value inf cannot be written as JSON")
			return append(b, "Infinity"...)
		case math.IsInf(v, -1):
			*err = fmt.Errorf("the value -inf cannot be written as JSON")
			return append(b, "-Infinity"...)
		case math.IsNaN(v):
			*err = fmt.Errorf("the value nan cannot be written as JSON")
			return append(b, "NaN"...)
		}
		return append(b, resource.FloatRepr(v)...)
	case string:
		return appendJSONString(b, v)
	case []any:
		b = append(b, '[')
		for i, e := range v {
			if i > 0 {
				b = append(b, comma...)
			}
			b = appendJSON(b, e, comma, colon, err)
		}
		return append(b, ']')
	case *resource.Object:
		keys := slices.Clone(v.Keys())
		slices.Sort(keys)
		b = append(b, '{')
		for i, k := range keys {
			if i > 0 {
				b = append(b, comma...)
			}
			b = appendJSONString(b, k)
			b = append(b, colon...)
			e, _ := v.Get(k)
			b = appendJSON(b, e, comma, colon, err)
		}
		return append(b, '}')
	}
	*err = fmt.Errorf("projection: cannot write %T as JSON", v)
	return b
}

// appendJSONString writes s as json.dumps does with ensure_ascii=False:
// only the quote, the backslash and control characters below U+0020 are
// escaped.
func appendJSONString(b []byte, s string) []byte {
	b = append(b, '"')
	for _, r := range s {
		switch r {
		case '"':
			b = append(b, `\"`...)
		case '\\':
			b = append(b, `\\`...)
		case '\n':
			b = append(b, `\n`...)
		case '\r':
			b = append(b, `\r`...)
		case '\t':
			b = append(b, `\t`...)
		case '\b':
			b = append(b, `\b`...)
		case '\f':
			b = append(b, `\f`...)
		default:
			if r < 0x20 {
				b = fmt.Appendf(b, `\u%04x`, r)
			} else {
				b = utf8.AppendRune(b, r)
			}
		}
	}
	return append(b, '"')
}
