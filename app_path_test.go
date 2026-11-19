package clix_test

import (
	"errors"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// A Required flag on the root was never enforced when a subcommand ran.
func TestRootRequiredEnforcedForSubcommand(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Required: true}})
	ran := false
	app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(mark(&ran))))

	err := app.Run(bg, []string{"ls"})
	var missing *clix.MissingFlagsError
	if !errors.As(err, &missing) || len(missing.Flags) != 1 || missing.Flags[0] != "project" {
		t.Fatalf("Run = %v (ran=%v), want *MissingFlagsError for --project", err, ran)
	}
	if err := app.Run(bg, []string{"ls", "--project", "p"}); err != nil || !ran {
		t.Fatalf("Run(ls --project p) = %v, ran = %v", err, ran)
	}
}

// newGroupApp builds "moln project ls" with --org on the project group.
func newGroupApp(t *testing.T, required bool) (app *clix.App, org, viaCtx *string, src *clix.Source) {
	app, _ = newTestApp(t, "moln")
	org, viaCtx, src = new(string), new(string), new(clix.Source)
	ls := clix.NewCommand("ls", clix.WithCommandRun(func(ctx *clix.Context) error {
		*viaCtx, *src, _ = ctx.EffectiveString("org")
		return nil
	}))
	project := clix.NewGroup("project", "Projects", ls)
	project.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "org", Required: required}, Value: org})
	app.Root.AddCommand(project)
	return app, org, viaCtx, src
}

// A group's flag was parsed, but Context getters in the leaf could not
// see it.
func TestGroupFlagVisibleToContext(t *testing.T) {
	app, org, viaCtx, src := newGroupApp(t, false)
	if err := app.Run(bg, []string{"project", "--org", "acme", "ls"}); err != nil {
		t.Fatal(err)
	}
	if *org != "acme" || *viaCtx != "acme" || *src != clix.SourceCommandFlag {
		t.Fatalf("bound org = %q, ctx.EffectiveString(org) = %q from %v", *org, *viaCtx, *src)
	}
}

// Env and config never filled a group's flag.
func TestGroupFlagReadsEnvAndConfig(t *testing.T) {
	t.Run("env", func(t *testing.T) {
		app, org, viaCtx, src := newGroupApp(t, false)
		t.Setenv("MOLN_ORG", "acme")
		if err := app.Run(bg, []string{"project", "ls"}); err != nil {
			t.Fatal(err)
		}
		if *org != "acme" || *viaCtx != "acme" || *src != clix.SourceEnvVar {
			t.Fatalf("bound org = %q, ctx = %q from %v", *org, *viaCtx, *src)
		}
	})
	t.Run("config", func(t *testing.T) {
		app, org, viaCtx, src := newGroupApp(t, false)
		writeConfigFile(t, app, "org: acme\n")
		if err := app.Run(bg, []string{"project", "ls"}); err != nil {
			t.Fatal(err)
		}
		if *org != "acme" || *viaCtx != "acme" || *src != clix.SourceConfigFile {
			t.Fatalf("bound org = %q, ctx = %q from %v", *org, *viaCtx, *src)
		}
	})
}

// Required on a group's flag was never enforced.
func TestGroupRequiredEnforced(t *testing.T) {
	app, _, _, _ := newGroupApp(t, true)
	err := app.Run(bg, []string{"project", "ls"})
	var missing *clix.MissingFlagsError
	if !errors.As(err, &missing) || missing.Flags[0] != "org" {
		t.Fatalf("Run = %v, want *MissingFlagsError for --org", err)
	}
	if err := app.Run(bg, []string{"project", "--org", "a", "ls"}); err != nil {
		t.Fatal(err)
	}
}

// Commands added by extensions, and their subcommands, skip the Required
// flags of the commands above them.
func TestExtensionCommandsSkipAncestorRequired(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Required: true}})
	ran := 0
	version := clix.NewCommand("version", clix.WithCommandRun(func(*clix.Context) error { ran++; return nil }))
	version.IsExtensionCommand = true
	get := clix.NewCommand("get", clix.WithCommandRun(func(*clix.Context) error { ran++; return nil }))
	get.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "key", Required: true}})
	config := clix.NewGroup("config", "Config", get)
	config.IsExtensionCommand = true
	app.Root.AddCommand(version)
	app.Root.AddCommand(config)

	if err := app.Run(bg, []string{"version"}); err != nil {
		t.Fatalf("version: %v", err)
	}
	if err := app.Run(bg, []string{"config", "get", "--key", "k"}); err != nil {
		t.Fatalf("config get: %v", err)
	}
	// The extension command's own subtree is still checked.
	err := app.Run(bg, []string{"config", "get"})
	var missing *clix.MissingFlagsError
	if !errors.As(err, &missing) || len(missing.Flags) != 1 || missing.Flags[0] != "key" {
		t.Fatalf("config get without --key = %v, want only --key missing", err)
	}
	if ran != 2 {
		t.Fatalf("ran %d times, want 2", ran)
	}
}

// A group's flag written after the leaf was an unknown flag: the parser
// looked only at the leaf and the root.
func TestGroupFlagAfterLeaf(t *testing.T) {
	app, org, viaCtx, _ := newGroupApp(t, false)
	if err := app.Run(bg, []string{"project", "ls", "--org", "acme"}); err != nil {
		t.Fatal(err)
	}
	if *org != "acme" || *viaCtx != "acme" {
		t.Fatalf("org = %q, ctx = %q", *org, *viaCtx)
	}
}
