package clix

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/SCKelemen/clix/v2/internal/table"
	"go.yaml.in/yaml/v3"
)

// hostile are strings a server could send to rewrite what a terminal
// shows, with how text output shows them.
var hostile = []struct{ name, in, shown string }{
	{"color", "\x1b[31mred\x1b[0m", `\x1b[31mred\x1b[0m`},
	{"clear screen", "\x1b[2J\x1b[H", `\x1b[2J\x1b[H`},
	{"OSC 8 hyperlink", "\x1b]8;;https://evil.test\x1b\\https://good.test\x1b]8;;\x1b\\", `\x1b]8;;https://evil.test\x1b\https://good.test\x1b]8;;\x1b\`},
	{"window title", "\x1b]0;pwned\x07", `\x1b]0;pwned\x07`},
	{"CR overwrite", "rm -rf /tmp/x\rls", `rm -rf /tmp/x\rls`},
	{"backspace", "allow\b\b\b\b\bdeny", `allow\x08\x08\x08\x08\x08deny`},
	{"C1 CSI", "\u009b31mred", `\u009b31mred`},
	{"8-bit CSI", "\x9b31mred", `\x9b31mred`},
	{"bidi override", "invoice\u202efdp.exe", `invoice\u202efdp.exe`},
}

// clean are strings text output prints byte for byte.
var clean = []string{"", "plain", `C:\dir\x1b`, "Café 日本語 👍🏽 🇸🇪", "with\ttab", "two\nlines", "crlf\r\nend", "\u200e\u00a0\ufffd"}

// textOf formats data as text with opts, on a terminal when term is set.
func textOf(t *testing.T, data any, opts FormatOptions, term textTerminal) string {
	t.Helper()
	var b bytes.Buffer
	node := textNode(data)
	if rows, ok := tableRows(node); ok {
		if err := writeTable(&b, data, rows, opts, term); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	if err := FormatDataWith(&b, data, opts); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// shapes puts s in each kind of value text output prints, s as a key too.
func shapes(s string) map[string]any {
	type item struct {
		Name string `json:"name"`
	}
	return map[string]any{
		"scalar":     s,
		"stringer":   testStringer(s),
		"list":       []string{"a", s},
		"anyList":    []any{"a", s, 1},
		"map":        map[string]string{"k": s, s: "v"},
		"anyMap":     map[string]any{"k": s, "n": 1},
		"nested":     map[string]any{"outer": map[string]any{s: []any{s, map[string]any{"x": s}}}},
		"struct":     struct{ Name, Other string }{s, "x"},
		"table":      []item{{Name: s}, {Name: "b"}},
		"mapTable":   []map[string]string{{"name": s, s: "header"}},
		"mixedBlock": []any{s, map[string]any{"k": s}},
	}
}

type testStringer string

func (s testStringer) String() string { return string(s) }

var textModes = map[string]textTerminal{"pipe": {}, "terminal": {on: true, width: 300, ellipsis: "…"}}

func TestTextEscapesControlChars(t *testing.T) {
	for _, h := range hostile {
		for shape, data := range shapes(h.in) {
			for mode, term := range textModes {
				got := textOf(t, data, FormatOptions{}, term)
				if !utf8.ValidString(got) {
					t.Errorf("%s/%s/%s: invalid UTF-8 in %q", h.name, shape, mode, got)
				}
				if i := strings.IndexFunc(got, func(r rune) bool {
					return (r < 0x20 && r != '\t' && r != '\n') || (r >= 0x7f && r < 0xa0) || (r >= 0x202a && r <= 0x202e)
				}); i >= 0 {
					t.Errorf("%s/%s/%s: control character %q at %d in %q", h.name, shape, mode, got[i:i+1], i, got)
				}
				want := h.shown
				if mode == "pipe" && (shape == "table" || shape == "mapTable") {
					want = EscapeControlChars(table.EscapeTSV(h.in)) // a backslash is \\ in TSV
				}
				// Values that go through JSON have U+FFFD for invalid UTF-8.
				fffd := strings.ReplaceAll(want, `\x9b`, "\ufffd")
				if !strings.Contains(got, want) && !strings.Contains(got, fffd) {
					t.Errorf("%s/%s/%s: %q does not show %q", h.name, shape, mode, got, want)
				}
			}
		}
	}
}

// Text without control characters prints exactly as before: escaping
// changes nothing, which RawControlChars shows by printing it as it is.
func TestTextCleanUnchanged(t *testing.T) {
	for _, s := range clean {
		for shape, data := range shapes(s) {
			for mode, term := range textModes {
				escaped := textOf(t, data, FormatOptions{}, term)
				raw := textOf(t, data, FormatOptions{RawControlChars: true}, term)
				if escaped != raw {
					t.Errorf("%q/%s/%s: escaped\n%q\nraw\n%q", s, shape, mode, escaped, raw)
				}
			}
		}
	}
	// And byte for byte what the value holds, outside tables.
	for _, s := range clean {
		if got := textOf(t, s, FormatOptions{}, textTerminal{}); got != s+"\n" {
			t.Errorf("FormatData(%q) = %q", s, got)
		}
		if got := textOf(t, map[string]string{"k": s}, FormatOptions{}, textTerminal{}); got != "k = "+s+"\n" {
			t.Errorf("FormatData(k: %q) = %q", s, got)
		}
	}
}

// RawControlChars prints control characters as they are, except that
// TSV keeps its escapes and a terminal table escapes them all.
func TestTextRawControlChars(t *testing.T) {
	raw := FormatOptions{RawControlChars: true}
	colored := "\x1b[32mOK\x1b[0m"
	if got := textOf(t, map[string]string{"status": colored}, raw, textTerminal{}); got != "status = "+colored+"\n" {
		t.Errorf("raw map: %q", got)
	}
	if got := textOf(t, []string{colored, "a\rb"}, raw, textTerminal{}); got != colored+"\na\rb\n" {
		t.Errorf("raw list: %q", got)
	}
	type row struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	rows := []row{{colored, "two\nlines\tand\rCR"}}
	if got, want := textOf(t, rows, raw, textModes["terminal"]), textOf(t, rows, FormatOptions{}, textModes["terminal"]); got != want {
		t.Errorf("raw terminal table:\n%q\nwant it escaped, as\n%q", got, want)
	}
	if got, want := textOf(t, rows, raw, textTerminal{}), colored+"\ttwo\\nlines\\tand\\rCR\n"; got != want {
		t.Errorf("raw TSV:\n%q\nwant\n%q", got, want)
	}
	// The default escapes all of it.
	if got, want := textOf(t, rows, FormatOptions{}, textTerminal{}), `\x1b[32mOK\x1b[0m`+"\ttwo\\nlines\\tand\\rCR\n"; got != want {
		t.Errorf("TSV:\n%q\nwant\n%q", got, want)
	}
}

// A Windows line ending ends a line, as a newline does; a CR elsewhere
// is escaped, and tables escape both.
func TestTextWindowsLineEndings(t *testing.T) {
	in := "first\r\nsecond\r\n"
	if got := textOf(t, in, FormatOptions{}, textTerminal{}); got != in+"\n" {
		t.Errorf("FormatData(%q) = %q", in, got)
	}
	if got := textOf(t, "a\r\r\nb\r", FormatOptions{}, textTerminal{}); got != "a\\r\r\nb\\r\n" {
		t.Errorf("lone CRs: %q", got)
	}
	rows := []map[string]string{{"text": in}}
	if got := textOf(t, rows, FormatOptions{}, textModes["terminal"]); got != "TEXT\n"+`first\r\nsecond\r\n`+"\n" {
		t.Errorf("terminal table: %q", got)
	}
	if got := textOf(t, rows, FormatOptions{}, textTerminal{}); got != `first\r\nsecond\r\n`+"\n" {
		t.Errorf("TSV: %q", got)
	}
}

// JSON and YAML escape control characters themselves (JSON as \u001b,
// YAML as \e), and RawControlChars does not change them.
func TestStructuredFormatsUnchanged(t *testing.T) {
	for _, h := range hostile {
		if h.name == "8-bit CSI" {
			continue // invalid UTF-8: JSON writes U+FFFD and YAML !!binary
		}
		data := map[string]string{"k": h.in}
		for _, format := range []string{FormatJSON, FormatYAML} {
			var a, b bytes.Buffer
			if err := FormatData(&a, data, format); err != nil {
				t.Fatal(err)
			}
			if err := FormatDataWith(&b, data, FormatOptions{Format: format, RawControlChars: true}); err != nil {
				t.Fatal(err)
			}
			if a.String() != b.String() {
				t.Errorf("%s/%s: RawControlChars changed the output", h.name, format)
			}
			if strings.ContainsAny(a.String(), "\x1b\x07\x08\r") {
				t.Errorf("%s/%s: %q has a raw C0 control character", h.name, format, a.String())
			}
			var back map[string]string
			var err error
			if format == FormatJSON {
				err = json.Unmarshal(a.Bytes(), &back)
			} else {
				err = yaml.Unmarshal(a.Bytes(), &back)
			}
			if err != nil || back["k"] != h.in {
				t.Errorf("%s/%s: decodes to %q, %v", h.name, format, back["k"], err)
			}
		}
	}
}

func TestEscapeControlChars(t *testing.T) {
	for _, h := range hostile {
		if got := EscapeControlChars(h.in); got != h.shown {
			t.Errorf("EscapeControlChars(%q) = %q, want %q", h.in, got, h.shown)
		}
	}
	for _, s := range clean {
		if got := EscapeControlChars(s); got != s {
			t.Errorf("EscapeControlChars(%q) = %q, want it unchanged", s, got)
		}
	}
}
