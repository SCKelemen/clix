package clix_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
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
