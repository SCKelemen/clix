package clix

import (
	"context"
	"strings"
	"testing"
)

func TestBoolReadsParsedValue(t *testing.T) {
	fs := NewFlagSet("test")
	fs.BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "verbose"}}) // no destination pointer

	if v, ok := fs.Bool("verbose"); v || !ok {
		t.Fatalf("before parse: Bool = %v, %v; want false, true", v, ok)
	}
	if _, err := fs.Parse([]string{"--verbose=false"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if v, _ := fs.Bool("verbose"); v {
		t.Fatal("--verbose=false read back as true")
	}
	if _, err := fs.Parse([]string{"--verbose"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if v, _ := fs.Bool("verbose"); !v {
		t.Fatal("--verbose read back as false")
	}
}

func TestHelpFalseRunsCommand(t *testing.T) {
	app, out, _ := newIsolatedApp(t, "moln")
	ran := false
	app.Root.AddCommand(NewCommand("deploy", WithCommandRun(func(*Context) error {
		ran = true
		return nil
	})))
	if err := app.Run(context.Background(), []string{"deploy", "--help=false"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !ran || out.Len() != 0 {
		t.Fatalf("ran=%v output=%q; want the command to run without help", ran, out.String())
	}
}

func TestEnvDoesNotHijackHelpOrVersion(t *testing.T) {
	for _, env := range []struct{ name, value string }{
		{"MOLN_HELP", "false"},
		{"MOLN_HELP", "true"},
		{"MOLN_VERSION", "1.2.3"},
		{"MOLN_VERSION", "true"},
	} {
		t.Run(env.name+"="+env.value, func(t *testing.T) {
			app, out, _ := newIsolatedApp(t, "moln")
			t.Setenv(env.name, env.value)
			app.Flags().BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "version", Short: "v"}})
			ran := false
			app.Root.AddCommand(NewCommand("deploy", WithCommandRun(func(*Context) error {
				ran = true
				return nil
			})))
			if err := app.Run(context.Background(), []string{"deploy"}); err != nil {
				t.Fatalf("Run error: %v", err)
			}
			if !ran || out.Len() != 0 {
				t.Fatalf("ran=%v output=%q; want the command to run", ran, out.String())
			}
		})
	}
}

func TestEnvSetErrorNamesSource(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	t.Setenv("MOLN_VERBOSE", "loud")
	cmd := NewCommand("deploy", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "verbose"}})
	app.Root.AddCommand(cmd)

	err := app.Run(context.Background(), []string{"deploy"})
	if err == nil || !strings.Contains(err.Error(), "MOLN_VERBOSE") || !strings.Contains(err.Error(), "--verbose") {
		t.Fatalf("Run error = %v, want an error naming --verbose and MOLN_VERBOSE", err)
	}
	if v, _ := cmd.Flags.Bool("verbose"); v {
		t.Fatal("a rejected environment value must not turn the flag on")
	}
}

func TestEmptyEnvIsUnset(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	t.Setenv("MOLN_VERBOSE", "")
	var verbose bool
	cmd := NewCommand("deploy", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "verbose"}, Value: &verbose})
	app.Root.AddCommand(cmd)
	if err := app.Run(context.Background(), []string{"deploy"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if verbose {
		t.Fatal("empty MOLN_VERBOSE turned the flag on")
	}
}

func TestConfigSetErrorNamesSource(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	app.configLoaded = true
	app.Config.Set("port", "eighty")
	var port int
	cmd := NewCommand("serve", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.IntVar(IntVarOptions{FlagOptions: FlagOptions{Name: "port"}, Value: &port})
	app.Root.AddCommand(cmd)

	err := app.Run(context.Background(), []string{"serve"})
	if err == nil || !strings.Contains(err.Error(), "config") || !strings.Contains(err.Error(), "--port") {
		t.Fatalf("Run error = %v, want an error naming --port and the config file", err)
	}
}
