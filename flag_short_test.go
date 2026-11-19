package clix_test

import (
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/help"
	"github.com/SCKelemen/clix/v2/ext/version"
)

// Help registered last on a hand-built Command (or a WithAppRoot root)
// took -h from the user's --host.
func TestHelpDoesNotTakeShortH(t *testing.T) {
	newHost := func(name string, host *string, ran *bool) *clix.Command {
		cmd := &clix.Command{Name: name, Flags: clix.NewFlagSet(name), Run: mark(ran)}
		cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "host", Short: "h"}, Value: host})
		return cmd
	}

	t.Run("hand-built command", func(t *testing.T) {
		app, out := newTestApp(t, "srv")
		app.AddExtension(help.Extension{})
		var host string
		ran := false
		app.Root.AddCommand(newHost("serve", &host, &ran))
		if err := app.Run(bg, []string{"serve", "-h", "localhost"}); err != nil || !ran || host != "localhost" {
			t.Fatalf("Run = %v ran=%v host=%q\n%s", err, ran, host, out)
		}
		if err := app.Run(bg, []string{"serve", "--help"}); err != nil || !strings.Contains(out.String(), "USAGE") {
			t.Fatalf("--help = %v\n%s", err, out)
		}
	})

	t.Run("WithAppRoot", func(t *testing.T) {
		var host string
		ran := false
		app, out := newTestApp(t, "srv", clix.WithAppRoot(newHost("srv", &host, &ran)))
		if err := app.Run(bg, []string{"-h", "localhost"}); err != nil || !ran || host != "localhost" {
			t.Fatalf("Run = %v ran=%v host=%q\n%s", err, ran, host, out)
		}
	})

	t.Run("NewCommand", func(t *testing.T) {
		app, _ := newTestApp(t, "srv")
		var host string
		cmd := clix.NewCommand("serve", clix.WithCommandRun(noop))
		cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "host", Short: "h"}, Value: &host})
		app.Root.AddCommand(cmd)
		if err := app.Run(bg, []string{"serve", "-h", "example.com"}); err != nil || host != "example.com" {
			t.Fatalf("Run = %v host=%q", err, host)
		}
		if err := cmd.Flags.Err(); err != nil {
			t.Fatalf("help giving up -h is not an error: %v", err)
		}
	})
}

// Extensions applied at Run used to take short names from the app's own
// flags: version's -v from --verbose and format's -f from --force.
func TestExtensionsDoNotTakeShortNames(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	app.AddExtension(version.Extension{Version: "1.2.3"})
	app.AddExtension(format.Extension{})
	var verbose, force bool
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Short: "v"}, Value: &verbose})
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force", Short: "f"}, Value: &force})
	ran := false
	app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(mark(&ran))))

	if err := app.Run(bg, []string{"-v", "-f", "ls"}); err != nil || !ran || !verbose || !force {
		t.Fatalf("Run(-v -f ls) = %v ran=%v verbose=%v force=%v", err, ran, verbose, force)
	}
	// The extensions' long names still work.
	if err := app.Run(bg, []string{"--format", "json", "ls"}); err != nil {
		t.Fatalf("--format: %v", err)
	}
	if got := format.OutputFormat(app); got != "json" {
		t.Fatalf("format = %q", got)
	}
	err := app.Flags().Err()
	if err == nil || !strings.Contains(err.Error(), "extension version.Extension: -v is used by the app's --verbose; version is available as --version only") || !strings.Contains(err.Error(), "extension format.Extension: -f is used by the app's --force") {
		t.Fatalf("Err() = %v, want both short-name conflicts", err)
	}
}

// Between two user flags, the first keeps the short name.
func TestFirstFlagKeepsShortName(t *testing.T) {
	fs := clix.NewFlagSet("x")
	var a, b string
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "alpha", Short: "a"}, Value: &a})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "also", Short: "a"}, Value: &b})
	if _, err := fs.Parse([]string{"-a", "1", "--also", "2"}); err != nil {
		t.Fatal(err)
	}
	if a != "1" || b != "2" {
		t.Fatalf("alpha=%q also=%q", a, b)
	}
	if err := fs.Err(); err == nil || !strings.Contains(err.Error(), "--also cannot use -a") {
		t.Fatalf("Err() = %v", err)
	}
}
