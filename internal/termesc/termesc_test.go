package termesc

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Escape keeps exactly the layout characters asked for.
func TestEscapeKeep(t *testing.T) {
	in := "a\tb\nc\r\nd\re"
	for keep, want := range map[Keep]string{
		0:                    `a\tb\nc\r\nd\re`,
		Tab:                  "a\tb\\nc\\r\\nd\\re",
		Newline:              "a\\tb\nc\\r\nd\\re",
		Tab | Newline:        "a\tb\nc\\r\nd\\re",
		Tab | Newline | CRLF: "a\tb\nc\r\nd\\re",
		CRLF:                 `a\tb\nc\r\nd\re`, // CRLF without Newline: the newline is escaped
	} {
		if got := Escape(in, keep); got != want {
			t.Errorf("Escape(%q, %b) = %q, want %q", in, keep, got, want)
		}
	}
}

func TestText(t *testing.T) {
	for _, tt := range []struct{ in, text, line string }{
		// Plain ASCII and UTF-8, and backslashes, are unchanged.
		{"", "", ""},
		{"plain ASCII ~!@#$%^&*()", "plain ASCII ~!@#$%^&*()", "plain ASCII ~!@#$%^&*()"},
		{`C:\dir\x1b`, `C:\dir\x1b`, `C:\dir\x1b`},
		{"Café 日本語 👍🏽 🇸🇪 \u00a0\u00ff\u200b\ufeff\ufffd", "Café 日本語 👍🏽 🇸🇪 \u00a0\u00ff\u200b\ufeff\ufffd", "Café 日本語 👍🏽 🇸🇪 \u00a0\u00ff\u200b\ufeff\ufffd"},
		// Tab and newline: kept by Text, escaped by Line.
		{"a\tb\nc", "a\tb\nc", `a\tb\nc`},
		// CR overwrites the line unless a newline follows it.
		{"safe\rEVIL", `safe\rEVIL`, `safe\rEVIL`},
		{"one\r\ntwo\r\n", "one\r\ntwo\r\n", `one\r\ntwo\r\n`},
		{"a\r\r\nb\r", "a\\r\r\nb\\r", `a\r\r\nb\r`},
		// ESC sequences: SGR color, OSC 8 hyperlink with ST, OSC title with
		// BEL, cursor movement, and a screen clear.
		{"\x1b[31mred\x1b[0m", `\x1b[31mred\x1b[0m`, `\x1b[31mred\x1b[0m`},
		{"\x1b]8;;https://evil.test\x1b\\click\x1b]8;;\x1b\\", `\x1b]8;;https://evil.test\x1b\click\x1b]8;;\x1b\`, `\x1b]8;;https://evil.test\x1b\click\x1b]8;;\x1b\`},
		{"\x1b]0;pwned\x07", `\x1b]0;pwned\x07`, `\x1b]0;pwned\x07`},
		{"ok\x1b[1A\x1b[2K", `ok\x1b[1A\x1b[2K`, `ok\x1b[1A\x1b[2K`},
		// Other C0 controls and DEL.
		{"\x00\x01\x07\x08\x0b\x0c\x1f\x7f", `\x00\x01\x07\x08\x0b\x0c\x1f\x7f`, `\x00\x01\x07\x08\x0b\x0c\x1f\x7f`},
		// C1 controls: CSI, OSC, ST and NEL.
		{"\u009b31m \u009d0;t\u009c \u0085", `\u009b31m \u009d0;t\u009c \u0085`, `\u009b31m \u009d0;t\u009c \u0085`},
		// Bidirectional controls, but not the marks.
		{"file\u202etxt.exe \u2066x\u2069 \u202a\u202b\u202c\u202d \u2067\u2068", `file\u202etxt.exe \u2066x\u2069 \u202a\u202b\u202c\u202d \u2067\u2068`, `file\u202etxt.exe \u2066x\u2069 \u202a\u202b\u202c\u202d \u2067\u2068`},
		{"\u200e\u200f\u061c", "\u200e\u200f\u061c", "\u200e\u200f\u061c"},
		// Invalid UTF-8: each byte, such as a raw 8-bit CSI.
		{"a\x9b31mb\xff\xc3", `a\x9b31mb\xff\xc3`, `a\x9b31mb\xff\xc3`},
	} {
		if got := Text(tt.in); got != tt.text {
			t.Errorf("Text(%q) = %q, want %q", tt.in, got, tt.text)
		}
		if got := Line(tt.in); got != tt.line {
			t.Errorf("Line(%q) = %q, want %q", tt.in, got, tt.line)
		}
	}
}

// Clean text is returned as it is, without a copy.
func TestCleanAllocs(t *testing.T) {
	s := strings.Repeat("plain text, Café 日本語\twith a tab\n", 10)
	if n := testing.AllocsPerRun(100, func() { _ = Text(s) }); n != 0 {
		t.Errorf("Text of clean text allocates %v times", n)
	}
}

func FuzzEscape(f *testing.F) {
	for _, s := range []string{"", "plain", "a\x1b[31mb", "\r\n\r", "\u009b\u202e", "\xff\xc3", "\t\n"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		text, line := Text(s), Line(s)
		for name, out := range map[string]string{"Text": text, "Line": line} {
			// The result is valid UTF-8 with nothing a terminal acts on,
			// except what Text keeps.
			if !utf8.ValidString(out) {
				t.Fatalf("%s(%q) = %q is not valid UTF-8", name, s, out)
			}
			keep := Keep(0)
			if name == "Text" {
				keep = Tab | Newline | CRLF
			}
			if i := firstUnsafe(out, keep); i >= 0 {
				t.Fatalf("%s(%q) = %q has %q at %d", name, s, out, out[i], i)
			}
			// Escaping twice changes nothing more.
			if again := Escape(out, keep); again != out {
				t.Fatalf("%s is not idempotent on %q: %q, then %q", name, s, out, again)
			}
		}
		if strings.ContainsAny(line, "\t\n\r") {
			t.Fatalf("Line(%q) = %q keeps a layout character", s, line)
		}
		// A lone CR is never kept, and without CRLF no CR is.
		if strings.Contains(strings.ReplaceAll(text, "\r\n", ""), "\r") {
			t.Fatalf("Text(%q) = %q keeps a lone CR", s, text)
		}
		if strings.Contains(Escape(s, Tab|Newline), "\r") {
			t.Fatalf("Escape(%q, Tab|Newline) keeps a CR", s)
		}
		// Text that needs no escaping is unchanged.
		if firstUnsafe(s, Tab|Newline|CRLF) < 0 && text != s {
			t.Fatalf("Text(%q) = %q changed clean text", s, text)
		}
	})
}
