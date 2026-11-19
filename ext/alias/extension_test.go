package alias

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/autocomplete"
	"github.com/SCKelemen/clix/v2/ext/config"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/help"
	"github.com/SCKelemen/clix/v2/ext/version"
)

type testApp struct {
	app    *clix.App
	out    *bytes.Buffer
	errOut *bytes.Buffer
	path   string
	ran    string
	args   map[string]string
}

// newTestApp returns a gh-like app with the alias extension and the other
// extensions, whose config file is in a temporary directory.
func newTestApp(t *testing.T) *testApp {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := &testApp{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
	a.app = clix.NewApp("probe", clix.WithAppOut(a.out), clix.WithAppErr(a.errOut), clix.WithAppInteractive(clix.InteractiveNever))
	a.path = filepath.Join(t.TempDir(), "config.yaml")
	a.app.ConfigPath = a.path
	record := func(ctx *clix.Context) error {
		a.ran = ctx.Command.Path()
		a.args = map[string]string{}
		for _, f := range ctx.Command.Flags.Flags() {
			if f.Name != "help" {
				a.args[f.Name] = f.Value.String()
			}
		}
		return nil
	}
	checkout := clix.NewCommand("checkout", clix.WithCommandShort("Check out a pull request"), clix.WithCommandRun(record))
	checkout.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "number", Positional: true}})
	checkout.Flags.SetCompletion("number", clix.CompleteValues("101", "102"))
	checkout.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force"}})
	a.app.Root.AddCommand(clix.NewGroup("pr", "Pull requests", checkout))
	a.app.AddExtension(Extension{})
	a.app.AddExtension(help.Extension{})
	a.app.AddExtension(config.Extension{})
	a.app.AddExtension(version.Extension{Version: "1.0.0"})
	a.app.AddExtension(format.Extension{})
	a.app.AddExtension(autocomplete.Extension{})
	return a
}

func (a *testApp) try(args ...string) (string, error) {
	a.out.Reset()
	a.errOut.Reset()
	a.ran = ""
	err := a.app.Run(context.Background(), args)
	return a.out.String(), err
}

func (a *testApp) run(t *testing.T, args ...string) string {
	t.Helper()
	out, err := a.try(args...)
	if err != nil {
		t.Fatalf("%q: %v", args, err)
	}
	return out
}

func TestAliasCommands(t *testing.T) {
	a := newTestApp(t)
	if got := a.run(t, "alias", "set", "co", "pr checkout"); got != "Added alias co: pr checkout\n" {
		t.Fatalf("set: %q", got)
	}
	a.run(t, "alias", "set", "fco", "pr checkout $1 --force")
	data, _ := os.ReadFile(a.path)
	if string(data) != "aliases.co: pr checkout\naliases.fco: pr checkout $1 --force\n" {
		t.Fatalf("config file:\n%s", data)
	}
	if info, err := os.Stat(a.path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config file mode: %v, %v", info, err)
	}

	a.run(t, "co", "7")
	if a.ran != "probe pr checkout" || a.args["number"] != "7" {
		t.Fatalf("co 7: ran %q, %v", a.ran, a.args)
	}
	a.run(t, "fco", "8")
	if a.args["number"] != "8" || a.args["force"] != "true" {
		t.Fatalf("fco 8: %v", a.args)
	}

	if got := a.run(t, "alias", "list"); got != "co:   pr checkout\nfco:  pr checkout $1 --force\n" {
		t.Fatalf("list:\n%s", got)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(a.run(t, "alias", "list", "--format", "json")), &m); err != nil || m["co"] != "pr checkout" || len(m) != 2 {
		t.Fatalf("list --format json: %v, %v", err, m)
	}

	// Replacing needs --clobber.
	if _, err := a.try("alias", "set", "co", "pr checkout --force"); err == nil || !strings.Contains(err.Error(), "--clobber") {
		t.Fatalf("set an existing alias: %v", err)
	}
	a.run(t, "alias", "set", "co", "pr checkout --force", "--clobber")
	if a.app.Aliases()["co"] != "pr checkout --force" {
		t.Fatal("--clobber did not replace the alias")
	}

	if got := a.run(t, "alias", "delete", "co"); got != "Deleted alias co\n" {
		t.Fatalf("delete: %q", got)
	}
	if _, err := a.try("alias", "delete", "co"); err == nil || !strings.Contains(err.Error(), `no such alias "co"`) {
		t.Fatalf("delete a missing alias: %v", err)
	}
	a.run(t, "alias", "set", "x", "pr checkout")
	a.run(t, "alias", "delete", "--all")
	if len(a.app.Aliases()) != 0 {
		t.Fatalf("after --all: %v", a.app.Aliases())
	}
	if _, err := a.try("alias", "delete", "x", "--all"); err == nil {
		t.Fatal("delete NAME --all succeeded")
	}
	a.run(t, "alias", "list")
	if a.errOut.String() != "no aliases configured\n" {
		t.Fatalf("empty list: %q", a.errOut.String())
	}
}

// Setting an alias that would shadow a command, the app's or an
// extension's, is an error, and so are shell aliases.
func TestAliasSetRefusals(t *testing.T) {
	a := newTestApp(t)
	for _, name := range []string{"pr", "alias", "help", "config", "version", "autocomplete", "PR"} {
		if _, err := a.try("alias", "set", name, "pr checkout"); err == nil || !strings.Contains(err.Error(), "would shadow the command") {
			t.Errorf("alias set %s: %v", name, err)
		}
	}
	for args, want := range map[string][]string{
		"shell aliases":                  {"alias", "set", "x", "!echo hi"},
		"unknown flag: --shell":          {"alias", "set", "--shell", "x", "echo hi"},
		"unknown flag: -s":               {"alias", "set", "-s", "x", "echo hi"},
		"must start with a command":      {"alias", "set", "x", "nosuch thing"},
		"unexpected arguments: checkout": {"alias", "set", "x", "pr", "checkout"},
		"invalid alias name":             {"alias", "set", "a.b", "pr checkout"},
	} {
		if _, err := a.try(want...); err == nil || !strings.Contains(err.Error(), args) {
			t.Errorf("%q: %v, want %q", want, err, args)
		}
	}
	if data, err := os.ReadFile(a.path); err == nil {
		t.Fatalf("a refused alias was saved:\n%s", data)
	}
}

// An alias that a command added later shadows never runs; list says so.
func TestAliasShadowedLater(t *testing.T) {
	a := newTestApp(t)
	a.run(t, "alias", "set", "deploy", "pr checkout")
	ran := false
	a.app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandRun(func(*clix.Context) error { ran = true; return nil })))
	a.run(t, "deploy")
	if !ran || a.ran != "" {
		t.Fatalf("deploy ran the alias: command ran %v, alias ran %q", ran, a.ran)
	}
	a.run(t, "alias", "list")
	if !strings.Contains(a.errOut.String(), `warning: alias deploy never runs: the command "deploy" has its name`) {
		t.Fatalf("list: %q", a.errOut.String())
	}
	if help := a.run(t, "--help"); strings.Contains(help, "ALIAS COMMANDS") {
		t.Fatalf("help lists a shadowed alias:\n%s", help)
	}
}

// Root help lists the aliases; "help" and completion know them.
func TestAliasHelpAndCompletion(t *testing.T) {
	a := newTestApp(t)
	a.run(t, "alias", "set", "co", "pr checkout")
	a.run(t, "alias", "set", "cof", "pr checkout $1 --force")
	help := a.run(t, "help")
	if !strings.Contains(help, "ALIAS COMMANDS\n  co                   Alias for \"pr checkout\"\n  cof                  Alias for \"pr checkout $1 --force\"\n") {
		t.Fatalf("help:\n%s", help)
	}

	complete := func(cur string, words ...string) []string {
		t.Helper()
		out := a.run(t, append([]string{"__complete", "--cur=" + cur, "--", "probe"}, words...)...)
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		return lines[:len(lines)-1]
	}
	if got := complete("co"); !reflect.DeepEqual(got, []string{"config\tManage CLI configuration", "co\tAlias for \"pr checkout\"", "cof\tAlias for \"pr checkout $1 --force\""}) {
		t.Errorf("co<TAB>: %q", got)
	}
	// After an alias, completion continues as for its expansion.
	if got := complete("", "co"); !reflect.DeepEqual(got, []string{"101", "102"}) {
		t.Errorf("co <TAB>: %q", got)
	}
	if got := complete("--f", "co", "101"); !contains(got, "--force") {
		t.Errorf("co 101 --f<TAB>: %q", got)
	}
	// $1 is not typed yet: nothing to offer.
	if got := complete("", "cof"); len(got) != 0 {
		t.Errorf("cof <TAB>: %q", got)
	}
	// cof 101 is "pr checkout 101 --force": --force is given already.
	if got := complete("--", "cof", "101"); contains(got, "--force") || !contains(got, "--number") {
		t.Errorf("cof 101 --<TAB>: %q", got)
	}
	// Aliases are only the first word.
	if got := complete("co", "--format", "json"); contains(got, "co\tAlias for \"pr checkout\"") {
		t.Errorf("--format json co<TAB> offers an alias: %q", got)
	}
	if got := complete("", "pr"); contains(got, "co\tAlias for \"pr checkout\"") {
		t.Errorf("pr <TAB> offers an alias: %q", got)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// The alias commands refuse to rewrite a config file that does not load,
// and keep working when a value in it is bad.
func TestAliasCommandsAndBrokenConfig(t *testing.T) {
	a := newTestApp(t)
	if err := os.WriteFile(a.path, []byte("aliases: [unclosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"alias", "set", "co", "pr checkout"}, {"alias", "list"}, {"alias", "delete", "co"}} {
		if _, err := a.try(args...); err == nil || !strings.Contains(err.Error(), "fix or reset the config file") {
			t.Errorf("%q: %v", args, err)
		}
	}
	if data, _ := os.ReadFile(a.path); string(data) != "aliases: [unclosed\n" {
		t.Fatalf("file rewritten: %q", data)
	}
	if err := os.WriteFile(a.path, []byte("format: xml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a2 := newTestApp(t)
	a2.app.ConfigPath = a.path
	a2.path = a.path
	a2.run(t, "alias", "set", "co", "pr checkout")
	if !strings.Contains(a2.errOut.String(), "ignoring invalid value") {
		t.Errorf("no warning for format: xml: %q", a2.errOut.String())
	}
}

// An app command named "alias" replaces the extension's; expansion stays
// on.
func TestAppAliasCommandWins(t *testing.T) {
	a := newTestApp(t)
	if err := os.WriteFile(a.path, []byte("aliases:\n  co: pr checkout\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mine := false
	a.app.Root.AddCommand(clix.NewCommand("alias", clix.WithCommandRun(func(*clix.Context) error { mine = true; return nil })))
	a.run(t, "alias")
	if !mine {
		t.Fatal("the app's alias command did not run")
	}
	a.run(t, "co", "1")
	if a.ran != "probe pr checkout" {
		t.Fatalf("co 1: ran %q", a.ran)
	}
	if err := a.app.Validate(); err == nil || !strings.Contains(err.Error(), `"alias"`) {
		// The extension adds its command only when the app has none, so
		// there is nothing to report.
		if err != nil {
			t.Fatalf("Validate: %v", err)
		}
	}
}

func TestValidate(t *testing.T) {
	a := newTestApp(t)
	if err := a.app.Validate(); err != nil {
		t.Fatal(err)
	}
}
