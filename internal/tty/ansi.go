package tty

import "strings"

// StripANSI removes terminal escape sequences from s: CSI sequences such
// as SGR color ("\x1b[1;31m"), OSC sequences such as hyperlinks
// ("\x1b]8;;url\x1b\\"), and other two-byte ESC sequences. The visible
// text is kept.
//
// Sequence syntax follows ECMA-48 sections 5.4 (control sequences) and
// 5.6 (control strings): https://ecma-international.org/publications-and-standards/standards/ecma-48/
func StripANSI(s string) string {
	if !strings.ContainsRune(s, 0x1b) && !strings.ContainsRune(s, 0x9b) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c != 0x1b {
			b.WriteByte(c)
			i++
			continue
		}
		i++
		if i >= len(s) {
			break
		}
		switch s[i] {
		case '[': // CSI: parameters 0x30-0x3F, intermediates 0x20-0x2F, final 0x40-0x7E
			i++
			for i < len(s) && s[i] >= 0x20 && s[i] <= 0x3f {
				i++
			}
			if i < len(s) && s[i] >= 0x40 && s[i] <= 0x7e {
				i++
			}
		case ']', 'P', '_', '^', 'X': // OSC, DCS, APC, PM, SOS: up to BEL or ST
			i++
			for i < len(s) {
				if s[i] == 0x07 {
					i++
					break
				}
				if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		default: // two-byte sequence such as ESC 7 or ESC ( B
			if s[i] >= 0x20 && s[i] <= 0x2f && i+1 < len(s) {
				i++
			}
			i++
		}
	}
	return b.String()
}

// VisibleWidth returns the number of terminal columns s occupies once its
// escape sequences are removed.
func VisibleWidth(s string) int {
	return StringWidth(StripANSI(s))
}

// PadRight pads s with spaces to width visible columns. Unlike
// fmt's %-20s, escape sequences in s do not count toward the width.
func PadRight(s string, width int) string {
	if pad := width - VisibleWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
