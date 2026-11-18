package clix

import (
	"context"
	"testing"
)

func TestEnvVarsAliasesAreCheckedInOrder(t *testing.T) {
	newCmd := func(app *App, project *string, got *string, src *Source) {
		cmd := NewCommand("ls", WithCommandRun(func(ctx *Context) error {
			*got, *src, _ = ctx.EffectiveString("project")
			return nil
		}))
		cmd.Flags.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "project", EnvVars: []string{"CLOUDSDK_CORE_PROJECT", "GOOGLE_CLOUD_PROJECT"}},
			Value:       project,
		})
		app.Root.AddCommand(cmd)
	}

	t.Run("first alias", func(t *testing.T) {
		app, _, _ := newIsolatedApp(t, "moln")
		t.Setenv("CLOUDSDK_CORE_PROJECT", "p1")
		t.Setenv("GOOGLE_CLOUD_PROJECT", "p2")
		var project, got string
		var src Source
		newCmd(app, &project, &got, &src)
		if err := app.Run(context.Background(), []string{"ls"}); err != nil {
			t.Fatalf("Run error: %v", err)
		}
		if project != "p1" || got != "p1" {
			t.Fatalf("project=%q ctx=%q source=%v; want p1 from the first alias", project, got, src)
		}
	})

	t.Run("later alias", func(t *testing.T) {
		app, _, _ := newIsolatedApp(t, "moln")
		t.Setenv("GOOGLE_CLOUD_PROJECT", "p2")
		var project, got string
		var src Source
		newCmd(app, &project, &got, &src)
		if err := app.Run(context.Background(), []string{"ls"}); err != nil {
			t.Fatalf("Run error: %v", err)
		}
		if project != "p2" || got != "p2" {
			t.Fatalf("project=%q ctx=%q; want p2", project, got)
		}
	})

	t.Run("functional option", func(t *testing.T) {
		fs := NewFlagSet("x")
		var v string
		fs.StringVar(WithFlagName("project"), WithFlagEnvVars("A", "B"), WithStringValue(&v))
		if got := fs.lookup("project").EnvVars; len(got) != 2 || got[0] != "A" || got[1] != "B" {
			t.Fatalf("EnvVars = %v, want [A B]", got)
		}
	})
}
