package version

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func TestVersionShortDoesNotShadowCommandFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("moln", clix.WithAppOut(out))
	app.AddExtension(Extension{Version: "1.0.0"})
	var verbose bool
	ran := false
	cmd := clix.NewCommand("deploy", clix.WithCommandRun(func(*clix.Context) error { ran = true; return nil }))
	cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Short: "v"}, Value: &verbose})
	app.Root.AddCommand(cmd)

	if err := app.Run(context.Background(), []string{"deploy", "-v"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !ran || !verbose || out.Len() != 0 {
		t.Fatalf("ran=%v verbose=%v out=%q; want deploy to run verbosely", ran, verbose, out.String())
	}

	out.Reset()
	if err := app.Run(context.Background(), []string{"-v"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out.String(), "1.0.0") {
		t.Fatalf("-v at the root should print the version, got %q", out.String())
	}
}

func TestVersionEnvDoesNotHijackCommands(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOLN_VERSION", "1.2.3")
	out := &bytes.Buffer{}
	app := clix.NewApp("moln", clix.WithAppOut(out))
	app.AddExtension(Extension{Version: "0.1.0"})
	ran := false
	app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandRun(func(*clix.Context) error { ran = true; return nil })))

	if err := app.Run(context.Background(), []string{"deploy"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !ran || out.Len() != 0 {
		t.Fatalf("ran=%v out=%q; MOLN_VERSION must not print the version", ran, out.String())
	}
}
