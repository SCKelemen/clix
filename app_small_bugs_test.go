package clix_test

import (
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// For a key that is not a flag, an empty environment variable hid the
// config file, while for flags it counts as unset.
func TestEmptyEnvDoesNotHideConfigForOtherKeys(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	writeConfigFile(t, app, "region: eu\n")
	t.Setenv("MYAPP_REGION", "")
	var got string
	var src clix.Source
	app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(func(ctx *clix.Context) error {
		got, src, _ = ctx.EffectiveString("region")
		return nil
	})))
	if err := app.Run(bg, []string{"ls"}); err != nil {
		t.Fatal(err)
	}
	if got != "eu" || src != clix.SourceConfigFile {
		t.Fatalf("EffectiveString(region) = %q from %v, want eu from the config file", got, src)
	}
}

// A bad value from the environment or the config file exited 1, but the
// same value on the command line exited 2.
func TestBadEnvAndConfigValuesAreUsageErrors(t *testing.T) {
	newServe := func(t *testing.T) *clix.App {
		app, _ := newTestApp(t, "myapp")
		cmd := clix.NewCommand("serve", clix.WithCommandRun(noop))
		cmd.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "port"}})
		app.Root.AddCommand(cmd)
		return app
	}
	if code := clix.ExitCode(newServe(t).Run(bg, []string{"serve", "--port", "x"})); code != 2 {
		t.Fatalf("command line: exit %d", code)
	}

	app := newServe(t)
	t.Setenv("MYAPP_PORT", "x")
	err := app.Run(bg, []string{"serve"})
	if code := clix.ExitCode(err); code != 2 || !strings.Contains(err.Error(), "MYAPP_PORT") {
		t.Fatalf("environment: %v, exit %d", err, code)
	}
	t.Setenv("MYAPP_PORT", "")

	app = newServe(t)
	writeConfigFile(t, app, "port: x\n")
	err = app.Run(bg, []string{"serve"})
	if code := clix.ExitCode(err); code != 2 || !strings.Contains(err.Error(), `"port"`) {
		t.Fatalf("config: %v, exit %d", err, code)
	}
}

type versionLikeExtension struct{}

func (versionLikeExtension) Extend(app *clix.App) error {
	c := clix.NewCommand("version", clix.WithCommandRun(noop))
	c.IsExtensionCommand = true
	app.Root.AddCommand(c)
	return nil
}

// A root with no Run and only extension commands errored on a bare
// invocation instead of showing help.
func TestExtensionOnlyRootShowsHelp(t *testing.T) {
	app, out := newTestApp(t, "myapp")
	app.AddExtension(versionLikeExtension{})
	if err := app.Run(bg, []string{}); err != nil {
		t.Fatalf("bare myapp = %v (exit %d)", err, clix.ExitCode(err))
	}
	if !strings.Contains(out.String(), "USAGE") || !strings.Contains(out.String(), "version") {
		t.Fatalf("no help printed:\n%s", out)
	}
}
