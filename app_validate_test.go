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
