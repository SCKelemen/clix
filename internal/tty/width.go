package tty

import (
	"unicode"
	"unicode/utf8"
)

// RuneWidth returns the number of terminal columns r occupies: 0 for
// control characters, combining marks and other zero-width characters, 2
// for East Asian Wide and Fullwidth characters and emoji, 1 otherwise.
//
// Widths follow Unicode Standard Annex #11 (East Asian Width),
// https://www.unicode.org/reports/tr11/, the way wcwidth(3) and terminal
// emulators apply it; ambiguous-width characters count as 1.
func RuneWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 0x20 || (r >= 0x7f && r < 0xa0):
		return 0
	case r < 0x300:
		return 1
	case isZeroWidth(r):
		return 0
	case isWide(r):
		return 2
	default:
		return 1
	}
}

func isZeroWidth(r rune) bool {
	if unicode.In(r, unicode.Mn, unicode.Me) {
		return true
	}
	switch {
	case r == 0x00ad: // soft hyphen is shown by most terminals; keep width 1
		return false
	case r >= 0x1160 && r <= 0x11ff: // Hangul Jamo medial vowels and final consonants
		return true
	case r == 0x200b || r == 0x200c || r == 0x200d || r == 0x2060 || r == 0xfeff:
		return true
	case r >= 0xfe00 && r <= 0xfe0f: // variation selectors
		return true
	case r >= 0xe0000 && r <= 0xe0fff: // tags and variation selectors supplement
		return true
	}
	return unicode.Is(unicode.Cf, r)
}

// wideRanges lists the East Asian Wide (W) and Fullwidth (F) ranges and the
// emoji that default to emoji presentation, from EastAsianWidth.txt.
var wideRanges = [][2]rune{
	{0x1100, 0x115f}, {0x231a, 0x231b}, {0x2329, 0x232a}, {0x23e9, 0x23ec},
	{0x23f0, 0x23f0}, {0x23f3, 0x23f3}, {0x25fd, 0x25fe}, {0x2614, 0x2615},
	{0x2648, 0x2653}, {0x267f, 0x267f}, {0x2693, 0x2693}, {0x26a1, 0x26a1},
	{0x26aa, 0x26ab}, {0x26bd, 0x26be}, {0x26c4, 0x26c5}, {0x26ce, 0x26ce},
	{0x26d4, 0x26d4}, {0x26ea, 0x26ea}, {0x26f2, 0x26f3}, {0x26f5, 0x26f5},
	{0x26fa, 0x26fa}, {0x26fd, 0x26fd}, {0x2705, 0x2705}, {0x270a, 0x270b},
	{0x2728, 0x2728}, {0x274c, 0x274c}, {0x274e, 0x274e}, {0x2753, 0x2755},
	{0x2757, 0x2757}, {0x2795, 0x2797}, {0x27b0, 0x27b0}, {0x27bf, 0x27bf},
	{0x2b1b, 0x2b1c}, {0x2b50, 0x2b50}, {0x2b55, 0x2b55},
	{0x2e80, 0x303e}, {0x3041, 0x33ff}, {0x3400, 0x4dbf}, {0x4e00, 0x9fff},
	{0xa000, 0xa4cf}, {0xa960, 0xa97f}, {0xac00, 0xd7a3}, {0xf900, 0xfaff},
	{0xfe10, 0xfe19}, {0xfe30, 0xfe6f}, {0xff00, 0xff60}, {0xffe0, 0xffe6},
	{0x16fe0, 0x16fe4}, {0x16ff0, 0x16ff1}, {0x17000, 0x18cff}, {0x18d00, 0x18d08},
	{0x1aff0, 0x1b2ff}, {0x1f004, 0x1f004}, {0x1f0cf, 0x1f0cf}, {0x1f18e, 0x1f18e},
	{0x1f191, 0x1f19a}, {0x1f200, 0x1f202}, {0x1f210, 0x1f23b}, {0x1f240, 0x1f248},
	{0x1f250, 0x1f251}, {0x1f260, 0x1f265}, {0x1f300, 0x1f320}, {0x1f32d, 0x1f335},
	{0x1f337, 0x1f37c}, {0x1f37e, 0x1f393}, {0x1f3a0, 0x1f3ca}, {0x1f3cf, 0x1f3d3},
	{0x1f3e0, 0x1f3f0}, {0x1f3f4, 0x1f3f4}, {0x1f3f8, 0x1f43e}, {0x1f440, 0x1f440},
	{0x1f442, 0x1f4fc}, {0x1f4ff, 0x1f53d}, {0x1f54b, 0x1f54e}, {0x1f550, 0x1f567},
	{0x1f57a, 0x1f57a}, {0x1f595, 0x1f596}, {0x1f5a4, 0x1f5a4}, {0x1f5fb, 0x1f64f},
	{0x1f680, 0x1f6c5}, {0x1f6cc, 0x1f6cc}, {0x1f6d0, 0x1f6d2}, {0x1f6d5, 0x1f6d7},
	{0x1f6dc, 0x1f6df}, {0x1f6eb, 0x1f6ec}, {0x1f6f4, 0x1f6fc}, {0x1f7e0, 0x1f7eb},
	{0x1f7f0, 0x1f7f0}, {0x1f90c, 0x1f93a}, {0x1f93c, 0x1f945}, {0x1f947, 0x1f9ff},
	{0x1fa70, 0x1faff}, {0x20000, 0x2fffd}, {0x30000, 0x3fffd},
}

func isWide(r rune) bool {
	lo, hi := 0, len(wideRanges)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		switch {
		case r < wideRanges[mid][0]:
			hi = mid - 1
		case r > wideRanges[mid][1]:
			lo = mid + 1
		default:
			return true
		}
	}
	return false
}

// StringWidth returns the number of terminal columns s occupies. It sums
// the widths of its grapheme clusters, so an emoji with a variation
// selector or a ZWJ sequence counts once.
func StringWidth(s string) int {
	w := 0
	for len(s) > 0 {
		cluster, cw := FirstGrapheme(s)
		w += cw
		s = s[len(cluster):]
	}
	return w
}

// FirstGrapheme returns the first user-perceived character of s (an
// approximation of an extended grapheme cluster from Unicode Standard Annex
// #29, https://www.unicode.org/reports/tr29/) and its display width.
//
// A cluster is a base character followed by combining marks, variation
// selectors, emoji modifiers, tag characters and zero-width-joiner
// sequences; two regional indicators (a flag) also form one cluster.
func FirstGrapheme(s string) (cluster string, width int) {
	if s == "" {
		return "", 0
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && size <= 1 {
		return s[:1], 1
	}
	if r == '\r' && len(s) > 1 && s[1] == '\n' {
		return s[:2], 0
	}
	i := size
	width = RuneWidth(r)
	emoji := false
	if isRegionalIndicator(r) {
		if r2, n := utf8.DecodeRuneInString(s[i:]); isRegionalIndicator(r2) {
			return s[:i+n], 2
		}
		return s[:i], 1
	}
	for i < len(s) {
		next, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case next == 0x200d: // ZWJ joins the following character
			i += n
			if i < len(s) {
				_, n2 := utf8.DecodeRuneInString(s[i:])
				i += n2
				emoji = true
			}
		case next == 0xfe0f: // emoji presentation selector
			i += n
			emoji = true
		case next >= 0x1f3fb && next <= 0x1f3ff: // skin tone modifiers
			i += n
			emoji = true
		case isZeroWidth(next) || unicode.Is(unicode.Mc, next):
			i += n
		default:
			if emoji && width < 2 {
				width = 2
			}
			return s[:i], width
		}
	}
	if emoji && width < 2 {
		width = 2
	}
	return s[:i], width
}

// LastGrapheme returns the last user-perceived character of s and its
// display width (see FirstGrapheme).
func LastGrapheme(s string) (cluster string, width int) {
	start := 0
	for start < len(s) {
		c, w := FirstGrapheme(s[start:])
		if start+len(c) >= len(s) {
			return c, w
		}
		start += len(c)
	}
	return "", 0
}

func isRegionalIndicator(r rune) bool {
	return r >= 0x1f1e6 && r <= 0x1f1ff
}

// Truncate shortens s (plain text, no escape sequences) to at most width
// columns, ending in tail when it had to cut. Whole grapheme clusters are
// kept or dropped together.
func Truncate(s string, width int, tail string) string {
	if StringWidth(s) <= width {
		return s
	}
	limit := width - StringWidth(tail)
	if limit < 0 {
		limit = 0
		tail = ""
	}
	used, end := 0, 0
	for end < len(s) {
		c, w := FirstGrapheme(s[end:])
		if used+w > limit {
			break
		}
		used += w
		end += len(c)
	}
	return s[:end] + tail
}
