package clix_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/autocomplete"
	"github.com/SCKelemen/clix/v2/ext/config"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/help"
	"github.com/SCKelemen/clix/v2/ext/version"
)

func TestValidateAcceptsACleanApp(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	app.AddExtension(help.Extension{})
	app.AddExtension(version.Extension{Version: "1.0.0"})
	app.AddExtension(config.Extension{})
	app.AddExtension(autocomplete.Extension{})
	app.AddExtension(format.Extension{})
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Short: "p"}})
	ls := clix.NewCommand("ls", clix.WithCommandAliases("list"), clix.WithCommandRun(noop))
	ls.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "all", Short: "a"}})
	app.Root.AddCommand(clix.NewGroup("project", "Projects", ls))
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if err := app.Run(bg, []string{"project", "list", "-a"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateReportsDefinitionMistakes(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	var rootP, cmdP string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project"}, Value: &rootP})

	ls := clix.NewCommand("ls", clix.WithCommandRun(noop))
	// A long name defined twice: the second definition is dropped.
	ls.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "out"}})
	ls.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "out"}})
	// A short name that is already taken.
	ls.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "all", Short: "a"}})
	ls.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "any", Short: "a"}})
	// A default the flag cannot parse.
	ls.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "port"}, Default: "eighty"})
	// The same name as a root flag.
	ls.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project"}, Default: "dflt", Value: &cmdP})
	app.Root.AddCommand(ls)
	// A child name that collides with another child's alias, ignoring case.
	app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandRun(noop)))
	app.Root.AddCommand(clix.NewCommand("ship", clix.WithCommandAliases("DEPLOY"), clix.WithCommandRun(noop)))
	app.Root.Children = append(app.Root.Children, nil)

	err := app.Validate()
	if err == nil {
		t.Fatal("Validate() = nil")
	}
	for _, want := range []string{
		"flag --out redefined",
		"--any cannot use -a",
		`invalid default "eighty" for flag --port`,
		"flag --project is also defined on \"moln\"",
		`"DEPLOY" names both "deploy" and "ship"`,
		"child 3 is nil",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() does not report %q:\n%v", want, err)
		}
	}

	// The shadowing it warns about: the getter and the bound variable
	// disagree when --project comes before the command name.
	var viaCtx string
	ls.Run = func(ctx *clix.Context) error { viaCtx, _ = ctx.String("project"); return nil }
	if err := app.Run(bg, []string{"--project", "p1", "ls"}); err != nil {
		t.Fatal(err)
	}
	if viaCtx != "p1" || cmdP != "dflt" || rootP != "p1" {
		t.Fatalf("ctx = %q, ls's project = %q, root's = %q", viaCtx, cmdP, rootP)
	}
}

// An int flag with Default "eighty" kept 0 silently, and Required then
// counted as satisfied.
func TestInvalidDefaultDoesNotSatisfyRequired(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	var port int
	cmd := clix.NewCommand("serve", clix.WithCommandRun(noop))
	cmd.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "port", Required: true}, Default: "eighty", Value: &port})
	app.Root.AddCommand(cmd)
	if err := cmd.Flags.Err(); err == nil || !strings.Contains(err.Error(), "eighty") {
		t.Fatalf("Err() = %v", err)
	}
	var missing *clix.MissingFlagsError
	if err := app.Run(bg, []string{"serve"}); !errors.As(err, &missing) {
		t.Fatalf("Run = %v (port %d), want --port missing", err, port)
	}
	var got string
	var ok bool
	cmd.Run = func(ctx *clix.Context) error { got, ok = ctx.String("port"); return nil }
	if err := app.Run(bg, []string{"serve", "--port", "80"}); err != nil || got != "80" || !ok {
		t.Fatalf("Run = %v, ctx.String(port) = %q", err, got)
	}
}

// Validate missed names that cannot be typed, a short name longer than
// one letter (which hides a cluster), the same short name on a command and
// an ancestor for different flags, and a command that can do nothing.
func TestValidateReportsMoreMistakes(t *testing.T) {
	app, _ := newTestApp(t, "v")
	c := clix.NewCommand("c", clix.WithCommandRun(noop))
	c.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "aa", Short: "a"}})
	c.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "bb", Short: "b"}})
	c.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "abab", Short: "ab"}})
	c.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "dry=run"}})
	c.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "-x"}})
	c.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "two words"}})
	c.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "eq", Short: "="}})
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Short: "a"}})
	shared := clix.NewCommand("ls", clix.WithCommandRun(noop))
	app.Root.AddCommand(clix.NewGroup("g1", "", shared))
	app.Root.AddCommand(clix.NewGroup("g2", "", shared))
	app.Root.AddCommand(c)
	app.Root.AddCommand(clix.NewCommand("empty"))
	assertValidate(t, app,
		`v c: flag --abab: short name -ab is longer than one letter; -ab then means --abab instead of the cluster of -a -b`,
		`v c: flag name "dry=run" contains "=", so --dry=run cannot be typed on a command line`,
		`v c: flag name "-x" starts with "-", so ---x cannot be typed on a command line`,
		`v c: flag name "two words" contains white space, so --two words cannot be typed on a command line`,
		`v c: flag --eq: short name "=" contains "=", so it cannot be typed`,
		`v c: flag --aa: -a is also the short name of --verbose on "v"; before the command name -a means --verbose, after it --aa`,
		`v empty: command has no Run handler and no children, so running it fails`,
	)
}
