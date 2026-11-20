package format_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/autocomplete"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/internal/gcloudtest"
)

// projApp is writeApp with format.Extension{Projections: true}, plus
// cols, a command that passes FormatOptions.Columns, and esc, whose data
// holds an escape sequence.
func projApp(t *testing.T, ext format.Extension) (*clix.App, *bytes.Buffer, *bool) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, v := range []string{"PROBE_FORMAT", "PROBE_JSON", "PROBE_JQ", "PROBE_TEMPLATE", "NO_COLOR", "CLICOLOR_FORCE"} {
		t.Setenv(v, "")
	}
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	app.AddExtension(ext)
	ran := new(bool)
	add := func(name string, run func(ctx *clix.Context) error) {
		app.Root.AddCommand(clix.NewCommand(name, clix.WithCommandRun(func(ctx *clix.Context) error {
			*ran = true
			return run(ctx)
		})))
	}
	for name, data := range map[string]any{"list": wRepos, "get": wRepos[0], "empty": []wRepo{}, "count": 3,
		"esc": []map[string]string{{"name": "a\x1b[31mred"}}} {
		add(name, func(ctx *clix.Context) error { return format.Write(ctx, data) })
	}
	add("cols", func(ctx *clix.Context) error {
		return format.WriteWith(ctx, wRepos, clix.FormatOptions{Columns: []string{"name", "owner.login"}})
	})
	return app, out, ran
}

func TestProjectionFormats(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"list", "--format", "table(name, owner.login, stars:label=STARS:align=right, labels.name.join(','))"},
			"NAME  LOGIN  STARS  LABELS_NAME\nclix  sam       42  cli,go\njq               7\n"},
		{[]string{"list", "--format=table[box](name, stars)"},
			"┌──────┬───────┐\n│ NAME │ STARS │\n├──────┼───────┤\n│ clix │ 42    │\n│ jq   │ 7     │\n└──────┴───────┘\n"},
		{[]string{"list", "--format", "value(name, stars)"}, "clix\t42\njq\t7\n"},
		{[]string{"list", "--format", "value[separator=' '](name, labels.name)"}, "clix cli;go\njq \n"},
		{[]string{"list", "--format", "csv(name, displayName, url)"},
			"name,display_name,url\nclix,CLI kit,https://example.com/clix\njq,Query <&>,https://example.com/jq\n"},
		// As in gcloud, a key's flattened name is its label in lower case.
		{[]string{"get", "--format", "flattened(name, owner, labels[0].name)"},
			"labels_name: cli\nname:        clix\nowner.id:    1\nowner.login: sam\n"},
		{[]string{"list", "--format", "list(name)"}, " - clix\n - jq\n"},
		{[]string{"list", "--format", "json(name, owner.login)"},
			"[\n  {\n    \"name\": \"clix\",\n    \"owner\": {\n      \"login\": \"sam\"\n    }\n  },\n  {\n    \"name\": \"jq\"\n  }\n]\n"},
		{[]string{"list", "--format", "yaml(name, owner.login)"}, "- name: clix\n  owner:\n    login: sam\n- name: jq\n"},
		{[]string{"get", "--format", "json(name)"}, "{\n  \"name\": \"clix\"\n}\n"},
		{[]string{"list", "--format", "none"}, ""},
		// Without a projection, table, csv and value print every
		// top-level field, or the command's FormatOptions.Columns.
		{[]string{"list", "--format", "value"},
			"clix\tCLI kit\t42\tid=1;login=sam\t{'name': 'cli', 'color': 'green'};{'name': 'go', 'color': 'blue'}\t2026-11-17T12:00:00Z\thttps://example.com/clix\n" +
				"jq\tQuery <&>\t7\t\t\t2025-01-01T00:00:00Z\thttps://example.com/jq\n"},
		{[]string{"cols", "--format", "table"}, "NAME  LOGIN\nclix  sam\njq\n"},
		{[]string{"cols", "--format", "csv(stars)"}, "stars\n42\n7\n"},
		// No resources print nothing, and JSON prints [].
		{[]string{"empty", "--format", "table"}, ""},
		{[]string{"empty", "--format", "table(name)"}, ""},
		{[]string{"empty", "--format", "json(name)"}, "[]\n"},
		{[]string{"empty", "--format", "yaml(name)"}, ""},
	} {
		app, out, ran := projApp(t, format.Extension{Projections: true})
		got, err := runWrite(t, app, out, ran, tt.args...)
		if err != nil || got != tt.want {
			t.Errorf("%q: err %v, printed\n%s\nwant\n%s", tt.args, err, got, tt.want)
		}
	}
}

// json, yaml and text alone print exactly as without Projections.
func TestProjectionsKeepPlainFormats(t *testing.T) {
	for _, args := range [][]string{
		{"list"}, {"list", "--format", "json"}, {"list", "--format", "YAML"}, {"get", "--format", "text"},
		{"list", "--json", "name"}, {"list", "--jq", ".[0].name"}, {"maps"}, {"count", "--format", "json"},
	} {
		plainApp, plainOut, plainRan := writeApp(t)
		want, wantErr := runWrite(t, plainApp, plainOut, plainRan, args...)
		app, out, ran := projApp(t, format.Extension{Projections: true})
		app.Root.AddCommand(clix.NewCommand("maps", clix.WithCommandRun(func(ctx *clix.Context) error {
			return format.Write(ctx, []map[string]any{{"b": 1, "a": "x"}, {"c": true}})
		})))
		got, err := runWrite(t, app, out, ran, args...)
		if got != want || (err == nil) != (wantErr == nil) {
			t.Errorf("%q: with Projections %q, %v; without %q, %v", args, got, err, want, wantErr)
		}
	}
	// " json " is json, as a projection expression.
	app, out, ran := projApp(t, format.Extension{Projections: true})
	got, err := runWrite(t, app, out, ran, "list", "--format", " json ")
	if err != nil || got != formatJSON(t, wRepos) {
		t.Errorf(`--format " json ": %v, %q`, err, got)
	}
}

func TestProjectionsOffByDefault(t *testing.T) {
	app, out, ran := projApp(t, format.Extension{})
	_, err := runWrite(t, app, out, ran, "list", "--format", "table(name)")
	if err == nil || clix.ExitCode(err) != 2 || *ran || !strings.Contains(err.Error(), "valid formats: json, yaml, text") {
		t.Errorf("without Projections: err %v (exit %d), ran %v; want a usage error", err, clix.ExitCode(err), *ran)
	}
}

func TestProjectionErrors(t *testing.T) {
	for _, tt := range []struct {
		args     []string
		usage    bool
		contains string
	}{
		{[]string{"list", "--format", "bogus(name)"}, true, `unknown format "bogus"`},
		{[]string{"list", "--format", "table[foo](name)"}, true, `the table attribute "foo" is not supported in clix --format`},
		{[]string{"list", "--format", "table(name.date())"}, true, "the transform date() is not supported in clix --format"},
		{[]string{"list", "--format", "text(name)"}, true, "text is clix's own format"},
		{[]string{"list", "--format", "table(name"}, true, "More tokens expected"},
		{[]string{"list", "--format", "table()"}, true, "requires a non-empty projection"},
		{[]string{"count", "--format", "table"}, true, "needs a projection"},
		{[]string{"esc", "--format", "value(name)"}, false, "control character U+001B"},
	} {
		app, out, ran := projApp(t, format.Extension{Projections: true})
		got, err := runWrite(t, app, out, ran, tt.args...)
		switch {
		case err == nil:
			t.Errorf("%q: no error, printed %q", tt.args, got)
		case !strings.Contains(err.Error(), tt.contains):
			t.Errorf("%q: %v, want it to mention %q", tt.args, err, tt.contains)
		case tt.usage != (clix.ExitCode(err) == 2):
			t.Errorf("%q: exit code %d, usage error %v", tt.args, clix.ExitCode(err), tt.usage)
		case got != "":
			t.Errorf("%q: printed %q before the error", tt.args, got)
		}
		// Syntax is checked before the command runs.
		if tt.usage && strings.HasPrefix(tt.args[2], "table(name") && *ran {
			t.Errorf("%q: the command ran", tt.args)
		}
	}
}

func TestProjectionFromEnvironmentAndConfig(t *testing.T) {
	app, out, ran := projApp(t, format.Extension{Projections: true})
	t.Setenv("PROBE_FORMAT", "value(name)")
	if got, err := runWrite(t, app, out, ran, "list"); err != nil || got != "clix\njq\n" {
		t.Errorf("PROBE_FORMAT=value(name): %q, %v", got, err)
	}
	// The command line wins.
	if got, err := runWrite(t, app, out, ran, "list", "--format", "value(stars)"); err != nil || got != "42\n7\n" {
		t.Errorf("--format value(stars) over the environment: %q, %v", got, err)
	}
	// --json, --jq and --template win over a --format from the environment.
	if got, err := runWrite(t, app, out, ran, "list", "--jq", ".[1].name"); err != nil || got != "jq\n" {
		t.Errorf("--jq with PROBE_FORMAT=value(name): %q, %v", got, err)
	}
	if got, err := runWrite(t, app, out, ran, "get", "--json", "name"); err != nil || got != "{\n  \"name\": \"clix\"\n}\n" {
		t.Errorf("--json with PROBE_FORMAT=value(name): %q, %v", got, err)
	}
	t.Setenv("PROBE_FORMAT", "table[nope](name)")
	_, err := runWrite(t, app, out, ran, "list")
	if err == nil || clix.ExitCode(err) != 2 || *ran || !strings.Contains(err.Error(), "PROBE_FORMAT") {
		t.Errorf("PROBE_FORMAT=table[nope](name): err %v, ran %v; want a usage error naming the variable", err, *ran)
	}
	t.Setenv("PROBE_FORMAT", "")

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("format: 'csv[no-heading](name, stars)'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, out, ran = projApp(t, format.Extension{Projections: true})
	app.ConfigPath = path
	if got, err := runWrite(t, app, out, ran, "list"); err != nil || got != "clix,42\njq,7\n" {
		t.Errorf("format: csv[...] in the config file: %q, %v", got, err)
	}
	if got, err := runWrite(t, app, out, ran, "list", "--template", "{{len .}}"); err != nil || got != "2" {
		t.Errorf("--template with a config file projection: %q, %v", got, err)
	}
}

func TestProjectionConflicts(t *testing.T) {
	for _, args := range [][]string{
		{"list", "--format", "table(name)", "--json", "name"},
		{"list", "--format", "json(name)", "--json", "name"},
		{"list", "--format", "value(name)", "--jq", "."},
		{"list", "--format", "flattened", "--template", "{{.}}"},
		{"list", "--json", "name", "--format=csv(name)"},
	} {
		app, out, ran := projApp(t, format.Extension{Projections: true})
		got, err := runWrite(t, app, out, ran, args...)
		if err == nil || clix.ExitCode(err) != 2 || got != "" || !strings.Contains(err.Error(), "works on JSON output and cannot be used with --format") {
			t.Errorf("%q: err %v (exit %d), printed %q; want a usage error", args, err, clix.ExitCode(err), got)
		}
		if !strings.Contains(err.Error(), args[slicesIndex(args, "--format")+1]) && !strings.Contains(err.Error(), "csv(name)") {
			t.Errorf("%q: %v does not quote the format as given", args, err)
		}
	}
	// --format json still goes with --json.
	app, out, ran := projApp(t, format.Extension{Projections: true})
	if got, err := runWrite(t, app, out, ran, "get", "--format", "json", "--json", "name"); err != nil || got != "{\n  \"name\": \"clix\"\n}\n" {
		t.Errorf("--format json --json name: %q, %v", got, err)
	}
}

func slicesIndex(args []string, s string) int {
	for i, a := range args {
		if a == s {
			return i
		}
	}
	return len(args) - 2
}

// The app's own --format replaces the extension's, and with it the
// projections.
func TestProjectionAppFormatFlagWins(t *testing.T) {
	app, out, ran := projApp(t, format.Extension{Projections: true})
	var mine string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "format"}, Value: &mine})
	got, err := runWrite(t, app, out, ran, "list", "--format", "table(name)")
	if err != nil || mine != "table(name)" || strings.Contains(got, "NAME") {
		t.Errorf("app --format: %q, %v, mine %q", got, err, mine)
	}
	if format.OutputFormat(app) != clix.FormatText {
		t.Errorf("OutputFormat = %q", format.OutputFormat(app))
	}
}

func TestProjectionOutputFormat(t *testing.T) {
	for value, want := range map[string]string{
		"json(name)": clix.FormatJSON, "yaml": clix.FormatYAML, " yaml ": clix.FormatYAML, "table(name)": clix.FormatText,
		"value(a)": clix.FormatText, "json": clix.FormatJSON, "flattened": clix.FormatText,
	} {
		app := clix.NewApp("probe")
		app.AddExtension(format.Extension{Projections: true})
		if err := app.ApplyExtensions(); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Flags().Parse([]string{"--format", value}); err != nil {
			t.Fatalf("--format %q: %v", value, err)
		}
		if got := format.OutputFormat(app); got != want {
			t.Errorf("OutputFormat with --format %q = %q, want %q", value, got, want)
		}
	}
}

func TestProjectionCompletionAndHelp(t *testing.T) {
	app, _, _ := projApp(t, format.Extension{Projections: true})
	if err := app.ApplyExtensions(); err != nil {
		t.Fatal(err)
	}
	f := app.Flags().Lookup("format")
	if f == nil || !strings.Contains(f.Usage, "NAME[ATTRIBUTES](PROJECTION)") || !strings.Contains(f.Usage, "table") {
		t.Fatalf("usage: %+v", f)
	}
	app, out, ran := projApp(t, format.Extension{Projections: true})
	app.AddExtension(autocomplete.Extension{})
	got, err := runWrite(t, app, out, ran, "__complete", "--cur=", "--", "probe", "--format")
	if want := "json\nyaml\ntext\ntable\ncsv\nvalue\nflattened\nlist\nnone\n:nofiles\n"; err != nil || got != want {
		t.Errorf("completing --format <TAB>: %v\n%s\nwant\n%s", err, got, want)
	}
}

// jsonPrinterScript prints with gcloud's json printer.
const jsonPrinterScript = `
import io
from googlecloudsdk.core.resource import resource_printer
out = []
for c in json.load(sys.stdin):
    buf = io.StringIO()
    resource_printer.Print(json.loads(c["json"]), c["format"], out=buf)
    out.append(buf.getvalue())
print(json.dumps(out))
`

// TestProjectedJSONMatchesGcloud checks that json(...) prints byte for
// byte what gcloud's json printer prints for the same resources.
func TestProjectedJSONMatchesGcloud(t *testing.T) {
	type c struct {
		Format string `json:"format"`
		JSON   string `json:"json"`
	}
	formats := []string{"json(name, owner.login)", "json(labels[].name, stars, url.basename())", "json(createdAt, note, owner)", "json()"}
	var cases []c
	for _, f := range formats {
		cases = append(cases, c{f, formatJSON(t, wRepos)}, c{f, formatJSON(t, wRepos[0])})
	}
	var want []string
	gcloudtest.Golden(t, "testdata/gcloud_json.json", jsonPrinterScript, cases, &want)
	for i, cs := range cases {
		app, out, ran := projApp(t, format.Extension{Projections: true})
		cmd := "list"
		var probe any
		_ = json.Unmarshal([]byte(cs.JSON), &probe)
		if _, ok := probe.(map[string]any); ok {
			cmd = "get"
		}
		got, err := runWrite(t, app, out, ran, cmd, "--format", cs.Format)
		if err != nil || got != want[i] {
			t.Errorf("%s %s: %v\n--- clix\n%s--- gcloud\n%s---", cmd, cs.Format, err, got, want[i])
		}
	}
}
