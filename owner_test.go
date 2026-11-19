package clix_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/autocomplete"
	"github.com/SCKelemen/clix/v2/ext/config"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/help"
	"github.com/SCKelemen/clix/v2/ext/version"
)

// extensionFunc is an Extension made from a function.
type extensionFunc func(*clix.App) error

func (f extensionFunc) Extend(app *clix.App) error { return f(app) }

// An extension that adds --project to the app's deploy used to win for
// "deploy --project x", leaving the app's root --project empty: ownership
// was only compared within one flag set.
func TestExtensionFlagOnAppCommandDoesNotShadowTheApp(t *testing.T) {
	app, _ := newTestApp(t, "o")
	var project, extProject string
	var src clix.Source
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Short: "p"}, Value: &project})
	deploy := clix.NewCommand("deploy", clix.WithCommandRun(func(ctx *clix.Context) error {
		_, src, _ = ctx.EffectiveString("project")
		return nil
	}))
	app.Root.AddCommand(deploy)
	app.AddExtension(extensionFunc(func(a *clix.App) error {
		deploy.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Short: "p"}, Value: &extProject})
		return nil
	}))
	for _, args := range [][]string{{"deploy", "--project", "x"}, {"deploy", "-p", "x"}, {"--project", "x", "deploy"}} {
		project, extProject = "", ""
		if err := app.Run(bg, args); err != nil || project != "x" || extProject != "" || src != clix.SourceAppFlag {
			t.Fatalf("Run(%q) = %v: app's project %q, extension's %q, source %v", args, err, project, extProject, src)
		}
	}
	assertValidate(t, app,
		`o deploy: clix: extension #1 (clix_test.extensionFunc): flag --project is also defined by the app on "o"; the app's --project is used here too, and this one is ignored`,
	)
}

// An extension's flag that loses its name to the app's root flag (see
// above) is ignored, as Validate says: it was still required, so "deploy
// --token x" set the root's --token and then failed with "missing
// required flags: --token", and it was still filled, and validated, from
// the environment and the config file, and listed in help.
func TestShadowedExtensionFlagIsIgnored(t *testing.T) {
	app, out := newTestApp(t, "lx")
	writeConfigFile(t, app, "token: from-file\n")
	var token, extToken string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "token"}, Value: &token})
	ran := false
	app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandRun(mark(&ran))))
	app.AddExtension(extensionFunc(func(a *clix.App) error {
		a.Root.ResolvePath([]string{"deploy"}).Flags.StringVar(clix.StringVarOptions{
			FlagOptions: clix.FlagOptions{Name: "token", Required: true, Validate: func(string) error { return errors.New("rejected") }},
			Value:       &extToken,
		})
		return nil
	}))
	if err := app.Run(bg, []string{"deploy", "--token", "x"}); err != nil || !ran || token != "x" || extToken != "" {
		t.Fatalf("deploy --token x: %v, ran=%v, root token %q, extension's %q", err, ran, token, extToken)
	}
	t.Setenv("LX_TOKEN", "from-env")
	ran = false
	if err := app.Run(bg, []string{"deploy"}); err != nil || !ran || token != "from-env" || extToken != "" {
		t.Fatalf("deploy with LX_TOKEN: %v, ran=%v, root token %q, extension's %q", err, ran, token, extToken)
	}
	t.Setenv("LX_TOKEN", "")
	if err := app.Run(bg, []string{"deploy"}); err != nil || token != "from-file" || extToken != "" {
		t.Fatalf("deploy with a config file: %v, root token %q, extension's %q", err, token, extToken)
	}
	out.Reset()
	if err := app.Run(bg, []string{"deploy", "--help"}); err != nil || strings.Contains(out.String(), "--token") {
		t.Fatalf("deploy --help = %v, lists the ignored --token:\n%s", err, out)
	}
	assertValidate(t, app,
		`lx deploy: clix: extension #1 (clix_test.extensionFunc): flag --token is also defined by the app on "lx"; the app's --token is used here too, and this one is ignored`,
	)
}

// Within its own command, an extension's flags win over the app's
// ancestor flags of the same name, and that is not reported: the app
// could not fix it.
func TestExtensionCommandKeepsItsOwnFlags(t *testing.T) {
	app, _ := newTestApp(t, "o")
	var rootKey, extKey string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "key", Short: "k"}, Value: &rootKey})
	app.AddExtension(extensionFunc(func(a *clix.App) error {
		c := clix.NewCommand("vault", clix.WithCommandRun(noop))
		c.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "key", Short: "k"}, Value: &extKey})
		a.Root.AddCommand(c)
		return nil
	}))
	if err := app.Run(bg, []string{"--key", "root", "vault", "-k", "ext"}); err != nil || rootKey != "root" || extKey != "ext" {
		t.Fatalf("err = %v, root key %q, vault key %q", err, rootKey, extKey)
	}
	assertValidate(t, app)
}

// The shipped extensions define --key, --value, --shell and --command on
// their own commands; an app with ordinary root flags of those names used
// to get Validate errors it could not fix.
func TestValidateIsQuietWithShippedExtensions(t *testing.T) {
	for _, name := range []string{"key", "value", "shell", "command", "format", "output", "config", "verbose", "force", "project"} {
		t.Run(name, func(t *testing.T) {
			app, out := newTestApp(t, "o")
			app.ConfigPath = t.TempDir() + "/c.yaml"
			app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: name, Usage: "Mine"}})
			app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandRun(noop)))
			app.AddExtension(help.Extension{})
			app.AddExtension(version.Extension{Version: "1"})
			app.AddExtension(config.Extension{})
			app.AddExtension(autocomplete.Extension{})
			app.AddExtension(format.Extension{})
			if name == "format" {
				// A real override the app can fix (it defines --format and
				// also asks ext/format for one): reported.
				assertValidate(t, app, `o: clix: extension #5 (format.Extension): flag --format in flag set "o" is also defined by the app; the app's --format is used and this one is ignored`)
			} else {
				assertValidate(t, app)
			}
			for _, args := range [][]string{{"config", "set", "region", "eu"}, {"config", "get", "region"}, {"autocomplete", "--shell", "bash"}, {"help", "--command", "deploy"}} {
				out.Reset()
				if err := app.Run(bg, args); err != nil {
					t.Fatalf("Run(%q) = %v", args, err)
				}
			}
		})
	}
}

// A later extension's command registered --quick -q in its PreRun, which
// took -q from an earlier extension's --quiet: after "b" ran once, "work
// -q" changed meaning, and Validate called it the app's flag.
func TestFlagsRegisteredDuringRunCannotTakeNames(t *testing.T) {
	app, _ := newTestApp(t, "o")
	var quiet, quick bool
	app.AddExtension(extensionFunc(func(a *clix.App) error {
		a.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "quiet", Short: "q"}, Value: &quiet})
		return nil
	}))
	app.AddExtension(extensionFunc(func(a *clix.App) error {
		b := clix.NewCommand("b", clix.WithCommandRun(noop))
		b.PreRun = func(ctx *clix.Context) error {
			if ctx.App.Flags().Lookup("quick") == nil {
				ctx.App.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "quick", Short: "q"}, Value: &quick})
			}
			return nil
		}
		a.Root.AddCommand(b)
		return nil
	}))
	app.Root.AddCommand(clix.NewCommand("work", clix.WithCommandRun(noop)))
	for _, args := range [][]string{{"work", "-q"}, {"b"}, {"work", "-q"}, {"b"}, {"work", "-q"}} {
		quiet, quick = false, false
		if err := app.Run(bg, args); err != nil {
			t.Fatal(err)
		}
		if args[0] == "work" && (!quiet || quick) {
			t.Fatalf("Run(%q): quiet = %v, quick = %v", args, quiet, quick)
		}
	}
	assertValidate(t, app, `o: clix: a flag registered during Run: -q is used by extension #1 (clix_test.extensionFunc)'s --quiet; quick is available as --quick only`)
}

// A Run started by a handler ended the outer Run's running state, so a
// flag the outer handler registered after it counted as the app's and
// took -v from ext/version.
func TestFlagsRegisteredAfterNestedRunCannotTakeNames(t *testing.T) {
	for _, nested := range []bool{false, true} {
		app, _ := newTestApp(t, "o")
		app.AddExtension(version.Extension{Version: "1.0"})
		var late bool
		sub := clix.NewCommand("sub", clix.WithCommandRun(func(ctx *clix.Context) error {
			if nested {
				if err := app.Run(ctx, []string{"ls"}); err != nil {
					return err
				}
			}
			app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "late", Short: "v"}, Value: &late})
			return nil
		}))
		app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(noop)))
		app.Root.AddCommand(sub)
		if err := app.Run(bg, []string{"sub"}); err != nil {
			t.Fatal(err)
		}
		if f := app.Flags().Lookup("version"); f == nil || f.Short != "v" {
			t.Errorf("nested=%v: a flag registered during Run took -v from ext/version", nested)
		}
		if f := app.Flags().Lookup("late"); f == nil || f.Short != "" {
			t.Errorf("nested=%v: --late has -%s", nested, f.Short)
		}
	}
}

// A flag added to an extension's command belongs to that extension,
// whenever it is registered; while another extension's Extend runs, it
// belongs to that one. Two extensions of the same type are told apart by
// their AddExtension index.
func TestFlagsOnExtensionCommandsBelongToTheExtension(t *testing.T) {
	app, _ := newTestApp(t, "o")
	var tools *clix.Command
	var mode, mute string
	app.AddExtension(extensionFunc(func(a *clix.App) error {
		tools = clix.NewCommand("tools", clix.WithCommandRun(noop))
		tools.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "mode", Short: "m"}, Value: &mode})
		a.Root.AddCommand(tools)
		return nil
	}))
	app.AddExtension(extensionFunc(func(a *clix.App) error {
		tools.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "mute", Short: "m"}, Value: &mute})
		return nil
	}))
	if err := app.ApplyExtensions(); err != nil {
		t.Fatal(err)
	}
	tools.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "mine", Short: "m"}})
	if err := app.Run(bg, []string{"tools", "-m", "x"}); err != nil || mode != "x" || mute != "" {
		t.Fatalf("err = %v, mode %q, mute %q", err, mode, mute)
	}
	// Extension #2's --mute keeps its long name, which is not reported;
	// --mine belongs to extension #1, like --mode, so it is.
	assertValidate(t, app,
		`o tools: clix: extension #1 (clix_test.extensionFunc): flag --mine cannot use -m in flag set "tools" because --mode already has it; it is available only as --mine`,
	)
}

// VisibleChildren always folded case, so with CaseSensitive a reachable
// extension command "status" next to the app's "Status" was missing from
// help and completion.
func TestCaseSensitiveCommandsAreVisible(t *testing.T) {
	app, out := newTestApp(t, "o")
	app.CaseSensitive = true
	app.Root.AddCommand(clix.NewCommand("Status", clix.WithCommandShort("App status"), clix.WithCommandRun(noop)))
	app.AddExtension(autocomplete.Extension{})
	app.AddExtension(extensionFunc(func(a *clix.App) error {
		a.Root.AddCommand(clix.NewCommand("status", clix.WithCommandShort("Extension status"), clix.WithCommandRun(noop)))
		return nil
	}))
	if err := app.Run(bg, []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "App status") || !strings.Contains(out.String(), "Extension status") {
		t.Fatalf("help:\n%s", out)
	}
	var script bytes.Buffer
	app.Out = &script
	if err := app.Run(bg, []string{"autocomplete", "--shell", "bash"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script.String(), "Status") || !strings.Contains(script.String(), " status") {
		t.Fatalf("completion:\n%s", script.String())
	}
	assertValidate(t, app)
}

// Replacing Root after ApplyExtensions moved the extensions' flags but not
// their commands: --version worked and version was unknown.
func TestReplacedRootKeepsExtensionCommands(t *testing.T) {
	app, out := newTestApp(t, "o")
	app.AddExtension(version.Extension{Version: "9"})
	if err := app.ApplyExtensions(); err != nil {
		t.Fatal(err)
	}
	ran := false
	app.Root = clix.NewGroup("o", "", clix.NewCommand("deploy", clix.WithCommandRun(mark(&ran))))
	for _, args := range [][]string{{"version"}, {"--version"}} {
		out.Reset()
		if err := app.Run(bg, args); err != nil || !strings.Contains(out.String(), "9") {
			t.Fatalf("Run(%q) = %v, out %q", args, err, out)
		}
	}
	if err := app.Run(bg, []string{"deploy"}); err != nil || !ran {
		t.Fatalf("deploy: %v", err)
	}
	assertValidate(t, app)
}
