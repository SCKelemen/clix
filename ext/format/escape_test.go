package format_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// hostileRecords hold strings a server could send to rewrite a terminal:
// an SGR color, an OSC 8 hyperlink whose text hides its target, a CR that
// overwrites the line, and a bidi override.
var hostileRecords = []map[string]string{
	{"name": "a\x1b[31mred\x1b[0m", "link": "\x1b]8;;https://evil.test\x1b\\https://good.test\x1b]8;;\x1b\\"},
	{"name": "safe\rEVIL", "link": "file\u202etxt.exe"},
}

// escapeApp has a "list" command that prints hostileRecords with opts.
func escapeApp(t *testing.T, opts clix.FormatOptions) (*clix.App, *bytes.Buffer, *bool) {
	t.Helper()
	app, out, ran := projApp(t, format.Extension{Projections: true})
	app.Root.AddCommand(clix.NewCommand("hostile", clix.WithCommandRun(func(ctx *clix.Context) error {
		*ran = true
		return format.WriteWith(ctx, hostileRecords, opts)
	})))
	return app, out, ran
}

// hasControl reports whether s holds a control character other than tab
// and newline, or a bidi override.
func hasControl(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return (r < 0x20 && r != '\t' && r != '\n') || (r >= 0x7f && r < 0xa0) || r == 0x202e
	})
}

// In a pipe, the text formats and gcloud's escape control characters, and
// --jq and --template print the exact text, as "jq -r" does.
func TestWriteEscapesInPipe(t *testing.T) {
	app, out, ran := escapeApp(t, clix.FormatOptions{})
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"hostile"}, `\x1b]8;;https://evil.test\x1b\\https://good.test\x1b]8;;\x1b\\` + "\t" + `a\x1b[31mred\x1b[0m` + "\n" +
			`file\u202etxt.exe` + "\t" + `safe\rEVIL` + "\n"},
		{[]string{"hostile", "--format", "value(name)"}, `a\x1b[31mred\x1b[0m` + "\n" + `safe\rEVIL` + "\n"},
		{[]string{"hostile", "--format", "table(name,link)"}, "NAME                 LINK\n" +
			`a\x1b[31mred\x1b[0m  \x1b]8;;https://evil.test\x1b\https://good.test\x1b]8;;\x1b\` + "\n" +
			`safe\rEVIL           file\u202etxt.exe` + "\n"},
		{[]string{"hostile", "--format", "list(name)"}, ` - a\x1b[31mred\x1b[0m` + "\n" + ` - safe\rEVIL` + "\n"},
		{[]string{"hostile", "--format", "flattened(name)"}, "---\nname: " + `a\x1b[31mred\x1b[0m` + "\n---\nname: " + `safe\rEVIL` + "\n"},
		{[]string{"hostile", "--format", "csv(name)"}, "name\n" + `a\x1b[31mred\x1b[0m` + "\n" + `safe\rEVIL` + "\n"},
	} {
		got, err := runWrite(t, app, out, ran, tc.args...)
		if err != nil {
			t.Fatalf("%q: %v", tc.args, err)
		}
		if got != tc.want || hasControl(got) {
			t.Errorf("%q:\ngot  %q\nwant %q", tc.args, got, tc.want)
		}
	}
	// --jq and --template in a pipe keep the bytes, for programs.
	for _, args := range [][]string{
		{"hostile", "--jq", ".[].name"},
		{"hostile", "--template", `{{range .}}{{.name}}{{"\n"}}{{end}}`},
	} {
		got, err := runWrite(t, app, out, ran, args...)
		if want := "a\x1b[31mred\x1b[0m\nsafe\rEVIL\n"; err != nil || got != want {
			t.Errorf("%q: %q, %v; want %q", args, got, err, want)
		}
	}
	// JSON escapes control characters itself; --jq prints non-strings as
	// JSON.
	for _, args := range [][]string{{"hostile", "--format", "json"}, {"hostile", "--json", "name"}, {"hostile", "--jq", ".[0]"}} {
		got, err := runWrite(t, app, out, ran, args...)
		if err != nil || !strings.Contains(got, `a\u001b[31mred\u001b[0m`) || strings.Contains(got, "\x1b") {
			t.Errorf("%q: %q, %v", args, got, err)
		}
	}
}

// Text without control characters prints byte for byte as before.
func TestWriteCleanUnchanged(t *testing.T) {
	app, out, ran := projApp(t, format.Extension{Projections: true})
	records := []map[string]string{{"name": "Café 日本語 👍🏽", "path": `C:\dir`, "note": "tab\there"}, {"name": "plain", "path": "/x", "note": "two\nlines"}}
	for _, raw := range []bool{false, true} {
		opts := clix.FormatOptions{RawControlChars: raw}
		app.Root.AddCommand(clix.NewCommand(map[bool]string{false: "escaped", true: "raw"}[raw], clix.WithCommandRun(func(ctx *clix.Context) error {
			*ran = true
			return format.WriteWith(ctx, records, opts)
		})))
	}
	for _, flags := range [][]string{nil, {"--format", "value(name,path,note)"}, {"--format", "table(name,path,note)"},
		{"--format", "flattened"}, {"--format", "list"}, {"--format", "csv(name,note)"}, {"--format", "yaml"}, {"--jq", ".[].name"},
		{"--template", `{{range .}}{{.name}} {{.note}}{{end}}`}} {
		escaped, err := runWrite(t, app, out, ran, append([]string{"escaped"}, flags...)...)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := runWrite(t, app, out, ran, append([]string{"raw"}, flags...)...)
		if err != nil {
			t.Fatal(err)
		}
		if escaped != raw {
			t.Errorf("%q: escaped\n%q\nraw\n%q", flags, escaped, raw)
		}
	}
}

// RawControlChars prints the control characters as they are; gcloud
// formats then print what gcloud prints.
func TestWriteRawControlChars(t *testing.T) {
	app, out, ran := escapeApp(t, clix.FormatOptions{RawControlChars: true})
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"hostile", "--format", "value(name)"}, "a\x1b[31mred\x1b[0m\nsafe\rEVIL\n"},
		{[]string{"hostile", "--format", "list(link)"}, " - \x1b]8;;https://evil.test\x1b\\https://good.test\x1b]8;;\x1b\\\n - file\u202etxt.exe\n"},
		// TSV keeps only its own escapes.
		{[]string{"hostile"}, "\x1b]8;;https://evil.test\x1b\\\\https://good.test\x1b]8;;\x1b\\\\\ta\x1b[31mred\x1b[0m\nfile\u202etxt.exe\tsafe\\rEVIL\n"},
	} {
		got, err := runWrite(t, app, out, ran, tc.args...)
		if err != nil || got != tc.want {
			t.Errorf("%q:\ngot  %q, %v\nwant %q", tc.args, got, err, tc.want)
		}
	}
}
