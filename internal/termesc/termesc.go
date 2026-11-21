// Package termesc shows the characters a terminal acts on as visible
// escapes, so that text a program prints, such as strings from a server,
// cannot move the cursor, change colors, set the window title, write a
// hyperlink, overwrite a line or reorder what is shown.
//
// It escapes:
//
//   - C0 control characters (U+0000 to U+001F), among them ESC, which
//     starts every escape sequence (CSI "ESC [", OSC "ESC ]", and so on),
//     BEL, backspace and CR, which returns to the start of the line so
//     that what follows overwrites it;
//   - DEL (U+007F);
//   - C1 control characters (U+0080 to U+009F), among them the one-byte
//     CSI (U+009B), OSC (U+009D) and ST (U+009C);
//   - the bidirectional embedding, override and isolate controls U+202A to
//     U+202E and U+2066 to U+2069, which reorder the text around them
//     ("Trojan Source", CVE-2021-42574);
//   - each byte of invalid UTF-8, which a terminal in an 8-bit mode could
//     read as a C1 control.
//
// Tab, newline and CR are written \t, \n and \r, other C0 controls and
// DEL \xHH, the rest \uHHHH, and invalid bytes \xHH, all in lower case:
// ESC [ 3 1 m becomes the text \x1b[31m. A backslash in the text is not
// escaped, so plain ASCII and UTF-8 text is unchanged, byte for byte.
//
// This is kubectl's fix for CVE-2021-25743 (kubernetes/kubernetes#112553,
// staging/src/k8s.io/cli-runtime/pkg/printers/terminal.go), which escapes
// ESC as ^[ and CR as \r, extended to every control character.
// https://github.com/kubernetes/kubernetes/pull/112553
package termesc

import (
	"strings"
	"unicode/utf8"
)

// Keep is a set of layout characters Escape leaves as they are.
type Keep uint8

const (
	// Tab keeps tabs.
	Tab Keep = 1 << iota
	// Newline keeps newlines.
	Newline
	// CRLF keeps a CR right before a newline, which ends a line as the
	// newline does (a Windows line ending), when Newline is set too. It
	// is safe only where the text is written whole: if the newline did
	// not follow the CR in the output (a cell laid out in columns, or
	// text cut short), what came after the CR would overwrite the line.
	CRLF
)

// Text returns s with the characters a terminal acts on escaped (see the
// package documentation) except tab, newline and a CR right before a
// newline, for multi-line text that is written whole.
func Text(s string) string { return Escape(s, Tab|Newline|CRLF) }

// Line returns s with every character a terminal acts on escaped, tab,
// newline and CR included, for text that must stay on one line, such as
// a table cell.
func Line(s string) string { return Escape(s, 0) }

// Escape returns s with the characters a terminal acts on escaped (see
// the package documentation), except the layout characters in keep.
func Escape(s string, keep Keep) string {
	i := firstUnsafe(s, keep)
	if i < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	b.WriteString(s[:i])
	for i < len(s) {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case kept(s, i, keep):
				b.WriteByte(c)
			case c == '\t':
				b.WriteString(`\t`)
			case c == '\n':
				b.WriteString(`\n`)
			case c == '\r':
				b.WriteString(`\r`)
			case c < 0x20 || c == 0x7f:
				writeHex(&b, `\x`, uint32(c), 2)
			default:
				b.WriteByte(c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			writeHex(&b, `\x`, uint32(c), 2)
		case unsafeRune(r):
			writeHex(&b, `\u`, uint32(r), 4)
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// firstUnsafe returns the index of the first byte of s that Escape
// changes, or -1.
func firstUnsafe(s string, keep Keep) int {
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if (c < 0x20 || c == 0x7f) && !kept(s, i, keep) {
				return i
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || unsafeRune(r) {
			return i
		}
		i += size
	}
	return -1
}

// kept reports whether s[i] is a layout character in keep.
func kept(s string, i int, keep Keep) bool {
	switch s[i] {
	case '\t':
		return keep&Tab != 0
	case '\n':
		return keep&Newline != 0
	case '\r':
		// Only with the newline kept too: a CR before an escaped newline
		// would be a lone CR in the output.
		return keep&(CRLF|Newline) == CRLF|Newline && i+1 < len(s) && s[i+1] == '\n'
	}
	return false
}

// unsafeRune reports whether r, outside ASCII, is a C1 control or a
// bidirectional control that Escape writes as \uHHHH.
func unsafeRune(r rune) bool {
	return (r >= 0x80 && r <= 0x9f) || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}

func writeHex(b *strings.Builder, prefix string, v uint32, digits int) {
	const hex = "0123456789abcdef"
	b.WriteString(prefix)
	for shift := 4 * (digits - 1); shift >= 0; shift -= 4 {
		b.WriteByte(hex[v>>uint(shift)&0xf])
	}
}
