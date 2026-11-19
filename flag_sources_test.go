package clix_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// newForceApp builds "myapp delete" with a --force switch configured by fo.
func newForceApp(t *testing.T, fo clix.FlagOptions) (*clix.App, *bool, *clix.Source) {
	app, _ := newTestApp(t, "myapp")
	force, src := new(bool), new(clix.Source)
	fo.Name = "force"
	cmd := clix.NewCommand("delete", clix.WithCommandRun(func(ctx *clix.Context) error {
		_, *src, _ = ctx.EffectiveBool("force")
		return nil
	}))
	cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: fo, Value: force})
	app.Root.AddCommand(cmd)
	return app, force, src
}

// MYAPP_FORCE=1 or "force: true" turned on "delete --force", with no way to
// opt out.
func TestCommandLineOnlyIgnoresEnvAndConfig(t *testing.T) {
	for _, src := range []string{"env", "config"} {
		t.Run(src, func(t *testing.T) {
			app, force, _ := newForceApp(t, clix.FlagOptions{CommandLineOnly: true, EnvVar: "FORCE"})
			if src == "env" {
				t.Setenv("MYAPP_FORCE", "1")
				t.Setenv("FORCE", "1")
			} else {
				writeConfigFile(t, app, "force: true\n")
			}
			if err := app.Run(bg, []string{"delete"}); err != nil {
				t.Fatal(err)
			}
			if *force {
				t.Fatalf("delete ran with --force=true from %s", src)
			}
			if err := app.Run(bg, []string{"delete", "--force"}); err != nil || !*force {
				t.Fatalf("--force on the command line: %v, force = %v", err, *force)
			}
		})
	}
}

func TestNoEnvAndNoConfig(t *testing.T) {
	t.Run("NoEnv", func(t *testing.T) {
		app, force, src := newForceApp(t, clix.FlagOptions{NoEnv: true, EnvVar: "FORCE"})
		t.Setenv("MYAPP_FORCE", "1")
		t.Setenv("FORCE", "1")
		writeConfigFile(t, app, "force: true\n")
		if err := app.Run(bg, []string{"delete"}); err != nil || !*force || *src != clix.SourceConfigFile {
			t.Fatalf("err = %v, force = %v from %v; want true from the config file", err, *force, *src)
		}
	})
	t.Run("NoConfig", func(t *testing.T) {
		app, force, _ := newForceApp(t, clix.FlagOptions{NoConfig: true})
		writeConfigFile(t, app, "force: true\n")
		if err := app.Run(bg, []string{"delete"}); err != nil || *force {
			t.Fatalf("err = %v, force = %v", err, *force)
		}
		t.Setenv("MYAPP_FORCE", "true")
		if err := app.Run(bg, []string{"delete"}); err != nil || !*force {
			t.Fatalf("env: err = %v, force = %v", err, *force)
		}
	})
}

// One bare config key fed every command's --force; ConfigKey scopes it.
func TestConfigKeyScopesConfigValues(t *testing.T) {
	app, force, _ := newForceApp(t, clix.FlagOptions{ConfigKey: "delete.force"})
	writeConfigFile(t, app, "force: true\n")
	if err := app.Run(bg, []string{"delete"}); err != nil || *force {
		t.Fatalf("bare key: err = %v, force = %v", err, *force)
	}

	app, force, src := newForceApp(t, clix.FlagOptions{ConfigKey: "delete.force"})
	writeConfigFile(t, app, "delete:\n  force: true\n")
	if err := app.Run(bg, []string{"delete"}); err != nil || !*force || *src != clix.SourceConfigFile {
		t.Fatalf("delete.force: err = %v, force = %v from %v", err, *force, *src)
	}

	// A bad value names the key it came from.
	app, _, _ = newForceApp(t, clix.FlagOptions{ConfigKey: "delete.force"})
	writeConfigFile(t, app, "delete:\n  force: maybe\n")
	if err := app.Run(bg, []string{"delete"}); err == nil || !strings.Contains(err.Error(), `"delete.force"`) {
		t.Fatalf("err = %v, want it to name delete.force", err)
	}
}

func TestFunctionalSourceOptions(t *testing.T) {
	fs := clix.NewFlagSet("x")
	fs.BoolVar(clix.WithFlagName("a"), clix.WithFlagCommandLineOnly())
	fs.BoolVar(clix.WithFlagName("b"), clix.WithFlagNoEnv(), clix.WithFlagNoConfig(), clix.WithFlagConfigKey("x.b"))
	a, b := fs.Flags()[0], fs.Flags()[1]
	if !a.CommandLineOnly || !b.NoEnv || !b.NoConfig || b.ConfigKey != "x.b" {
		t.Fatalf("a = %+v, b = %+v", a, b)
	}
}

func TestAutoEnvAndAutoConfigOff(t *testing.T) {
	off := false
	app, _ := newTestApp(t, "myapp")
	app.AutoEnv, app.AutoConfig = &off, &off
	writeConfigFile(t, app, "region: eu\nzone: z1\nnote: hi\n")
	t.Setenv("MYAPP_REGION", "us")
	t.Setenv("MYAPP_ZONE", "z2")
	t.Setenv("ZONE", "z3")
	var region, zone, note, viaCtx string
	cmd := clix.NewCommand("ls", clix.WithCommandRun(func(ctx *clix.Context) error {
		viaCtx, _ = ctx.String("note")
		return nil
	}))
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region"}, Value: &region})
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "zone", EnvVar: "ZONE"}, Value: &zone})
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "note", ConfigKey: "note", Required: true}, Value: &note})
	app.Root.AddCommand(cmd)
	if err := app.Run(bg, []string{"ls"}); err != nil {
		t.Fatal(err)
	}
	if region != "" || zone != "z3" || note != "hi" || viaCtx != "hi" {
		t.Fatalf("region = %q, zone = %q, note = %q (ctx %q)", region, zone, note, viaCtx)
	}

	// MissingFlagsError lists only the variables that are read.
	req := clix.NewCommand("req", clix.WithCommandRun(noop))
	req.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "token", EnvVar: "TOKEN", Required: true}})
	req.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "pin", CommandLineOnly: true, Required: true}})
	app.Root.AddCommand(req)
	err := app.Run(bg, []string{"req"})
	var missing *clix.MissingFlagsError
	if !errors.As(err, &missing) || !reflect.DeepEqual(missing.EnvVars, [][]string{{"TOKEN"}, nil}) {
		t.Fatalf("err = %v (%+v)", err, missing)
	}
}
