package clix_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/autocomplete"
	"github.com/SCKelemen/clix/v2/ext/config"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/help"
	"github.com/SCKelemen/clix/v2/ext/version"
)

// allFive returns the five command and flag extensions.
func allFive() []clix.Extension {
	return []clix.Extension{help.Extension{}, config.Extension{}, autocomplete.Extension{}, format.Extension{}, version.Extension{Version: "1.2.3"}}
}

// fiveApp builds an app using all five extensions, with a required root
// --project flag and a "widgets get" command. The root is built by build,
// so callers can use WithAppRoot or replace Root.
func fiveApp(t *testing.T, build func(out *bytes.Buffer) *clix.App) (*clix.App, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_PROJECT", "")
	t.Setenv("PROBE_FORMAT", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	out := &bytes.Buffer{}
	var app *clix.App
	if build != nil {
		app = build(out)
	} else {
		app = clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(out), clix.WithAppInteractive(clix.InteractiveNever))
	}
	app.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	var project string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Short: "p", Usage: "Project", Required: true}, Value: &project})
	get := clix.NewCommand("get", clix.WithCommandShort("Get a widget"), clix.WithCommandRun(func(ctx *clix.Context) error {
		return clix.FormatData(ctx.App.Out, map[string]string{"project": project}, format.OutputFormat(ctx.App))
	}))
	app.Root.AddCommand(clix.NewGroup("widgets", "Manage widgets", get))
	for _, ext := range allFive() {
		app.AddExtension(ext)
	}
	return app, out
}

func fiveRun(t *testing.T, app *clix.App, out *bytes.Buffer, args ...string) string {
	t.Helper()
	out.Reset()
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("%q: %v\n%s", args, err, out.String())
	}
	return out.String()
}

// checkFive runs one command of each extension and a user command.
func checkFive(t *testing.T, app *clix.App, out *bytes.Buffer) {
	t.Helper()
	if got := fiveRun(t, app, out, "help", "widgets", "get"); !strings.HasPrefix(got, "Get a widget\n") || !strings.Contains(got, "--format") || !strings.Contains(got, "--project") {
		t.Errorf("help widgets get:\n%s", got)
	}
	fiveRun(t, app, out, "config", "set", "region", "eu")
	if got := fiveRun(t, app, out, "config", "list", "-f", "json"); got != "{\n  \"region\": \"eu\"\n}\n" {
		t.Errorf("config list -f json: %q", got)
	}
	if got := fiveRun(t, app, out, "--version"); got != "probe version 1.2.3\n" {
		t.Errorf("--version: %q", got)
	}
	if got := fiveRun(t, app, out, "version", "--format", "json"); !strings.Contains(got, `"version": "1.2.3"`) {
		t.Errorf("version --format json: %q", got)
	}
	if got := fiveRun(t, app, out, "autocomplete", "bash"); !strings.Contains(got, "__complete") {
		t.Errorf("autocomplete bash printed no script:\n%.200s", got)
	}
	if got := fiveRun(t, app, out, "widgets", "get", "-p", "p1", "-f", "json"); got != "{\n  \"project\": \"p1\"\n}\n" {
		t.Errorf("widgets get -p p1 -f json: %q", got)
	}
}

func TestExtensionsE2E_AllFiveValidate(t *testing.T) {
	app, out := fiveApp(t, nil)
	if err := app.Validate(); err != nil {
		t.Errorf("App.Validate() = %v", err)
	}
	checkFive(t, app, out)
	if err := app.Validate(); err != nil {
		t.Errorf("App.Validate() after runs = %v", err)
	}
}

func TestExtensionsE2E_AddedTwice(t *testing.T) {
	app, out := fiveApp(t, nil)
	for _, ext := range allFive() {
		app.AddExtension(ext)
	}
	if err := app.Validate(); err != nil {
		t.Errorf("App.Validate() with every extension added twice = %v", err)
	}
	checkFive(t, app, out)
	counts := map[string]int{}
	for _, c := range app.Root.Children {
		counts[c.Name]++
	}
	for name, n := range counts {
		if n != 1 {
			t.Errorf("%d %q commands", n, name)
		}
	}
	flags := map[string]int{}
	for _, f := range app.Flags().Flags() {
		flags[f.Name]++
	}
	if flags["format"] != 1 || flags["version"] != 1 {
		t.Errorf("root flags: %v", flags)
	}
}

func TestExtensionsE2E_WithAppRoot(t *testing.T) {
	app, out := fiveApp(t, func(out *bytes.Buffer) *clix.App {
		root := clix.NewCommand("probe")
		return clix.NewApp("probe", clix.WithAppRoot(root), clix.WithAppOut(out), clix.WithAppErr(out), clix.WithAppInteractive(clix.InteractiveNever))
	})
	checkFive(t, app, out)
	if err := app.Validate(); err != nil {
		t.Errorf("App.Validate() = %v", err)
	}
}

// Replacing Root before the first Run: the extensions attach to the new
// root, and root flags registered earlier move with it.
func TestExtensionsE2E_ReplacedRoot(t *testing.T) {
	app, out := fiveApp(t, func(out *bytes.Buffer) *clix.App {
		app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(out), clix.WithAppInteractive(clix.InteractiveNever))
		app.Root = clix.NewCommand("probe")
		return app
	})
	checkFive(t, app, out)
	if err := app.Validate(); err != nil {
		t.Errorf("App.Validate() = %v", err)
	}
}

// Extension commands do not demand the app's required flags; the app's own
// commands still do.
func TestExtensionsE2E_SkipRequiredFlags(t *testing.T) {
	app, out := fiveApp(t, nil)
	for _, args := range [][]string{
		{"help"}, {"help", "widgets"}, {"config", "list"}, {"config", "set", "a", "1"}, {"config", "get", "a"},
		{"version"}, {"--version"}, {"autocomplete", "zsh"}, {"--help"}, {"widgets", "get", "--help"},
		{"__complete", "--cur=", "--", "probe", "help"},
	} {
		fiveRun(t, app, out, args...)
	}
	out.Reset()
	err := app.Run(context.Background(), []string{"widgets", "get"})
	var missing *clix.MissingFlagsError
	if err == nil || !errors.As(err, &missing) || missing.Flags[0] != "project" {
		t.Errorf("widgets get without --project: %v", err)
	}
}

// The help command's argument completes command names, through the
// autocomplete extension's __complete command.
func TestExtensionsE2E_HelpCompletion(t *testing.T) {
	app, out := fiveApp(t, nil)
	got := fiveRun(t, app, out, "__complete", "--cur=", "--", "probe", "help", "widgets")
	if !strings.Contains(got, "get\tGet a widget") {
		t.Errorf("completing help widgets <TAB>:\n%s", got)
	}
	got = fiveRun(t, app, out, "__complete", "--cur=wid", "--", "probe", "help")
	if !strings.Contains(got, "widgets\tManage widgets") || strings.Contains(got, "version") {
		t.Errorf("completing help wid<TAB>:\n%s", got)
	}
}

// One App runs many times in sequence without state from one Run leaking
// into the next.
func TestExtensionsE2E_SequentialRuns(t *testing.T) {
	app, out := fiveApp(t, nil)
	first := map[string]string{}
	sequence := [][]string{
		{"help", "widgets", "get"}, {"version", "-f", "json"}, {"version"}, {"widgets", "get", "-p", "a"},
		{"widgets", "get", "-p", "b", "-f", "yaml"}, {"config", "list"}, {"help"}, {"--version"},
	}
	for round := 0; round < 3; round++ {
		for _, args := range sequence {
			got := fiveRun(t, app, out, args...)
			key := strings.Join(args, " ")
			if round == 0 {
				first[key] = got
			} else if got != first[key] {
				t.Errorf("round %d, %q:\n%s\nfirst round:\n%s", round, key, got, first[key])
			}
		}
	}
	// -f json on one run does not make the next run's version JSON.
	if v := first["version"]; json.Valid([]byte(v)) || !strings.Contains(v, "version = 1.2.3") {
		t.Errorf("version after version -f json:\n%s", v)
	}
}
