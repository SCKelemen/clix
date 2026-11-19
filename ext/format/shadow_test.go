package format_test

import (
	"bytes"
	"context"
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
