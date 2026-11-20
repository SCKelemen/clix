package format_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// quietApp builds "probe get", which prints {"x": 1, "y": "two"} with
// format.Write and records whether --quiet was in effect. order says when
// the format extension is added relative to EnableQuiet.
func quietApp(t *testing.T, opts clix.QuietOptions, extensionFirst bool) (*clix.App, *bytes.Buffer, *bool) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_FORMAT", "")
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
	if extensionFirst {
		app.AddExtension(format.Extension{})
		if err := app.ApplyExtensions(); err != nil {
			t.Fatal(err)
		}
		app.EnableQuiet(opts)
	} else {
		app.EnableQuiet(opts)
		app.AddExtension(format.Extension{})
	}
	quiet := new(bool)
	app.Root.AddCommand(clix.NewCommand("get", clix.WithCommandRun(func(ctx *clix.Context) error {
		*quiet = ctx.App.Quiet()
		return format.Write(ctx, map[string]any{"x": 1, "y": "two"})
	})))
	return app, out, quiet
}

func runQuiet(t *testing.T, app *clix.App, out *bytes.Buffer, args ...string) (string, error) {
	t.Helper()
	out.Reset()
	err := app.Run(context.Background(), args)
	return out.String(), err
}

// By default --quiet has no short name, so -q is --jq, as in gh.
func TestQuietWithoutShortLeavesQToJQ(t *testing.T) {
	for _, extensionFirst := range []bool{false, true} {
		app, out, quiet := quietApp(t, clix.QuietOptions{}, extensionFirst)
		got, err := runQuiet(t, app, out, "get", "-q", ".y")
		if err != nil || got != "two\n" || *quiet {
			t.Fatalf("-q .y: %q, %v, quiet %v; want the jq result", got, err, *quiet)
		}
		got, err = runQuiet(t, app, out, "--quiet", "get", "--jq", ".x")
		if err != nil || got != "1\n" || !*quiet {
			t.Fatalf("--quiet --jq .x: %q, %v, quiet %v", got, err, *quiet)
		}
		if f := app.Flags().Lookup("jq"); f == nil || f.Short != "q" {
			t.Fatalf("--jq lost -q: %+v", f)
		}
		if err := app.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	}
}

// An app that opts in to -q for --quiet owns it, whatever the order of
// EnableQuiet and the format extension: -q never reaches --jq, which keeps
// its long name, and nothing is reported.
func TestQuietShortWinsOverJQ(t *testing.T) {
	for _, extensionFirst := range []bool{false, true} {
		app, out, quiet := quietApp(t, clix.QuietOptions{Short: "q"}, extensionFirst)
		got, err := runQuiet(t, app, out, "get", "-q")
		if err != nil || !*quiet || got != "x = 1\ny = two\n" {
			t.Fatalf("extension first %v: -q: %q, %v, quiet %v; want quiet text output", extensionFirst, got, err, *quiet)
		}
		// "-q .y": -q is the switch, so ".y" is an unexpected argument,
		// never a jq expression.
		if got, err := runQuiet(t, app, out, "get", "-q", ".y"); err == nil || got != "" {
			t.Fatalf("extension first %v: -q .y: %q, %v; want an error", extensionFirst, got, err)
		}
		got, err = runQuiet(t, app, out, "get", "-q", "--jq", ".y")
		if err != nil || got != "two\n" || !*quiet {
			t.Fatalf("extension first %v: -q --jq .y: %q, %v, quiet %v", extensionFirst, got, err, *quiet)
		}
		if f := app.Flags().Lookup("jq"); f == nil || f.Short != "" {
			t.Fatalf("--jq still has a short name: %+v", f)
		}
		if f := app.Flags().Lookup("quiet"); f == nil || f.Short != "q" {
			t.Fatalf("--quiet lacks -q: %+v", f)
		}
		if err := app.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		// Help shows the resolution.
		help, err := runQuiet(t, app, out, "--help")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(help, "-q, --quiet") || strings.Contains(help, "-q, --jq") || !strings.Contains(help, "--jq") {
			t.Fatalf("help:\n%s", help)
		}
	}
}

// Another app flag that wants -q conflicts with --quiet's -q: both are the
// app's, so the first registration keeps it and App.Validate reports it.
func TestQuietShortConflictsWithAppFlag(t *testing.T) {
	app, _, _ := quietApp(t, clix.QuietOptions{Short: "q"}, false)
	var query string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "query", Short: "q"}, Value: &query})
	if err := app.ApplyExtensions(); err != nil {
		t.Fatal(err)
	}
	err := app.Validate()
	if err == nil || !strings.Contains(err.Error(), "flag --query cannot use -q") || !strings.Contains(err.Error(), "--quiet already has it") {
		t.Fatalf("Validate = %v, want the -q conflict", err)
	}
}

// Called inside an extension's Extend, EnableQuiet's flag belongs to that
// extension, so -q follows extension order: the extension added first
// wins.
func TestQuietInExtensionFollowsExtensionOrder(t *testing.T) {
	quietExt := extension(func(app *clix.App) error {
		app.EnableQuiet(clix.QuietOptions{Short: "q"})
		return nil
	})
	for _, tc := range []struct {
		name      string
		exts      []clix.Extension
		quietHasQ bool
	}{
		{"quiet extension first", []clix.Extension{quietExt, format.Extension{}}, true},
		{"format extension first", []clix.Extension{format.Extension{}, quietExt}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			app := clix.NewApp("probe", clix.WithAppOut(&bytes.Buffer{}), clix.WithAppErr(&bytes.Buffer{}))
			for _, e := range tc.exts {
				app.AddExtension(e)
			}
			if err := app.ApplyExtensions(); err != nil {
				t.Fatal(err)
			}
			quiet, jq := app.Flags().Lookup("quiet"), app.Flags().Lookup("jq")
			if quiet == nil || jq == nil {
				t.Fatal("missing --quiet or --jq")
			}
			if got := quiet.Short == "q"; got != tc.quietHasQ {
				t.Fatalf("--quiet has -q: %v, want %v", got, tc.quietHasQ)
			}
			if got := jq.Short == "q"; got == tc.quietHasQ {
				t.Fatalf("--jq has -q: %v, want %v", got, !tc.quietHasQ)
			}
		})
	}
}

type extension func(*clix.App) error

func (f extension) Extend(app *clix.App) error { return f(app) }
