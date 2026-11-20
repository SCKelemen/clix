package resource

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	posInf = math.Inf(1)
	negInf = math.Inf(-1)
	nan    = math.NaN()
)

// Str is Python's str() of a value: a string as it is, and the repr of
// anything else.
func Str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return Repr(v)
}

// Repr is Python's repr() of a value: None, True, False, an int's digits,
// a float as Python prints it (FloatRepr), a string in quotes with
// Python's escapes, and lists and dicts in brackets and braces with ", "
// and ": " between their parts.
func Repr(v any) string {
	var b strings.Builder
	writeRepr(&b, v)
	return b.String()
}

func writeRepr(b *strings.Builder, v any) {
	switch v := v.(type) {
	case nil:
		b.WriteString("None")
	case bool:
		if v {
			b.WriteString("True")
		} else {
			b.WriteString("False")
		}
	case *big.Int:
		b.WriteString(v.String())
	case float64:
		b.WriteString(FloatRepr(v))
	case string:
		writeStringRepr(b, v)
	case []any:
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			writeRepr(b, e)
		}
		b.WriteByte(']')
	case *Object:
		b.WriteByte('{')
		for i, k := range v.keys {
			if i > 0 {
				b.WriteString(", ")
			}
			writeStringRepr(b, k)
			b.WriteString(": ")
			writeRepr(b, v.vals[k])
		}
		b.WriteByte('}')
	default:
		fmt.Fprintf(b, "%v", v)
	}
}

// writeStringRepr writes Python's repr of a str: in single quotes, or in
// double quotes when s contains a single quote and no double quote, with
// backslash escapes for the quote, the backslash, \t, \n, \r and
// characters that are not printable (str.isprintable).
func writeStringRepr(b *strings.Builder, s string) {
	quote := '\''
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	b.WriteRune(quote)
	for _, r := range s {
		switch {
		case r == quote || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < ' ' || r == 0x7f:
			fmt.Fprintf(b, `\x%02x`, r)
		case r < 0x7f:
			b.WriteRune(r)
		case isPrintable(r):
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(b, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(b, `\u%04x`, r)
		default:
			fmt.Fprintf(b, `\U%08x`, r)
		}
	}
	b.WriteRune(quote)
}

// isPrintable is Python's str.isprintable for one character: false for
// the categories Cc, Cf, Cs, Co, Cn, Zl, Zp and Zs except the space,
// which is the set unicode.IsPrint accepts.
func isPrintable(r rune) bool {
	return unicode.IsPrint(r)
}

// FloatRepr is Python's repr() of a float: the shortest digits that read
// back as f, in fixed notation with at least one digit after the point
// when the decimal exponent is from -4 to 15 ("100.0", "0.0001"), and in
// scientific notation otherwise ("1e+16", "1.5e-05"); "inf", "-inf" and
// "nan" for the special values.
func FloatRepr(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	sci := strconv.FormatFloat(f, 'e', -1, 64)
	exp, err := strconv.Atoi(sci[strings.IndexByte(sci, 'e')+1:])
	if err == nil && (exp < -4 || exp >= 16) {
		return sci // Go's shortest 'e' format is Python's: "1e+16", "1.5e-05"
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.ContainsRune(s, '.') {
		s += ".0"
	}
	return s
}

// isSpace is Python's str.isspace for one character: Unicode White_Space
// plus the information separators U+001C to U+001F.
func isSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// trimSpace strips whitespace from both ends of s as Python's int() and
// float() do: Unicode White_Space, which unlike str.isspace leaves out
// U+001C to U+001F.
func trimSpace(s string) string {
	return strings.TrimFunc(s, unicode.IsSpace)
}

// Lower is Python's str.lower(): Go's lower case, except that U+0130
// (capital I with dot above) becomes "i" followed by U+0307, and a
// capital sigma at the end of a word becomes the final sigma, as in
// Unicode's SpecialCasing.txt.
func Lower(s string) string {
	if !strings.ContainsAny(s, "\u0130\u03a3") {
		return strings.ToLower(s)
	}
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		switch r {
		case 0x130:
			b.WriteString("i\u0307")
		case 0x3a3:
			if finalSigma(runes, i) {
				b.WriteRune(0x3c2)
			} else {
				b.WriteRune(0x3c3)
			}
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// finalSigma is the Final_Sigma condition of SpecialCasing.txt for the
// sigma at i: a cased letter before it and none after it, skipping
// case-ignorable characters either way.
func finalSigma(runes []rune, i int) bool {
	j := i - 1
	for j >= 0 && caseIgnorable(runes[j]) {
		j--
	}
	if j < 0 || !isCased(runes[j]) {
		return false
	}
	k := i + 1
	for k < len(runes) && caseIgnorable(runes[k]) {
		k++
	}
	return k == len(runes) || !isCased(runes[k])
}

// caseIgnorable is Unicode's Case_Ignorable property: the categories Mn,
// Me, Cf, Lm and Sk, and the word-break MidLetter, MidNumLet and
// Single_Quote characters.
func caseIgnorable(r rune) bool {
	if unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk) {
		return true
	}
	switch r {
	case '\'', '.', ':', 0xb7, 0x387, 0x55f, 0x5f4, 0x2018, 0x2019, 0x2024, 0x2027, 0xfe13, 0xfe52, 0xfe55, 0xff07, 0xff0e, 0xff1a:
		return true
	}
	return false
}

// UnsupportedError reports something gcloud would do that clix does not
// reproduce, so that the result is never different from gcloud's.
type UnsupportedError struct {
	What string // what is not supported, such as "the ~ operator"
	In   string // where, such as "--filter"; may be empty
}

func (e *UnsupportedError) Error() string {
	msg := e.What + " is not supported in clix"
	if e.In != "" {
		msg += " " + e.In
	}
	return msg
}

// Unsupportedf returns an *UnsupportedError.
func Unsupportedf(format string, args ...any) error {
	return &UnsupportedError{What: fmt.Sprintf(format, args...)}
}

// hasNonASCIIDigit reports whether s holds a decimal digit outside ASCII,
// which Python's int() and float() read and this port does not.
func hasNonASCIIDigit(s string) bool {
	for _, r := range s {
		if r >= utf8.RuneSelf && unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// ParseInt is Python's int(s) for a string: optional surrounding
// whitespace, an optional sign, and decimal digits with single
// underscores between them. ok is false where int() raises ValueError.
// A digit outside ASCII, which int() reads, is an *UnsupportedError.
func ParseInt(s string) (n *big.Int, ok bool, err error) {
	t := trimSpace(s)
	if hasNonASCIIDigit(t) {
		return nil, false, Unsupportedf("reading the number %q, which has digits outside ASCII,", s)
	}
	sign := ""
	if t != "" && (t[0] == '+' || t[0] == '-') {
		sign, t = t[:1], t[1:]
	}
	digits, ok := underscoreDigits(t)
	if !ok || digits == "" {
		return nil, false, nil
	}
	n, ok = new(big.Int).SetString(sign+digits, 10)
	return n, ok, nil
}

// underscoreDigits removes the underscores from s when s is ASCII digits
// with single underscores between them ("1_000"), as Python's int() and
// float() accept, and reports whether it was.
func underscoreDigits(s string) (string, bool) {
	if s == "" {
		return "", true
	}
	if s[0] == '_' || s[len(s)-1] == '_' || strings.Contains(s, "__") {
		return "", false
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case isASCIIDigit(c):
			b.WriteByte(c)
		case c == '_':
		default:
			return "", false
		}
	}
	return b.String(), true
}

// ParseFloat is Python's float(s) for a string: optional surrounding
// whitespace, then inf, infinity or nan in any case with an optional
// sign, or a decimal number with an optional fraction and exponent, whose
// digit groups may have single underscores between digits. ok is false
// where float() raises ValueError. A digit outside ASCII is an
// *UnsupportedError.
func ParseFloat(s string) (f float64, ok bool, err error) {
	t := trimSpace(s)
	if hasNonASCIIDigit(t) {
		return 0, false, Unsupportedf("reading the number %q, which has digits outside ASCII,", s)
	}
	sign := ""
	body := t
	if body != "" && (body[0] == '+' || body[0] == '-') {
		sign, body = body[:1], body[1:]
	}
	switch strings.ToLower(body) {
	case "inf", "infinity":
		if sign == "-" {
			return negInf, true, nil
		}
		return posInf, true, nil
	case "nan":
		return nan, true, nil
	}
	mant, exp, hasExp := body, "", false
	if i := strings.IndexAny(body, "eE"); i >= 0 {
		mant, exp, hasExp = body[:i], body[i+1:], true
	}
	intPart, frac, hasPoint := mant, "", false
	if i := strings.IndexByte(mant, '.'); i >= 0 {
		intPart, frac, hasPoint = mant[:i], mant[i+1:], true
	}
	ip, ok1 := underscoreDigits(intPart)
	fp, ok2 := underscoreDigits(frac)
	if !ok1 || !ok2 || (ip == "" && fp == "") {
		return 0, false, nil
	}
	text := sign + ip
	if hasPoint {
		text += "." + fp
	}
	if hasExp {
		esign := ""
		if exp != "" && (exp[0] == '+' || exp[0] == '-') {
			esign, exp = exp[:1], exp[1:]
		}
		ep, ok := underscoreDigits(exp)
		if !ok || ep == "" {
			return 0, false, nil
		}
		text += "e" + esign + ep
	}
	if text == sign+"" || strings.HasPrefix(text, sign+"e") {
		return 0, false, nil
	}
	f, err = strconv.ParseFloat(text, 64)
	if err != nil && !isRangeError(err) {
		return 0, false, nil
	}
	return f, true, nil
}

// Number is a Python number: an int (also a bool, which Python treats as
// the int 0 or 1) or a float.
type Number struct {
	Int   *big.Int // nil for a float
	Float float64
}

// NumberOf returns v as a Python number when it is a bool, an int or a
// float.
func NumberOf(v any) (Number, bool) {
	switch v := v.(type) {
	case bool:
		if v {
			return Number{Int: big.NewInt(1)}, true
		}
		return Number{Int: big.NewInt(0)}, true
	case *big.Int:
		return Number{Int: v}, true
	case float64:
		return Number{Float: v}, true
	}
	return Number{}, false
}

// NumericType is gcloud's resource_expr._NumericType: a number as it is,
// and a string as int(s), or else float(s). ok is false where Python
// raises TypeError or ValueError (None, lists, dicts, other strings).
func NumericType(v any) (Number, bool, error) {
	if n, ok := NumberOf(v); ok {
		return n, true, nil
	}
	s, ok := v.(string)
	if !ok {
		return Number{}, false, nil
	}
	if i, ok, err := ParseInt(s); err != nil || ok {
		return Number{Int: i}, ok, err
	}
	f, ok, err := ParseFloat(s)
	return Number{Float: f}, ok, err
}

// Compare compares two Python numbers exactly, as Python does (an int
// and a float are compared by value, not by converting the int). ok is
// false when either is NaN, which is unordered.
func (a Number) Compare(b Number) (cmp int, ok bool) {
	switch {
	case a.Int != nil && b.Int != nil:
		return a.Int.Cmp(b.Int), true
	case a.Int == nil && b.Int == nil:
		if math.IsNaN(a.Float) || math.IsNaN(b.Float) {
			return 0, false
		}
		switch {
		case a.Float < b.Float:
			return -1, true
		case a.Float > b.Float:
			return 1, true
		}
		return 0, true
	case a.Int == nil:
		c, ok := b.Compare(a)
		return -c, ok
	}
	// a is an int, b a float.
	switch {
	case math.IsNaN(b.Float):
		return 0, false
	case math.IsInf(b.Float, 1):
		return -1, true
	case math.IsInf(b.Float, -1):
		return 1, true
	}
	x := new(big.Float).SetInt(a.Int)
	y := new(big.Float).SetFloat64(b.Float)
	return x.Cmp(y), true
}

// Repr is Python's repr() of the number. A bool reads as its int here;
// use the original value for "True" and "False".
func (a Number) Repr() string {
	if a.Int != nil {
		return a.Int.String()
	}
	return FloatRepr(a.Float)
}

// Equal is Python's == for two values, as gcloud uses it to match keys:
// numbers (and bools) by value, strings exactly, None only with None, and
// lists and dicts element by element.
func Equal(a, b any) bool {
	if na, ok := NumberOf(a); ok {
		nb, ok := NumberOf(b)
		if !ok {
			return false
		}
		c, ok := na.Compare(nb)
		return ok && c == 0
	}
	switch a := a.(type) {
	case nil:
		return b == nil
	case string:
		s, ok := b.(string)
		return ok && a == s
	case []any:
		l, ok := b.([]any)
		if !ok || len(a) != len(l) {
			return false
		}
		for i := range a {
			if !Equal(a[i], l[i]) {
				return false
			}
		}
		return true
	case *Object:
		o, ok := b.(*Object)
		if !ok || a.Len() != o.Len() {
			return false
		}
		for _, k := range a.keys {
			v, ok := o.vals[k]
			if !ok || !Equal(a.vals[k], v) {
				return false
			}
		}
		return true
	}
	return false
}

// Truthy is Python's bool() of a value: false for None, False, zero,
// empty strings, lists and dicts.
func Truthy(v any) bool {
	switch v := v.(type) {
	case nil:
		return false
	case bool:
		return v
	case *big.Int:
		return v.Sign() != 0
	case float64:
		return v != 0 // NaN is true
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case *Object:
		return v.Len() > 0
	}
	return true
}

// isCased reports whether r has case, as Python's str methods decide it.
func isCased(r rune) bool {
	return unicode.In(r, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Other_Lowercase, unicode.Other_Uppercase)
}

// title is Python's str.title(): each character after a cased character
// in lower case, every other one in title case.
func title(s string) string {
	var b strings.Builder
	prevCased := false
	for _, r := range s {
		if prevCased {
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(unicode.ToTitle(r))
		}
		prevCased = isCased(r)
	}
	return b.String()
}

// CamelCase is resource_property.ConvertToCamelCase: snake_case to
// camelCase ("project_id" to "projectId").
func CamelCase(name string) string {
	parts := strings.Split(name, "_")
	var b strings.Builder
	b.WriteString(parts[0])
	for _, p := range parts[1:] {
		b.WriteString(title(p))
	}
	return b.String()
}

// SnakeCase is resource_property.ConvertToSnakeCase: camelCase to
// snake_case ("projectId" to "project_id", "URLPath" to "url_path"). It
// implements the substitution
//
//	((?<=[a-z0-9])[A-Z]+(?=[A-Z][a-z]|$)|(?!^)[A-Z](?=[a-z])) -> _\1
//
// followed by str.lower(), which Go's regexp cannot express (it has no
// lookaround).
func SnakeCase(name string) string {
	s := name
	isUpper := func(i int) bool { return i < len(s) && s[i] >= 'A' && s[i] <= 'Z' }
	isLower := func(i int) bool { return i < len(s) && s[i] >= 'a' && s[i] <= 'z' }
	// atEnd is Python's $ without MULTILINE: the end, or before a final
	// newline.
	atEnd := func(i int) bool { return i == len(s) || (i == len(s)-1 && s[i] == '\n') }
	var b strings.Builder
	for i := 0; i < len(s); {
		if i > 0 && isUpper(i) && (isLower(i-1) || isASCIIDigit(s[i-1])) {
			// First alternative: a run of capitals ending before a
			// capitalized word or at the end; the longest that does.
			j := i
			for isUpper(j) {
				j++
			}
			matched := false
			for k := j; k > i; k-- {
				if (isUpper(k) && isLower(k+1)) || atEnd(k) {
					b.WriteByte('_')
					b.WriteString(s[i:k])
					i = k
					matched = true
					break
				}
			}
			if matched {
				continue
			}
		}
		if i > 0 && isUpper(i) && isLower(i+1) {
			b.WriteByte('_')
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return Lower(b.String())
}
