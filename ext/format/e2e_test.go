package format_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/autocomplete"
	"github.com/SCKelemen/clix/v2/ext/format"
	"go.yaml.in/yaml/v3"
)

type e2eItem struct {
	Name string            `json:"name"`
	Tags []string          `json:"tags,omitempty"`
	Meta map[string]string `json:"meta,omitempty"`
}

// e2eApp builds "probe get", which prints a slice of structs with
// clix.FormatData in the selected format. Config and environment are
// isolated.
func e2eApp(t *testing.T) (*clix.App, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_FORMAT", "")
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	app.AddExtension(format.Extension{})
	get := clix.NewCommand("get", clix.WithCommandRun(func(ctx *clix.Context) error {
		data := []e2eItem{
			{Name: "a", Tags: []string{"x", "y"}, Meta: map[string]string{"zone": "eu", "app": "web"}},
			{Name: "b"},
		}
		return clix.FormatData(ctx.App.Out, data, format.OutputFormat(ctx.App))
	}))
	app.Root.AddCommand(get)
	return app, out
}

func run(t *testing.T, app *clix.App, out *bytes.Buffer, args ...string) (string, error) {
	t.Helper()
	out.Reset()
	err := app.Run(context.Background(), args)
	return out.String(), err
}

const e2eText = "meta.app = web\nmeta.zone = eu\nname = a\ntags = x, y\n\nname = b\n"

func TestFormatE2E_DefaultIsText(t *testing.T) {
	app, out := e2eApp(t)
	got, err := run(t, app, out, "get")
	if err != nil {
		t.Fatal(err)
	}
	if got != e2eText {
		t.Errorf("default output %q, want %q", got, e2eText)
	}
	// Deterministic: a second run prints the same.
	if again, _ := run(t, app, out, "get"); again != got {
		t.Errorf("second run printed %q", again)
	}
}

func TestFormatE2E_AcceptsEachFormat(t *testing.T) {
	for _, args := range [][]string{
		{"get", "--format", "json"}, {"get", "--format=JSON"}, {"get", "-f", "json"}, {"--format", "json", "get"}, {"get", "-fjson"},
	} {
		app, out := e2eApp(t)
		got, err := run(t, app, out, args...)
		if err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		var items []e2eItem
		if err := json.Unmarshal([]byte(got), &items); err != nil || len(items) != 2 || items[0].Meta["zone"] != "eu" {
			t.Errorf("%q: output is not the JSON list (%v):\n%s", args, err, got)
		}
	}
	for _, args := range [][]string{{"get", "--format", "yaml"}, {"get", "-f", "YAML"}} {
		app, out := e2eApp(t)
		got, err := run(t, app, out, args...)
		if err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		var items []map[string]any
		if err := yaml.Unmarshal([]byte(got), &items); err != nil || len(items) != 2 {
			t.Errorf("%q: output is not a YAML list (%v):\n%s", args, err, got)
		}
	}
	for _, args := range [][]string{{"get", "--format", "text"}, {"get", "--format="}, {"get", "-f", "Text"}} {
		app, out := e2eApp(t)
		got, err := run(t, app, out, args...)
		if err != nil || got != e2eText {
			t.Errorf("%q: output %q, err %v; want text", args, got, err)
		}
	}
}

func TestFormatE2E_UnknownFormatIsUsageError(t *testing.T) {
	for _, args := range [][]string{{"get", "--format", "jsno"}, {"get", "-f", "xml"}, {"--format=table", "get"}} {
		app, out := e2eApp(t)
		got, err := run(t, app, out, args...)
		if err == nil {
			t.Fatalf("%q ran and printed %q", args, got)
		}
		if code := clix.ExitCode(err); code != 2 {
			t.Errorf("%q: exit code %d, want 2 (%v)", args, code, err)
		}
		if !strings.Contains(err.Error(), "valid formats: json, yaml, text") {
			t.Errorf("%q: error %q does not list the valid formats", args, err)
		}
		if got != "" {
			t.Errorf("%q: printed %q before failing", args, got)
		}
	}
}

func TestFormatE2E_EnvironmentVariable(t *testing.T) {
	app, out := e2eApp(t)
	t.Setenv("PROBE_FORMAT", "json")
	got, err := run(t, app, out, "get")
	if err != nil || !strings.HasPrefix(got, "[") {
		t.Errorf("PROBE_FORMAT=json: output %q, err %v", got, err)
	}
	// The command line beats the environment.
	if got, err := run(t, app, out, "get", "--format", "text"); err != nil || got != e2eText {
		t.Errorf("--format text with PROBE_FORMAT=json: output %q, err %v", got, err)
	}
	t.Setenv("PROBE_FORMAT", "bogus")
	_, err = run(t, app, out, "get")
	if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), "PROBE_FORMAT") {
		t.Errorf("PROBE_FORMAT=bogus: err %v (exit code %d), want a usage error naming the variable", err, clix.ExitCode(err))
	}
}

func TestFormatE2E_ConfigKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("format: yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, out := e2eApp(t)
	app.ConfigPath = path
	got, err := run(t, app, out, "get")
	if err != nil || !strings.HasPrefix(got, "- ") {
		t.Errorf("format: yaml in the config file: output %q, err %v", got, err)
	}

	bad := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(bad, []byte("format: table\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, out = e2eApp(t)
	app.ConfigPath = bad
	_, err = run(t, app, out, "get")
	if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), `config file key "format"`) {
		t.Errorf("format: table in the config file: err %v (exit code %d), want a usage error naming the key", err, clix.ExitCode(err))
	}
}

// The app's own -f always wins; the extension's flag stays reachable as
// --format.
func TestFormatE2E_AppShortFlagWins(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, app *clix.App, force *bool)
	}{
		{"app flag before AddExtension", nil},
		{"app flag after AddExtension", func(t *testing.T, app *clix.App, force *bool) {
			app.AddExtension(format.Extension{})
			addForce(app, force)
		}},
		{"app flag after a first Run", func(t *testing.T, app *clix.App, force *bool) {
			app.AddExtension(format.Extension{})
			if err := app.Run(context.Background(), []string{"get"}); err != nil {
				t.Fatal(err)
			}
			addForce(app, force)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			out := &bytes.Buffer{}
			app := clix.NewApp("probe", clix.WithAppOut(out))
			var force bool
			app.Root.AddCommand(clix.NewCommand("get", clix.WithCommandRun(func(ctx *clix.Context) error {
				return clix.FormatData(ctx.App.Out, map[string]bool{"force": force}, format.OutputFormat(ctx.App))
			})))
			if tc.setup == nil {
				addForce(app, &force)
				app.AddExtension(format.Extension{})
			} else {
				tc.setup(t, app, &force)
			}
			if got, err := run(t, app, out, "get", "-f", "--format", "json"); err != nil || !force || !strings.Contains(got, `"force": true`) {
				t.Errorf("get -f --format json: output %q, force %v, err %v", got, force, err)
			}
			if err := app.Validate(); err == nil || !strings.Contains(err.Error(), "-f") {
				t.Errorf("App.Validate() = %v, want a report that --format lost -f", err)
			}
		})
	}
}

func addForce(app *clix.App, force *bool) {
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force", Short: "f", Usage: "Force"}, Value: force})
}

// An app flag named --format replaces the extension's.
func TestFormatE2E_AppLongFlagWins(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out))
	var mine string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "format", Usage: "Report format"}, Value: &mine})
	app.AddExtension(format.Extension{})
	app.Root.AddCommand(clix.NewCommand("get", clix.WithCommandRun(func(*clix.Context) error { return nil })))
	if _, err := run(t, app, out, "get", "--format", "table"); err != nil || mine != "table" {
		t.Errorf("--format table with the app's own --format: mine %q, err %v", mine, err)
	}
}

func TestFormatE2E_AddedTwice(t *testing.T) {
	app, out := e2eApp(t)
	app.AddExtension(format.Extension{})
	if got, err := run(t, app, out, "get", "-f", "json"); err != nil || !strings.HasPrefix(got, "[") {
		t.Errorf("-f json with the extension added twice: %q, %v", got, err)
	}
	if err := app.Validate(); err != nil {
		t.Errorf("App.Validate() = %v", err)
	}
	if n := countFlags(app.Flags(), "format"); n != 1 {
		t.Errorf("%d --format flags registered", n)
	}
}

func countFlags(fs *clix.FlagSet, name string) int {
	n := 0
	for _, f := range fs.Flags() {
		if f.Name == name {
			n++
		}
	}
	return n
}

// Shell completion offers the formats for -f and --format (through the
// autocomplete extension's __complete command), not file names.
func TestFormatE2E_CompletesFormats(t *testing.T) {
	for _, words := range [][]string{{"probe", "-f"}, {"probe", "get", "--format"}} {
		app, out := e2eApp(t)
		app.AddExtension(autocomplete.Extension{})
		got, err := run(t, app, out, append([]string{"__complete", "--cur=", "--"}, words...)...)
		if err != nil {
			t.Fatal(err)
		}
		if got != "json\nyaml\ntext\n:nofiles\n" {
			t.Errorf("completing %q <TAB>:\n%s", words, got)
		}
	}
	// The app's own --format keeps its completion (none here).
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	app := clix.NewApp("probe", clix.WithAppOut(&bytes.Buffer{}))
	var mine string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "format"}, Value: &mine})
	app.AddExtension(format.Extension{})
	if err := app.ApplyExtensions(); err != nil {
		t.Fatal(err)
	}
	if app.Flags().Lookup("format").Completer() != nil {
		t.Error("the extension set a completion on the app's own --format")
	}
}
