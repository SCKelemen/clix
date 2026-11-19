package clix

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEffectiveStringReportsActualSource(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "myapp")
	app.configLoaded = true
	app.Config.Set("zone", "from-config")
	t.Setenv("MYAPP_PROJECT", "from-env")

	sources := map[string]Source{}
	cmd := NewCommand("ls", WithCommandRun(func(ctx *Context) error {
		for _, key := range []string{"project", "zone", "region", "name", "global"} {
			_, src, ok := ctx.EffectiveString(key)
			if !ok {
				return errors.New("missing " + key)
			}
			sources[key] = src
		}
		return nil
	}))
	var project, zone, region, name, global string
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "project"}, Value: &project})
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "zone"}, Value: &zone})
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "region"}, Default: "us", Value: &region})
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "name"}, Value: &name})
	app.Flags().StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "global"}, Value: &global})
	app.Root.AddCommand(cmd)

	if err := app.Run(context.Background(), []string{"ls", "--name", "n", "--global", "g"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	want := map[string]Source{
		"project": SourceEnvVar,
		"zone":    SourceConfigFile,
		"region":  SourceDefault,
		"name":    SourceCommandFlag,
		"global":  SourceAppFlag,
	}
	for key, w := range want {
		if sources[key] != w {
			t.Errorf("source of %s = %v, want %v", key, sources[key], w)
		}
	}
}

func TestPromptedValueReportsPromptSource(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "myapp")
	var src Source
	var name string
	cmd := NewCommand("mk", WithCommandRun(func(ctx *Context) error {
		_, src, _ = ctx.EffectiveString("name")
		return nil
	}))
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "name", Required: true}, Value: &name})
	app.Root.AddCommand(cmd)
	app.Interactive = InteractiveAlways
	app.Prompter = prompterFunc(func(context.Context, ...PromptOption) (string, error) { return "typed", nil })

	if err := app.Run(context.Background(), []string{"mk"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if name != "typed" || src != SourcePrompt {
		t.Fatalf("name=%q source=%v, want typed from prompt", name, src)
	}
}

func TestEnvAndConfigValuesAreValidated(t *testing.T) {
	noSpaces := func(s string) error {
		if strings.Contains(s, " ") {
			return errors.New("must not contain spaces")
		}
		return nil
	}
	newCmd := func(app *App) {
		var zone string
		cmd := NewCommand("ls", WithCommandRun(func(*Context) error { return nil }))
		cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "zone", Validate: noSpaces}, Value: &zone})
		app.Root.AddCommand(cmd)
	}

	t.Run("env", func(t *testing.T) {
		app, _, _ := newIsolatedApp(t, "myapp")
		t.Setenv("MYAPP_ZONE", "bad zone")
		newCmd(app)
		err := app.Run(context.Background(), []string{"ls"})
		if err == nil || !strings.Contains(err.Error(), "MYAPP_ZONE") || !strings.Contains(err.Error(), "must not contain spaces") {
			t.Fatalf("Run error = %v, want a validation error naming MYAPP_ZONE", err)
		}
	})

	t.Run("config", func(t *testing.T) {
		app, _, _ := newIsolatedApp(t, "myapp")
		app.configLoaded = true
		app.Config.Set("zone", "bad zone")
		newCmd(app)
		err := app.Run(context.Background(), []string{"ls"})
		if err == nil || !strings.Contains(err.Error(), "config") || !strings.Contains(err.Error(), "must not contain spaces") {
			t.Fatalf("Run error = %v, want a validation error naming the config file", err)
		}
	})

	t.Run("bad int from env", func(t *testing.T) {
		app, _, _ := newIsolatedApp(t, "myapp")
		t.Setenv("MYAPP_PORT", "notanint")
		var port int
		cmd := NewCommand("serve", WithCommandRun(func(*Context) error { return nil }))
		cmd.Flags.IntVar(IntVarOptions{FlagOptions: FlagOptions{Name: "port"}, Default: "8080", Value: &port})
		app.Root.AddCommand(cmd)
		err := app.Run(context.Background(), []string{"serve"})
		if err == nil || !strings.Contains(err.Error(), "MYAPP_PORT") {
			t.Fatalf("Run error = %v, want an error naming MYAPP_PORT", err)
		}
	})

	t.Run("command line overrides a bad env value", func(t *testing.T) {
		app, _, _ := newIsolatedApp(t, "myapp")
		t.Setenv("MYAPP_ZONE", "bad zone")
		newCmd(app)
		if err := app.Run(context.Background(), []string{"ls", "--zone", "good"}); err != nil {
			t.Fatalf("Run error: %v", err)
		}
	})
}
