package format_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// The root --format/-f flag must not shadow a command's own -f.
func TestFormatShortDoesNotShadowCommandFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	t.Run("bool -f", func(t *testing.T) {
		app := clix.NewApp("myapp", clix.WithAppOut(&bytes.Buffer{}))
		app.AddExtension(format.Extension{})
		var force bool
		cmd := clix.NewCommand("delete", clix.WithCommandRun(func(*clix.Context) error { return nil }))
		cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force", Short: "f"}, Value: &force})
		app.Root.AddCommand(cmd)

		if err := app.Run(context.Background(), []string{"delete", "-f"}); err != nil {
			t.Fatalf("Run error: %v", err)
		}
		if !force {
			t.Fatal("delete -f did not set --force")
		}
	})

	t.Run("string -f", func(t *testing.T) {
		app := clix.NewApp("myapp", clix.WithAppOut(&bytes.Buffer{}))
		app.AddExtension(format.Extension{})
		var file string
		cmd := clix.NewCommand("apply", clix.WithCommandRun(func(*clix.Context) error { return nil }))
		cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "file", Short: "f"}, Value: &file})
		app.Root.AddCommand(cmd)

		if err := app.Run(context.Background(), []string{"apply", "-f", "x.yaml", "--format", "json"}); err != nil {
			t.Fatalf("Run error: %v", err)
		}
		if file != "x.yaml" {
			t.Fatalf("file = %q, want x.yaml", file)
		}
		if got := format.OutputFormat(app); got != clix.FormatJSON {
			t.Fatalf("format = %q, want json", got)
		}
	})
}

// shortApp is an app with the format extension e and an "api" command
// whose -f is --raw-field, as in gh's api command.
func shortApp(t *testing.T, e format.Extension) (*clix.App, *string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MYAPP_FORMAT", "")
	app := clix.NewApp("myapp", clix.WithAppOut(&bytes.Buffer{}), clix.WithAppErr(&bytes.Buffer{}))
	app.AddExtension(e)
	raw := new(string)
	api := clix.NewCommand("api", clix.WithCommandRun(func(*clix.Context) error { return nil }))
	api.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "raw-field", Short: "f"}, Value: raw})
	app.Root.AddCommand(api)
	return app, raw
}

func shortOf(app *clix.App, name string) string {
	if f := app.Flags().Lookup(name); f != nil {
		return f.Short
	}
	return "missing"
}

func TestFormatShort(t *testing.T) {
	t.Run("default -f", func(t *testing.T) {
		app, raw := shortApp(t, format.Extension{})
		if err := app.Run(context.Background(), []string{"-f", "json", "api", "-f", "k=v"}); err != nil {
			t.Fatal(err)
		}
		if format.OutputFormat(app) != clix.FormatJSON || *raw != "k=v" {
			t.Errorf("format %q, raw-field %q", format.OutputFormat(app), *raw)
		}
		// Validate reports the two meanings of -f, as before.
		if err := app.Validate(); err == nil || !strings.Contains(err.Error(), "-f is also the short name of --format") {
			t.Errorf("Validate = %v", err)
		}
	})

	t.Run("NoShort", func(t *testing.T) {
		app, raw := shortApp(t, format.Extension{Projections: true, FormatShort: format.NoShort})
		if err := app.Validate(); err != nil {
			t.Errorf("Validate = %v", err)
		}
		if err := app.Run(context.Background(), []string{"--format", "json", "api", "-f", "k=v"}); err != nil {
			t.Fatal(err)
		}
		if format.OutputFormat(app) != clix.FormatJSON || *raw != "k=v" {
			t.Errorf("format %q, raw-field %q", format.OutputFormat(app), *raw)
		}
		if got := [3]string{shortOf(app, "format"), shortOf(app, "jq"), shortOf(app, "template")}; got != [3]string{"", "q", "t"} {
			t.Errorf("short names %q", got)
		}
		if err := app.Run(context.Background(), []string{"-f", "json", "api"}); err == nil {
			t.Error("-f before the command name is still a flag")
		}
	})

	t.Run("another letter", func(t *testing.T) {
		app, _ := shortApp(t, format.Extension{FormatShort: "o"})
		if err := app.Validate(); err != nil {
			t.Errorf("Validate = %v", err)
		}
		if err := app.Run(context.Background(), []string{"-o", "yaml", "api"}); err != nil {
			t.Fatal(err)
		}
		if format.OutputFormat(app) != clix.FormatYAML {
			t.Errorf("format %q", format.OutputFormat(app))
		}
	})

	// --format takes a letter --jq or --template would use, and that flag
	// keeps its long name only, without a conflict to report.
	for _, tc := range []struct{ short, loser string }{{"q", "jq"}, {"t", "template"}} {
		t.Run("-"+tc.short, func(t *testing.T) {
			app, _ := shortApp(t, format.Extension{FormatShort: tc.short})
			if err := app.Validate(); err != nil {
				t.Errorf("Validate = %v", err)
			}
			if shortOf(app, "format") != tc.short || shortOf(app, tc.loser) != "" {
				t.Errorf("--format -%s, --%s -%s", shortOf(app, "format"), tc.loser, shortOf(app, tc.loser))
			}
		})
	}

	// When the app has its own --format, the extension registers none, so
	// --jq and --template keep -q and -t whatever FormatShort says.
	t.Run("app's own --format", func(t *testing.T) {
		app := clix.NewApp("myapp", clix.WithAppOut(&bytes.Buffer{}), clix.WithAppErr(&bytes.Buffer{}))
		var own string
		app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "format", Short: "o"}, Value: &own})
		app.AddExtension(format.Extension{FormatShort: "q"})
		app.AddExtension(format.Extension{FormatShort: "t"}) // registers nothing more
		app.Root.AddCommand(clix.NewCommand("run", clix.WithCommandRun(func(*clix.Context) error { return nil })))
		// Validate reports the replaced --format, and nothing about -q or -t.
		if err := app.Validate(); err == nil || strings.Contains(err.Error(), "-q") || strings.Contains(err.Error(), "-t") {
			t.Errorf("Validate = %v", err)
		}
		if got := [3]string{shortOf(app, "format"), shortOf(app, "jq"), shortOf(app, "template")}; got != [3]string{"o", "q", "t"} {
			t.Errorf("short names %q", got)
		}
	})
}
