package projection

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/gcloudtest"
	"github.com/SCKelemen/clix/v2/internal/resource"
)

// hostileDoc holds strings that would rewrite a terminal, in values and
// in a key: an SGR color, an OSC 8 hyperlink, a window title ended by
// BEL, a CR that overwrites the line, a C1 CSI and a bidi override.
const hostileDoc = `[{"name":"a\u001b[31mred\u001b[0m","link":"\u001b]8;;https://evil.test\u001b\\click\u001b]8;;\u001b\\",` +
	`"title":"\u001b]0;pwned\u0007","cr":"safe\rEVIL","c1":"\u009b2J","bidi":"x\u202eyz","crlf":"one\r\ntwo",` +
	`"lab":{"k\u001b[1m":"v"}}]`

var escapeFormats = []string{
	"table(name,link,title,cr,c1,bidi,crlf)",
	"csv(name,link,title,cr,c1,bidi,crlf,lab)",
	"value(name,link,title,cr,c1,bidi,crlf,lab)",
	"flattened(name,link,title,cr,c1,bidi,crlf,lab)",
	"list(name,link,title,cr,c1,bidi,crlf,lab)",
}

func printDoc(t *testing.T, format, doc string, opts Options) string {
	t.Helper()
	f, err := Parse(format)
	if err != nil {
		t.Fatal(err)
	}
	data, err := resource.Decode([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := f.Print(&b, data, opts); err != nil {
		t.Fatalf("%s: %v", format, err)
	}
	return b.String()
}

// The printers show control characters as visible escapes, where gcloud
// prints them as they are and clix used to fail.
func TestPrintEscapes(t *testing.T) {
	for _, format := range escapeFormats {
		got := printDoc(t, format, hostileDoc, Options{UTF8: true})
		if i := strings.IndexFunc(got, func(r rune) bool {
			return (r < 0x20 && r != '\t' && r != '\n' && r != '\r') || (r >= 0x7f && r < 0xa0) || r == 0x202e
		}); i >= 0 {
			t.Errorf("%s: control character at %d in %q", format, i, got)
		}
		// No CR but that of a Windows line ending, right before its newline.
		if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\r") {
			t.Errorf("%s: a lone CR in %q", format, got)
		}
		for _, shown := range []string{`a\x1b[31mred\x1b[0m`, `\x1b]8;;https://evil.test\x1b\click\x1b]8;;\x1b\`, `\x1b]0;pwned\x07`, `safe\rEVIL`, `\u009b2J`, `x\u202eyz`} {
			if !strings.Contains(got, shown) {
				t.Errorf("%s: %q does not show %q", format, got, shown)
			}
		}
		if !strings.HasPrefix(format, "table") && !strings.Contains(got, `k\x1b[1m`) {
			t.Errorf("%s: %q does not show the key k\\x1b[1m", format, got)
		}
	}
	// A table lays a cell out on several lines and pads them, so the CR of
	// a Windows line ending is escaped too; elsewhere the value is written
	// whole and keeps it.
	if got := printDoc(t, "table(crlf,name)", `[{"crlf":"one\r\ntwo","name":"n"}]`, Options{UTF8: true}); got != "CRLF   NAME\none\\r  n\ntwo\n" {
		t.Errorf("table: %q", got)
	}
	if got := printDoc(t, "value(crlf,name)", `[{"crlf":"one\r\ntwo","name":"n"}]`, Options{UTF8: true}); got != "one\r\ntwo\tn\n" {
		t.Errorf("value: %q", got)
	}
	// Escapes are ASCII, so an ASCII table accepts an escaped bidi control.
	if got := printDoc(t, "table(bidi)", hostileDoc, Options{}); got != "BIDI\nx\\u202eyz\n" {
		t.Errorf("ASCII table: %q", got)
	}
}

// RawControl prints what gcloud prints: the control characters as they
// are.
func TestPrintRawControl(t *testing.T) {
	for _, format := range escapeFormats {
		got := printDoc(t, format, hostileDoc, Options{UTF8: true, RawControl: true})
		for _, raw := range []string{"\x1b[31m", "\x1b]8;;", "\x07", "safe\rEVIL", "\u009b", "\u202e"} {
			if !strings.Contains(got, raw) {
				t.Errorf("%s: %q does not have %q as it is", format, got, raw)
			}
		}
	}
}

// TestPrintRawControlMatchesGcloud checks RawControl output against
// gcloud's, byte for byte, when CLIX_GCLOUD_SDK is set. Not for table:
// gcloud's column widths skip ESC [ sequences when it takes the terminal
// (TERM) to support ANSI, and count them otherwise; clix counts every
// character.
func TestPrintRawControlMatchesGcloud(t *testing.T) {
	var cases []printCase
	for _, f := range escapeFormats {
		if !strings.HasPrefix(f, "table") {
			cases = append(cases, printCase{f, hostileDoc})
		}
	}
	var want []printResult
	gcloudtest.Run(t, printScript, cases, &want)
	for i, c := range cases {
		if want[i].Error != "" {
			t.Errorf("%s: gcloud failed: %s", c.Format, want[i].Error)
			continue
		}
		if got := printDoc(t, c.Format, c.JSON, Options{UTF8: true, RawControl: true}); got != want[i].Out {
			t.Errorf("%s:\n clix   %q\n gcloud %q", c.Format, got, want[i].Out)
		}
	}
}
