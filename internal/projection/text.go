package projection

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

// isSpace is Python's str.isspace for one character: Unicode White_Space
// plus U+001C to U+001F, which is also what \s, strip() and lstrip()
// match.
func isSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// floatText is gcloud's resource_transform.TransformFloat with its
// defaults: Python's "{:.6}" format (six significant digits, fixed
// notation with at least one digit after the point for exponents from -4
// to 4, scientific otherwise), except that a positive exponent below 9 is
// written in fixed notation with one decimal ("1234567.0").
func floatText(f float64) string {
	s := pyFormatPrecision(f, 6)
	if i := strings.Index(s, "e+"); i >= 0 {
		if exp, err := strconv.Atoi(s[i+2:]); err == nil && exp < 9 {
			return strconv.FormatFloat(f, 'f', 1, 64)
		}
	}
	return s
}

// pyFormatPrecision is Python's format(f, ".<p>") with no presentation
// type: like "g" with p significant digits, but a fixed-point result
// keeps at least one digit after the point, and scientific notation is
// used when the exponent is below -4 or at least p-1.
func pyFormatPrecision(f float64, p int) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	sci := strconv.FormatFloat(f, 'e', p-1, 64)
	i := strings.IndexByte(sci, 'e')
	exp, _ := strconv.Atoi(sci[i+1:])
	if exp < -4 || exp >= p-1 {
		mant := sci[:i]
		if strings.Contains(mant, ".") {
			mant = strings.TrimRight(strings.TrimRight(mant, "0"), ".")
		}
		return mant + sci[i:]
	}
	s := strconv.FormatFloat(f, 'f', p-1-exp, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		if strings.HasSuffix(s, ".") {
			s += "0"
		}
	} else {
		s += ".0"
	}
	return s
}

// stringify is a table cell's text (table_printer._Stringify): nothing
// for None, a float as floatText, and Python's str() of anything else.
func stringify(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case float64:
		return floatText(v)
	}
	return resource.Str(v)
}

// charWidth is gcloud's console_attr.GetCharacterDisplayWidth: 0 for
// combining characters and format controls (Cf), 2 for East Asian wide
// and fullwidth characters, and 1 for everything else.
func charWidth(r rune) int {
	s := norm.NFC.String(string(r))
	c := []rune(s)[0]
	switch {
	case norm.NFC.PropertiesString(string(c)).CCC() != 0:
		return 0
	case unicode.Is(unicode.Cf, c):
		return 0
	}
	switch width.LookupRune(c).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	}
	return 1
}

// displayWidth is console_attr.DisplayWidth (without ANSI sequences,
// which clix rejects): the width of the widest line.
func displayWidth(s string) int {
	w, max := 0, 0
	for _, r := range s {
		if r == '\n' {
			if w > max {
				max = w
			}
			w = 0
			continue
		}
		w += charWidth(r)
	}
	if w > max {
		max = w
	}
	return max
}

// runeLen is Python's len() of a str.
func runeLen(s string) int { return len([]rune(s)) }

// ljust, rjust and center are table_printer._Justify's: Python's
// str.ljust, rjust and center with the width adjusted by the difference
// between display width and length.
func ljust(s string, w int) string {
	return s + strings.Repeat(" ", max(0, w-displayWidth(s)))
}

func rjust(s string, w int) string {
	return strings.Repeat(" ", max(0, w-displayWidth(s))) + s
}

func center(s string, w int) string {
	return pyCenter(s, w-(displayWidth(s)-runeLen(s)))
}

// pyCenter is Python's str.center(width) by length: the extra space of an
// odd margin goes left when width is odd.
func pyCenter(s string, w int) string {
	n := runeLen(s)
	marg := w - n
	if marg <= 0 {
		return s
	}
	left := marg/2 + (marg & w & 1)
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", marg-left)
}

// rstripSpace and stripSpace are Python's rstrip() and strip().
func rstripSpace(s string) string { return strings.TrimRightFunc(s, isSpace) }
func stripSpace(s string) string  { return strings.TrimFunc(s, isSpace) }

// checkText rejects text that holds control characters other than tab
// and newline: gcloud writes them to the terminal as they are, so a value
// could move the cursor or change colors, and clix does not print them.
func checkText(s string) error {
	for _, r := range s {
		if (r < 0x20 && r != '\t' && r != '\n') || (r >= 0x7f && r < 0xa0) {
			return resource.Unsupportedf("printing the control character %U in %q", r, s)
		}
	}
	return nil
}

var whitespaceRun = regexp.MustCompile(`[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]+`)

// nextLine is table_printer._GetNextLineAndRemainder (without wrapping
// attributes or color): the part of s that fits in maxWidth columns,
// ended at the first newline or, failing that, after the last whitespace
// that fits, and what remains.
func nextLine(s string, maxWidth int) (line, rest string) {
	runes := []rune(s)
	cur, split := 0, 0
	for split < len(runes) {
		cur += charWidth(runes[split])
		if cur > maxWidth {
			break
		}
		split++
	}
	tail := string(runes[split:])
	split += runeLen(tail) - runeLen(strings.TrimLeftFunc(tail, isSpace))
	if nl := strings.IndexByte(s, '\n'); nl >= 0 && runeLen(s[:nl])+1 <= split {
		split = runeLen(s[:nl]) + 1
	} else {
		maxWS := -1
		for _, m := range whitespaceRun.FindAllStringIndex(s, -1) {
			end := runeLen(s[:m[1]])
			if end > split {
				break
			}
			maxWS = end
		}
		if maxWS > 0 {
			split = maxWS
		}
	}
	return rstripSpace(string(runes[:split])), string(runes[split:])
}

// pyInt returns n as a Python int value.
func pyInt(n int) *big.Int { return big.NewInt(int64(n)) }

// keyName is gcloud's resource_lex.GetKeyName with quoting: names joined
// with dots, a name with characters other than letters, digits, _, - and
// @ in double quotes, and indices appended in brackets.
func keyName(key resource.Key) string {
	var parts []string
	for _, p := range key {
		switch {
		case p.Slice:
			if len(parts) > 0 {
				parts[len(parts)-1] += "[]"
				continue
			}
			parts = append(parts, "[]")
		case p.Index != nil:
			s := "[" + p.Index.String() + "]"
			if len(parts) > 0 {
				parts[len(parts)-1] += s
				continue
			}
			parts = append(parts, s)
		default:
			name := p.Name
			word := func(r rune) bool {
				return r == '-' || r == '@' || r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
			}
			if strings.IndexFunc(name, func(r rune) bool { return !word(r) }) >= 0 {
				name = `"` + strings.ReplaceAll(strings.ReplaceAll(name, `\`, `\\`), `"`, `\"`) + `"`
			}
			parts = append(parts, name)
		}
	}
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, ".")
}
