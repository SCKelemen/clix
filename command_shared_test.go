package clix_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/help"
)

// One *Command registered under two groups used to follow the parent of
// the last prepare: "s a ls" demanded group b's required --bflag, and
// --aflag was an unknown flag.
func TestSharedCommandUsesTheMatchedPath(t *testing.T) {
	app, out := newTestApp(t, "s")
	var ran, viaCtx string
	shared := clix.NewCommand("ls", clix.WithCommandRun(func(ctx *clix.Context) error {
		ran = ctx.Command.Path()
		viaCtx, _ = ctx.String("aflag")
		return nil
	}))
	a := clix.NewGroup("a", "A", shared)
	a.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "aflag"}})
	b := clix.NewGroup("b", "B", shared)
	b.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "bflag", Required: true}})
	app.Root.AddCommand(a)
	app.Root.AddCommand(b)

	cases := []struct {
		args          []string
		ran, ctx, err string
	}{
		{args: []string{"a", "ls"}, ran: "s a ls"},
		{args: []string{"a", "ls", "--aflag", "x"}, ran: "s a ls", ctx: "x"},
		{args: []string{"b", "ls", "--bflag", "y"}, ran: "s b ls"},
		{args: []string{"b", "ls"}, err: "missing required flags: --bflag"},
		{args: []string{"b", "ls", "--aflag", "x"}, err: "unknown flag: --aflag"},
		{args: []string{"a", "ls"}, ran: "s a ls"},
	}
	for _, c := range cases {
		ran, viaCtx = "", ""
		err := app.Run(bg, c.args)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%q: err = %v, want %q", c.args, err, c.err)
			}
			continue
		}
		if err != nil || ran != c.ran || viaCtx != c.ctx {
			t.Errorf("%q: err = %v, ran %q, ctx.String(aflag) = %q; want %q, %q", c.args, err, ran, viaCtx, c.ran, c.ctx)
		}
	}

	out.Reset()
	if err := app.Run(bg, []string{"a", "ls", "--help"}); err != nil || !strings.Contains(out.String(), "s a ls") {
		t.Fatalf("help: %v\n%s", err, out)
	}
	var missing *clix.MissingFlagsError
	if err := app.Run(bg, []string{"b", "ls"}); !errors.As(err, &missing) {
		t.Fatalf("b ls = %v", err)
	}
	if err := app.Validate(); err != nil {
		t.Fatalf("a shared command is not a definition error: %v", err)
	}
}

// Help from ext/help and ResolvePath named the group that listed a shared
// command last: Run prepared the tree, which linked the command to that
// group, and ResolvePath never linked it to the group it went through, so
// "help 'alpha status'" printed "sh beta status".
func TestSharedCommandPathFollowsResolvePath(t *testing.T) {
	app, out := newTestApp(t, "sh")
	shared := clix.NewCommand("status", clix.WithCommandRun(noop))
	app.Root.AddCommand(clix.NewGroup("alpha", "A", shared))
	app.Root.AddCommand(clix.NewGroup("beta", "B", shared))
	app.AddExtension(help.Extension{})
	for _, args := range [][]string{{"help", "alpha status"}, {"alpha", "status", "--help"}, {"beta", "status"}, {"help", "alpha status"}} {
		out.Reset()
		if err := app.Run(bg, args); err != nil {
			t.Fatal(err)
		}
		if args[0] != "beta" && !strings.Contains(out.String(), "sh alpha status") {
			t.Errorf("%q: help names the wrong path:\n%s", args, out)
		}
	}
	for _, group := range []string{"alpha", "beta", "alpha"} {
		if got := app.Root.ResolvePath([]string{group, "status"}); got == nil || got.Path() != "sh "+group+" status" {
			t.Errorf("ResolvePath(%s status).Path() = %q", group, got.Path())
		}
	}

	// Preparing the tree again (Validate, the next Run) keeps the path a
	// command was last reached through.
	if err := app.Run(bg, []string{"alpha", "status"}); err != nil {
		t.Fatal(err)
	}
	_ = app.Validate()
	if got := shared.Path(); got != "sh alpha status" {
		t.Errorf("after Validate, Path() = %q", got)
	}
}

// A Run started by a handler that goes through the other group put the
// outer handler's shared command on the other path.
func TestNestedRunKeepsSharedCommandPath(t *testing.T) {
	app, _ := newTestApp(t, "sh")
	var before, after string
	shared := clix.NewCommand("status")
	shared.Run = func(ctx *clix.Context) error {
		if before != "" {
			return nil // the nested Run
		}
		before = ctx.Command.Path()
		if err := app.Run(ctx, []string{"beta", "status"}); err != nil {
			return err
		}
		after = ctx.Command.Path()
		return nil
	}
	app.Root.AddCommand(clix.NewGroup("alpha", "A", shared))
	app.Root.AddCommand(clix.NewGroup("beta", "B", shared))
	if err := app.Run(bg, []string{"alpha", "status"}); err != nil {
		t.Fatal(err)
	}
	if before != "sh alpha status" || after != before {
		t.Errorf("Path() = %q before the nested Run, %q after", before, after)
	}
}
