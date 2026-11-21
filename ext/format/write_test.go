package format_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

type wOwner struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
}

type wLabel struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type wRepo struct {
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Stars       int       `json:"stars"`
	Owner       *wOwner   `json:"owner"`
	Labels      []wLabel  `json:"labels"`
	Created     time.Time `json:"createdAt"`
	Note        string    `json:"note,omitempty"`
	Secret      string    `json:"-"`
	URL         string    `json:"url"`
}

var wRepos = []wRepo{
	{Name: "clix", DisplayName: "CLI kit", Stars: 42, Owner: &wOwner{"sam", 1},
		Labels:  []wLabel{{"cli", "green"}, {"go", "blue"}},
		Created: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC), Secret: "s", URL: "https://example.com/clix"},
	{Name: "jq", DisplayName: "Query <&>", Stars: 7, Created: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), URL: "https://example.com/jq"},
}

// writeApp builds an app whose commands print with format.Write: list (a
// slice of structs), get (one struct), empty (an empty slice), count (a
// number) and mk (a command with a required flag, which records that it
// ran).
func writeApp(t *testing.T) (*clix.App, *bytes.Buffer, *bool) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, v := range []string{"PROBE_FORMAT", "PROBE_JSON", "PROBE_JQ", "PROBE_TEMPLATE", "NO_COLOR", "CLICOLOR_FORCE"} {
		t.Setenv(v, "")
	}
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	app.AddExtension(format.Extension{})
	ran := new(bool)
	cmd := func(name string, data any) *clix.Command {
		return clix.NewCommand(name, clix.WithCommandRun(func(ctx *clix.Context) error {
			*ran = true
			return format.Write(ctx, data)
		}))
	}
	app.Root.AddCommand(cmd("list", wRepos))
	app.Root.AddCommand(cmd("get", wRepos[0]))
	app.Root.AddCommand(cmd("empty", []wRepo{}))
	app.Root.AddCommand(cmd("count", 3))
	app.Root.AddCommand(cmd("maps", []map[string]any{{"b": 1, "a": "x"}, {"c": true}}))
	return app, out, ran
}

func runWrite(t *testing.T, app *clix.App, out *bytes.Buffer, ran *bool, args ...string) (string, error) {
	t.Helper()
	out.Reset()
	*ran = false
	err := app.Run(context.Background(), args)
	return out.String(), err
}

func formatJSON(t *testing.T, data any) string {
	t.Helper()
	var b bytes.Buffer
	if err := clix.FormatData(&b, data, clix.FormatJSON); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

const wFields = "\n  createdAt\n  displayName\n  labels\n  name\n  note\n  owner\n  stars\n  url"

// --json alone, --json= and --json followed by another flag are usage
// errors that list the fields, sorted, including omitempty fields and,
// for an empty list, the fields of its element type.
func TestJSONWithoutFieldsListsThem(t *testing.T) {
	for _, args := range [][]string{
		{"list", "--json"},
		{"list", "--json="},
		{"list", "--json", ""},
		{"list", "--json", ","},
		{"--json", "--format", "json", "list"},
		{"list", "--json", "--format=json"},
		{"empty", "--json"},
		{"get", "--json"},
	} {
		app, out, ran := writeApp(t)
		got, err := runWrite(t, app, out, ran, args...)
		want := "specify one or more comma-separated fields for --json:" + wFields
		if err == nil || err.Error() != want || clix.ExitCode(err) != 2 {
			t.Errorf("%q: err = %v (exit %d), want usage error %q", args, err, clix.ExitCode(err), want)
		}
		if got != "" {
			t.Errorf("%q: printed %q before the error", args, got)
		}
	}

	app, out, ran := writeApp(t)
	_, err := runWrite(t, app, out, ran, "count", "--json")
	if err == nil || !strings.Contains(err.Error(), "this output has no fields") {
		t.Errorf("count --json: err = %v", err)
	}
	_, err = runWrite(t, app, out, ran, "maps", "--json")
	if err == nil || !strings.HasSuffix(err.Error(), ":\n  a\n  b\n  c") {
		t.Errorf("maps --json: err = %v", err)
	}
}

func TestJSONUnknownField(t *testing.T) {
	app, out, ran := writeApp(t)
	_, err := runWrite(t, app, out, ran, "list", "--json", "name,nope")
	want := "unknown field for --json: \"nope\"\navailable fields:" + wFields
	if err == nil || err.Error() != want || clix.ExitCode(err) != 2 {
		t.Errorf("err = %v, want %q", err, want)
	}
	// Fields tagged json:"-" do not exist.
	if _, err := runWrite(t, app, out, ran, "list", "--json", "Secret"); err == nil {
		t.Error("--json Secret: no error")
	}
	// Syntax is checked before the command runs.
	if _, err := runWrite(t, app, out, ran, "list", "--json", "owner..login"); err == nil || *ran || clix.ExitCode(err) != 2 {
		t.Errorf("--json owner..login: err = %v, ran = %v", err, *ran)
	}
}

// --json fields restrict every object, keeping the full output's order
// and formatting; dotted paths select nested fields, also through lists.
func TestJSONSelectsFields(t *testing.T) {
	app, out, ran := writeApp(t)
	got, err := runWrite(t, app, out, ran, "list", "--json", "url, name,owner.login,labels.name")
	if err != nil {
		t.Fatal(err)
	}
	want := `[
  {
    "name": "clix",
    "owner": {
      "login": "sam"
    },
    "labels": [
      {
        "name": "cli"
      },
      {
        "name": "go"
      }
    ],
    "url": "https://example.com/clix"
  },
  {
    "name": "jq",
    "owner": null,
    "labels": [],
    "url": "https://example.com/jq"
  }
]
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	got, err = runWrite(t, app, out, ran, "get", "--json=stars,displayName")
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"displayName\": \"CLI kit\",\n  \"stars\": 42\n}\n"; got != want {
		t.Errorf("get: got %q, want %q", got, want)
	}

	// Every field selected is exactly --format json, with null for the
	// field both objects omit (note is omitempty).
	all := "name,displayName,stars,owner,labels,createdAt,note,url"
	got, err = runWrite(t, app, out, ran, "list", "--json", all)
	if err != nil {
		t.Fatal(err)
	}
	withNote := strings.ReplaceAll(formatJSON(t, wRepos), "\"url\": \"https://example.com/clix\"\n", "\"url\": \"https://example.com/clix\",\n    \"note\": null\n")
	withNote = strings.ReplaceAll(withNote, "\"url\": \"https://example.com/jq\"\n", "\"url\": \"https://example.com/jq\",\n    \"note\": null\n")
	if got != withNote {
		t.Errorf("all fields:\n%s\nwant (--format json with a null note):\n%s", got, withNote)
	}
	got, _ = runWrite(t, app, out, ran, "list", "--format", "json")
	if got != formatJSON(t, wRepos) {
		t.Errorf("--format json changed: %q", got)
	}

	// A whole field wins over a path into it.
	got, err = runWrite(t, app, out, ran, "get", "--json", "owner.id,owner")
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"owner\": {\n    \"login\": \"sam\",\n    \"id\": 1\n  }\n}\n"; got != want {
		t.Errorf("owner.id,owner: got %q", got)
	}
}

// --json, --jq and --template print JSON-based output, so --format must be
// json (or unset) on the command line; a format from the environment
// gives way.
func TestJSONFlagsAndFormat(t *testing.T) {
	app, out, ran := writeApp(t)
	for _, args := range [][]string{
		{"list", "--json", "name", "--format", "yaml"},
		{"list", "--json", "name", "-f", "text"},
		{"list", "--template", "{{.}}", "--format", "yaml"},
	} {
		_, err := runWrite(t, app, out, ran, args...)
		if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), "cannot be used with --format") {
			t.Errorf("%q: err = %v", args, err)
		}
	}
	if _, err := runWrite(t, app, out, ran, "list", "--jq", ".", "--template", "x"); err == nil || !strings.Contains(err.Error(), "--jq and --template cannot be used together") {
		t.Errorf("--jq with --template: err = %v", err)
	}
	got, err := runWrite(t, app, out, ran, "get", "--json", "stars", "--format", "json")
	if err != nil || got != "{\n  \"stars\": 42\n}\n" {
		t.Errorf("--format json --json stars: %q, %v", got, err)
	}
	t.Setenv("PROBE_FORMAT", "yaml")
	got, err = runWrite(t, app, out, ran, "get", "--json", "stars")
	if err != nil || got != "{\n  \"stars\": 42\n}\n" {
		t.Errorf("PROBE_FORMAT=yaml --json stars: %q, %v", got, err)
	}
	// ... and still applies without the JSON flags.
	got, _ = runWrite(t, app, out, ran, "get")
	if !strings.HasPrefix(got, "name: clix\n") {
		t.Errorf("PROBE_FORMAT=yaml: %q", got)
	}
}

// --json, --jq and --template come only from the command line.
func TestJSONFlagsAreCommandLineOnly(t *testing.T) {
	app, out, ran := writeApp(t)
	t.Setenv("PROBE_JSON", "name")
	t.Setenv("PROBE_TEMPLATE", "{{.}}")
	got, err := runWrite(t, app, out, ran, "count")
	if err != nil || got != "3\n" {
		t.Errorf("with PROBE_JSON and PROBE_TEMPLATE set: %q, %v", got, err)
	}
}

// Values do not leak from one Run to the next.
func TestJSONFlagsReset(t *testing.T) {
	app, out, ran := writeApp(t)
	if _, err := runWrite(t, app, out, ran, "get", "--json", "stars"); err != nil {
		t.Fatal(err)
	}
	got, err := runWrite(t, app, out, ran, "count")
	if err != nil || got != "3\n" {
		t.Errorf("second run: %q, %v", got, err)
	}
	if format.OutputFormat(app) != clix.FormatText {
		t.Errorf("OutputFormat after a run without --json = %q", format.OutputFormat(app))
	}
}

func TestOutputFormatWithJSON(t *testing.T) {
	app, out, ran := writeApp(t)
	app.Root.AddCommand(clix.NewCommand("plain", clix.WithCommandRun(func(ctx *clix.Context) error {
		return clix.FormatData(ctx.App.Out, map[string]int{"n": 1}, format.OutputFormat(ctx.App))
	})))
	got, err := runWrite(t, app, out, ran, "plain", "--json", "n")
	if err != nil || got != "{\n  \"n\": 1\n}\n" {
		t.Errorf("OutputFormat with --json: %q, %v", got, err)
	}
}

// An app flag named --json replaces the extension's, and Write ignores it.
func TestAppJSONFlagWins(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppInteractive(clix.InteractiveNever))
	var mine bool
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "json"}, Value: &mine})
	app.AddExtension(format.Extension{})
	app.Root.AddCommand(clix.NewCommand("count", clix.WithCommandRun(func(ctx *clix.Context) error {
		return format.Write(ctx, 3)
	})))
	if err := app.Run(context.Background(), []string{"count", "--json"}); err != nil {
		t.Fatal(err)
	}
	if !mine || out.String() != "3\n" {
		t.Errorf("app --json = %v, output %q", mine, out.String())
	}
}

func TestTemplate(t *testing.T) {
	app, out, ran := writeApp(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"list", "--template", `{{range .}}{{.name}} {{.stars}}{{"\n"}}{{end}}`}, "clix 42\njq 7\n"},
		{[]string{"list", "-t", `{{range .}}{{if gt .stars 10}}{{.name}}{{end}}{{end}}`}, "clix"},
		{[]string{"get", "--template", `{{join ", " (pluck "name" .labels)}}`}, "cli, go"},
		{[]string{"get", "--template", `{{.owner.login}}/{{.name}}`}, "sam/clix"},
		{[]string{"get", "--template", `{{truncate 5 .displayName}}|{{truncate 10 .displayName}}`}, "CL...|CLI kit"},
		{[]string{"get", "--template", `{{timefmt "2006-01-02" .createdAt}}`}, "2026-11-17"},
		{[]string{"get", "--template", `{{timeago .createdAt}}`}, "about 2 days ago"},
		{[]string{"get", "--template", `{{color "green+b" .name}} {{autocolor "red" .name}}`}, "clix clix"},
		{[]string{"get", "--template", `{{hyperlink .url .name}}`}, "clix"},
		{[]string{"list", "--template", `{{range .}}{{tablerow .name .stars .url}}{{end}}`},
			"clix  42  https://example.com/clix\njq    7   https://example.com/jq\n"},
		{[]string{"list", "--template", `{{range .}}{{tablerow .name}}{{end}}{{tablerender}}-{{"\n"}}{{tablerow "x" "y"}}`},
			"clix\njq\n-\nx  y\n"},
		{[]string{"list", "--json", "name", "--template", `{{range .}}{{.stars}}{{end}}`}, "<no value><no value>"},
		{[]string{"maps", "--template", `{{range .}}{{.a}}{{end}}`}, "x<no value>"},
	} {
		restore := format.SetNowForTest(time.Date(2026, 11, 19, 13, 0, 0, 0, time.UTC))
		got, err := runWrite(t, app, out, ran, tc.args...)
		restore()
		if err != nil || got != tc.want {
			t.Errorf("%q: got %q, %v; want %q", tc.args, got, err, tc.want)
		}
	}

	// color and hyperlink follow CLICOLOR_FORCE and NO_COLOR.
	t.Setenv("CLICOLOR_FORCE", "1")
	got, err := runWrite(t, app, out, ran, "get", "--template", `{{color "green+b:white" .name}}{{hyperlink .url "x"}}{{color "208" "o"}}`)
	if want := "\x1b[32;1;47mclix\x1b[0m\x1b]8;;https://example.com/clix\x1b\\x\x1b]8;;\x1b\\\x1b[38;5;208mo\x1b[0m"; err != nil || got != want {
		t.Errorf("forced color: got %q, %v; want %q", got, err, want)
	}
	t.Setenv("NO_COLOR", "1")
	got, _ = runWrite(t, app, out, ran, "get", "--template", `{{color "red" .name}}`)
	if got != "clix" {
		t.Errorf("NO_COLOR: %q", got)
	}
}

func TestTemplateErrors(t *testing.T) {
	app, out, ran := writeApp(t)
	// A template that does not parse is a usage error before the command runs.
	_, err := runWrite(t, app, out, ran, "list", "--template", "{{range .}")
	if err == nil || *ran || clix.ExitCode(err) != 2 {
		t.Errorf("bad template: err = %v, ran = %v", err, *ran)
	}
	_, err = runWrite(t, app, out, ran, "list", "--template", "{{nosuchfunc .}}")
	if err == nil || *ran {
		t.Errorf("unknown function: err = %v, ran = %v", err, *ran)
	}
	// Errors while running come after the output before them.
	got, err := runWrite(t, app, out, ran, "list", "--template", `{{range .}}{{.name}}:{{color "nocolor" .name}}{{end}}`)
	if err == nil || clix.ExitCode(err) != 1 || got != "clix:" {
		t.Errorf("runtime error: %q, %v", got, err)
	}
	for _, tmpl := range []string{`{{join "," .}}`, `{{truncate 3 .}}`, `{{timeago .name}}`, `{{pluck "x" (pluck "name" .labels)}}`} {
		if _, err := runWrite(t, app, out, ran, "get", "--template", tmpl); err == nil {
			t.Errorf("%s: no error", tmpl)
		}
	}
}

func TestJQ(t *testing.T) {
	app, out, ran := writeApp(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"list", "--jq", ".[].name"}, "clix\njq\n"},
		{[]string{"list", "-q", ".[] | select(.stars > 10) | .displayName"}, "CLI kit\n"},
		{[]string{"get", "--jq", ".stars"}, "42\n"},
		{[]string{"get", "--jq", ".labels | map(.name) | join(\",\")"}, "cli,go\n"},
		{[]string{"list", "--json", "name,stars", "--jq", ".[1]"}, "{\n  \"name\": \"jq\",\n  \"stars\": 7\n}\n"},
		{[]string{"list", "--jq", `.[] | "\(.name): \(.owner.login)"`}, "clix: sam\njq: null\n"},
		{[]string{"list", "--json", "displayName", "--jq", ".[].displayName"}, "CLI kit\nQuery <&>\n"},
		{[]string{"count", "--jq", "."}, "3\n"},
		{[]string{"empty", "--jq", "length"}, "0\n"},
	} {
		got, err := runWrite(t, app, out, ran, tc.args...)
		if err != nil || got != tc.want {
			t.Errorf("%q: got %q, %v; want %q", tc.args, got, err, tc.want)
		}
	}
}

func TestJQErrors(t *testing.T) {
	app, out, ran := writeApp(t)
	// Syntax errors and constructs outside the subset are usage errors
	// before the command runs.
	for _, expr := range []string{".[", "1 +", ".a + 1", "$x", "reduce .[] as $x (0; .)", "splits(\"a\")", ""} {
		_, err := runWrite(t, app, out, ran, "list", "--jq", expr)
		if err == nil || *ran || clix.ExitCode(err) != 2 || !strings.HasPrefix(err.Error(), "invalid value for jq: ") {
			t.Errorf("--jq %q: err = %v, ran = %v", expr, err, *ran)
		}
	}
	_, err := runWrite(t, app, out, ran, "list", "--jq", ".a + 1")
	if err == nil || !strings.Contains(err.Error(), "not supported in clix --jq") {
		t.Errorf("unsupported construct: %v", err)
	}
	// A runtime error comes after the results before it, with exit status 1.
	got, err := runWrite(t, app, out, ran, "list", "--jq", ".[] | .name, .name.x")
	if err == nil || clix.ExitCode(err) != 1 || got != "clix\n" || !strings.HasPrefix(err.Error(), "--jq: ") {
		t.Errorf("runtime error: %q, %v", got, err)
	}
}
