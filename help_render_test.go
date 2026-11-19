package clix

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// renderHelp runs "args... --help" and returns the output.
func renderHelp(t *testing.T, app *App, args ...string) string {
	t.Helper()
	out := &bytes.Buffer{}
	app.Out = out
	if err := app.Run(context.Background(), append(args, "--help")); err != nil {
		t.Fatalf("%q --help: %v", args, err)
	}
	return out.String()
}

func newHelpTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return NewApp("probe", WithAppErr(&bytes.Buffer{}))
}

func TestHelpRendersVariadicArgument(t *testing.T) {
	app := newHelpTestApp(t)
	add := NewCommand("add", WithCommandRun(func(*Context) error { return nil }))
	var dest string
	add.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "dest", Usage: "Destination", Positional: true, Required: true}, Value: &dest})
	add.Flags.ArgsVar(ArgsVarOptions{Name: "path", Usage: "Files to add"})
	app.Root.AddCommand(add)

	out := renderHelp(t, app, "add")
	if !strings.Contains(out, "probe add [flags] <dest> [path...]") {
		t.Errorf("usage line should mark the variadic argument:\n%s", out)
	}
	if !strings.Contains(out, "  <path>...") || !strings.Contains(out, "  <dest> ") {
		t.Errorf("ARGUMENTS should mark the variadic argument:\n%s", out)
	}
}
