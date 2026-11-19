package clix

import (
	"context"
	"strings"
	"testing"
)

func TestCommandShortFlagBeatsRootShortFlag(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	var project string
	var port int
	app.Flags().StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "project", Short: "p"}, Value: &project})
	cmd := NewCommand("serve", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.IntVar(IntVarOptions{FlagOptions: FlagOptions{Name: "port", Short: "p"}, Value: &port})
	app.Root.AddCommand(cmd)

	if err := app.Run(context.Background(), []string{"serve", "-p", "8080"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if port != 8080 || project != "" {
		t.Fatalf("port=%d project=%q; want the command's -p to win", port, project)
	}

	// A root flag still works after the command when the command lacks it.
	if err := app.Run(context.Background(), []string{"serve", "--project", "p1"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if project != "p1" {
		t.Fatalf("project=%q, want p1", project)
	}
}

func TestRootFlagBeforeCommandBindsToRoot(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	var global, local string
	app.Flags().StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "value"}, Value: &global})
	cmd := NewCommand("run", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "value"}, Value: &local})
	app.Root.AddCommand(cmd)

	if err := app.Run(context.Background(), []string{"--value", "g", "run", "--value", "l"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if global != "g" || local != "l" {
		t.Fatalf("global=%q local=%q, want g and l", global, local)
	}
}

func TestRootBoolShortDoesNotStealCommandShort(t *testing.T) {
	// Mirrors ext/format (-f) and ext/version (-v) registering root shorts.
	app, out, _ := newIsolatedApp(t, "moln")
	app.Flags().StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "format", Short: "f"}, Default: FormatText})
	app.Flags().BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "version", Short: "v"}})

	var force, verbose bool
	cmd := NewCommand("delete", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "force", Short: "f"}, Value: &force})
	cmd.Flags.BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "verbose", Short: "v"}, Value: &verbose})
	app.Root.AddCommand(cmd)

	if err := app.Run(context.Background(), []string{"delete", "-f", "-v"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !force || !verbose {
		t.Fatalf("force=%v verbose=%v, want both set", force, verbose)
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output (version or help?): %q", out.String())
	}
}

func TestDoubleDashEndsFlags(t *testing.T) {
	app, out, _ := newIsolatedApp(t, "moln")
	var text string
	ran := false
	cmd := NewCommand("echo", WithCommandRun(func(*Context) error { ran = true; return nil }))
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "text", Positional: true}, Value: &text})
	app.Root.AddCommand(cmd)

	for _, arg := range []string{"-x", "--help"} {
		ran, text = false, ""
		out.Reset()
		if err := app.Run(context.Background(), []string{"echo", "--", arg}); err != nil {
			t.Fatalf("echo -- %s: %v", arg, err)
		}
		if !ran || text != arg || out.Len() != 0 {
			t.Fatalf("echo -- %s: ran=%v text=%q out=%q", arg, ran, text, out.String())
		}
	}
}

func TestHelpIsCheckedBeforePositionals(t *testing.T) {
	app, out, _ := newIsolatedApp(t, "moln")
	app.Root.AddCommand(NewCommand("ls", WithCommandRun(func(*Context) error {
		t.Fatal("command ran")
		return nil
	})))
	if err := app.Run(context.Background(), []string{"ls", "--help", "foo"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out.String(), "USAGE") {
		t.Fatalf("no help printed: %q", out.String())
	}
}

func TestCommandCanClaimHelpShort(t *testing.T) {
	app, out, _ := newIsolatedApp(t, "moln")
	var host string
	cmd := NewCommand("ssh", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "host", Short: "h"}, Value: &host})
	app.Root.AddCommand(cmd)

	if err := app.Run(context.Background(), []string{"ssh", "-h", "example.com"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if host != "example.com" || out.Len() != 0 {
		t.Fatalf("host=%q out=%q; want -h to set --host", host, out.String())
	}
}

func TestUnknownSubcommandStillErrors(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	app.Root.AddCommand(NewGroup("projects", "Projects", NewCommand("list", WithCommandRun(func(*Context) error { return nil }))))
	err := app.Run(context.Background(), []string{"projects", "lsit"})
	if err == nil || !strings.Contains(err.Error(), "unknown command: moln projects lsit") {
		t.Fatalf("Run error = %v, want unknown command", err)
	}
}
