package clix_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// valueApp has a --format flag that accepts only json or text and a --verbose
// switch, a "repair" command marked with AllowConfigErrors and an ordinary
// "run" command.
func valueApp(t *testing.T, cfg string) (*clix.App, *bytes.Buffer, *string, *bool) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.yaml")
	if cfg != "" {
		if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	errOut := &bytes.Buffer{}
	app := clix.NewApp("demo", clix.WithAppOut(&bytes.Buffer{}), clix.WithAppErr(errOut))
	app.ConfigPath = path
	format, verbose := new(string), new(bool)
	app.Flags().StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{Name: "format", Validate: func(v string) error {
			if v != "json" && v != "text" {
				return errors.New("must be json or text")
			}
			return nil
		}},
		Default: "text",
		Value:   format,
	})
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose"}, Value: verbose})
	repair := clix.NewCommand("repair", clix.WithCommandRun(func(*clix.Context) error { return nil }))
	clix.AllowConfigErrors(repair)
	app.Root.AddCommand(repair)
	app.Root.AddCommand(clix.NewCommand("run", clix.WithCommandRun(func(*clix.Context) error { return nil })))
	return app, errOut, format, verbose
}

// A value that a flag rejects does not stop commands marked with
// AllowConfigErrors: the flag keeps its default and a warning names the
// source. Other commands still fail.
func TestAllowConfigErrorsCoversValueErrors(t *testing.T) {
	cases := map[string]struct {
		cfg, env string
		want     string
	}{
		"config value": {cfg: "format: xml\nverbose: true\n", want: `config file key "format"`},
		"config bool":  {cfg: "verbose: notabool\nformat: json\n", want: `config file key "verbose"`},
		"env value":    {env: "zzz", want: "environment variable DEMO_VERBOSE"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			app, errOut, format, verbose := valueApp(t, c.cfg)
			if c.env != "" {
				t.Setenv("DEMO_VERBOSE", c.env)
			}
			ctx := context.Background()
			if err := app.Run(ctx, []string{"run"}); err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("run: %v", err)
			}
			if err := app.Run(ctx, []string{"repair"}); err != nil {
				t.Fatalf("repair: %v", err)
			}
			if !strings.Contains(errOut.String(), "demo: ignoring invalid value") || !strings.Contains(errOut.String(), c.want) {
				t.Fatalf("warning: %q", errOut.String())
			}
			switch name {
			case "config value":
				// The bad value is undone and the default applies; the other
				// keys still apply.
				if *format != "text" || !*verbose {
					t.Fatalf("format = %q, verbose = %v", *format, *verbose)
				}
			case "config bool":
				if *verbose || *format != "json" {
					t.Fatalf("format = %q, verbose = %v", *format, *verbose)
				}
			}
		})
	}
}

func TestFlagCheckValue(t *testing.T) {
	fs := clix.NewFlagSet("t")
	fs.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose"}})
	fs.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "port", Validate: func(v string) error {
		if v == "0" {
			return errors.New("port 0 is reserved")
		}
		return nil
	}}})
	fs.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "pin", Sensitive: true}})
	verbose, port, pin := fs.Lookup("verbose"), fs.Lookup("port"), fs.Lookup("pin")
	if err := verbose.CheckValue("notabool"); err == nil {
		t.Error("notabool accepted")
	}
	if err := verbose.CheckValue("true"); err != nil {
		t.Error(err)
	}
	if err := port.CheckValue("0"); err == nil || err.Error() != "port 0 is reserved" {
		t.Errorf("Validate not run: %v", err)
	}
	if err := pin.CheckValue("12ab-SECRET"); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Errorf("sensitive value in error: %v", err)
	}
	if v := verbose.Value.String(); v != "false" {
		t.Errorf("CheckValue changed the flag: %q", v)
	}
}

func TestConfigLoadErrorAndFlagConfigKey(t *testing.T) {
	app, _, _, _ := valueApp(t, "format: [unterminated\n")
	if err := app.ConfigLoadError(); err == nil {
		t.Fatal("ConfigLoadError = nil for a malformed file")
	}
	dir := filepath.Join(t.TempDir(), "none")
	app.ConfigPath = filepath.Join(dir, "config.yaml")
	if err := app.ConfigLoadError(); err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("ConfigLoadError created a directory")
	}
	if key, ok := app.FlagConfigKey(app.Flags().Lookup("format")); !ok || key != "format" {
		t.Fatalf("FlagConfigKey = %q, %v", key, ok)
	}
	if _, ok := app.FlagConfigKey(app.Flags().Lookup("help")); ok {
		t.Fatal("--help has a config key")
	}
}
