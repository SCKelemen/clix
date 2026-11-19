package jq

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// A value is one of:
//
//   - nil (null)
//   - bool
//   - string
//   - number
//   - []any (an array; never modified once built)
//   - *object
//
// Values are immutable, so they may be shared freely.

// number is a jq number. A literal (written in the input or in the filter)
// keeps its decimal text, as jq 1.7 does when built with decNumber: it
// prints as written (in canonical form) and compares exactly with other
// literals. A computed number (the result of length, keys or negation)
// has only its float64 value.
type number struct {
	f   float64
	lit *decimal
}

// object is a jq object. Keys keep their insertion order, as jq's do.
type object struct {
	keys []string
	vals map[string]any
}

func newObject(n int) *object {
	return &object{keys: make([]string, 0, n), vals: make(map[string]any, n)}
}

// set adds or replaces key; a replaced key keeps its position.
func (o *object) set(key string, v any) {
	if _, ok := o.vals[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = v
}

// with returns a copy of o with key set to v.
func (o *object) with(key string, v any) *object {
	c := newObject(len(o.keys) + 1)
	c.keys = append(c.keys, o.keys...)
	for k, x := range o.vals {
		c.vals[k] = x
	}
	c.set(key, v)
	return c
}

// sortedKeys returns the keys in jq's order (bytewise).
func (o *object) sortedKeys() []string {
	keys := append([]string(nil), o.keys...)
	sort.Strings(keys)
	return keys
}

func computed(f float64) number { return number{f: f} }

func literalNumber(d *decimal) number { return number{f: d.float(), lit: d} }

// decimal is an arbitrary-precision decimal literal: (-1)^neg * coef *
// 10^exp, the representation decNumber uses.
type decimal struct {
	neg  bool
	coef string // digits without leading zeros; "0" for zero
	exp  int
}

// maxExponent bounds literal exponents, as decNumber's emax does.
const maxExponent = 999999999

// parseDecimal parses a number literal: an optional minus sign, digits
// with an optional fraction (either part may be empty, not both), and an
// optional exponent. It accepts JSON numbers and jq's literals such as
// "1." and ".5".
func parseDecimal(s string) (*decimal, bool) {
	d := &decimal{}
	i := 0
	if i < len(s) && s[i] == '-' {
		d.neg = true
		i++
	}
	start := i
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	intPart := s[start:i]
	frac := ""
	if i < len(s) && s[i] == '.' {
		i++
		fs := i
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		frac = s[fs:i]
	}
	if intPart == "" && frac == "" {
		return nil, false
	}
	exp := 0
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		neg := false
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			neg = s[i] == '-'
			i++
		}
		es := i
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		digits := strings.TrimLeft(s[es:i], "0")
		if es == i {
			return nil, false
		}
		if len(digits) > 9 {
			return nil, false
		}
		e, _ := strconv.Atoi("0" + digits)
		if neg {
			e = -e
		}
		exp = e
	}
	if i != len(s) {
		return nil, false
	}
	coef := strings.TrimLeft(intPart+frac, "0")
	if coef == "" {
		coef = "0"
	}
	d.coef = coef
	d.exp = exp - len(frac)
	if adj := d.exp + len(d.coef) - 1; adj > maxExponent || adj < -maxExponent {
		return nil, false
	}
	return d, true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// String formats d the way decNumber's to-scientific-string does, which is
// how jq 1.7 prints a literal: "1.0", "1E+2", "0.00001", "1.5E-7".
func (d *decimal) String() string {
	c := d.coef
	adj := d.exp + len(c) - 1
	var b strings.Builder
	if d.neg {
		b.WriteByte('-')
	}
	if d.exp <= 0 && adj >= -6 {
		switch p := len(c) + d.exp; {
		case d.exp == 0:
			b.WriteString(c)
		case p > 0:
			b.WriteString(c[:p])
			b.WriteByte('.')
			b.WriteString(c[p:])
		default:
			b.WriteString("0.")
			b.WriteString(strings.Repeat("0", -p))
			b.WriteString(c)
		}
		return b.String()
	}
	b.WriteByte(c[0])
	if len(c) > 1 {
		b.WriteByte('.')
		b.WriteString(c[1:])
	}
	b.WriteByte('E')
	if adj >= 0 {
		b.WriteByte('+')
	}
	b.WriteString(strconv.Itoa(adj))
	return b.String()
}

// float converts d to a float64 the way jq 1.7 does: it rounds d to 17
// significant digits (half up), then parses that. Out-of-range values
// become ±Inf or 0.
func (d *decimal) float() float64 {
	c, e := d.coef, d.exp
	if len(c) > 17 {
		head := []byte(c[:17])
		e += len(c) - 17
		if c[17] >= '5' {
			i := len(head) - 1
			for ; i >= 0; i-- {
				if head[i] == '9' {
					head[i] = '0'
					continue
				}
				head[i]++
				break
			}
			if i < 0 {
				head = append([]byte{'1'}, head...)
			}
		}
		c = string(head)
	}
	f, _ := strconv.ParseFloat(c+"e"+strconv.Itoa(e), 64)
	if d.neg {
		f = -f
	}
	return f
}

// sign returns -1, 0 or 1.
func (d *decimal) sign() int {
	switch {
	case d.coef == "0":
		return 0
	case d.neg:
		return -1
	}
	return 1
}

// cmpDecimal compares two decimals exactly.
func cmpDecimal(a, b *decimal) int {
	sa, sb := a.sign(), b.sign()
	if sa != sb {
		return cmpInt(sa, sb)
	}
	if sa == 0 {
		return 0
	}
	ac := strings.TrimRight(a.coef, "0")
	bc := strings.TrimRight(b.coef, "0")
	// The adjusted exponent is the power of ten of the first digit; it is
	// the same with or without trailing zeros.
	aa := a.exp + len(a.coef) - 1
	ba := b.exp + len(b.coef) - 1
	m := cmpInt(aa, ba)
	if m == 0 {
		for i := 0; i < len(ac) || i < len(bc); i++ {
			var da, db byte = '0', '0'
			if i < len(ac) {
				da = ac[i]
			}
			if i < len(bc) {
				db = bc[i]
			}
			if da != db {
				m = cmpInt(int(da), int(db))
				break
			}
		}
	}
	if sa < 0 {
		return -m
	}
	return m
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// cmpNumbers compares as jq 1.7 does: exactly when both are literals, and
// as doubles otherwise.
func cmpNumbers(a, b number) int {
	if a.lit != nil && b.lit != nil {
		return cmpDecimal(a.lit, b.lit)
	}
	switch {
	case a.f < b.f:
		return -1
	case a.f > b.f:
		return 1
	}
	return 0
}

// kindOrder is jq's ordering of types.
func kindOrder(v any) int {
	switch x := v.(type) {
	case nil:
		return 0
	case bool:
		if x {
			return 2
		}
		return 1
	case number:
		return 3
	case string:
		return 4
	case []any:
		return 5
	case *object:
		return 6
	}
	panic("jq: unknown value type")
}

// compare orders two values the way jq does: null < false < true <
// numbers < strings < arrays < objects; strings bytewise, arrays
// element by element, objects by their sorted keys and then by the values
// of those keys.
func compare(a, b any) int {
	ka, kb := kindOrder(a), kindOrder(b)
	if ka != kb {
		return cmpInt(ka, kb)
	}
	switch x := a.(type) {
	case number:
		return cmpNumbers(x, b.(number))
	case string:
		return strings.Compare(x, b.(string))
	case []any:
		y := b.([]any)
		for i := 0; i < len(x) && i < len(y); i++ {
			if c := compare(x[i], y[i]); c != 0 {
				return c
			}
		}
		return cmpInt(len(x), len(y))
	case *object:
		y := b.(*object)
		xk, yk := x.sortedKeys(), y.sortedKeys()
		for i := 0; i < len(xk) && i < len(yk); i++ {
			if c := strings.Compare(xk[i], yk[i]); c != 0 {
				return c
			}
		}
		if c := cmpInt(len(xk), len(yk)); c != 0 {
			return c
		}
		for _, k := range xk {
			if c := compare(x.vals[k], y.vals[k]); c != 0 {
				return c
			}
		}
	}
	return 0
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	}
	return true
}

func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case number:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case *object:
		return "object"
	}
	panic("jq: unknown value type")
}

// describe formats a value for an error message as jq does: its type and
// its JSON text, cut to 11 bytes followed by "..." when it is longer than
// 14 bytes, as in `string ("abcdefghij...)`.
func describe(v any) string {
	s := compactString(v)
	if len(s) > 14 {
		cut := 11
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut] + "..."
	}
	return typeName(v) + " (" + s + ")"
}

// clampInf replaces an infinity with the largest finite float64 of the
// same sign, which is how jq prints infinities.
func clampInf(f float64) float64 {
	switch {
	case math.IsInf(f, 1):
		return math.MaxFloat64
	case math.IsInf(f, -1):
		return -math.MaxFloat64
	}
	return f
}
