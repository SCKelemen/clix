package clix

import (
	"context"
	"strings"
	"testing"
)

func TestRootFlagCountsAsFlagsPassed(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "myapp")
	var project, name string
	app.Flags().StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "project"}, Value: &project})
	cmd := NewCommand("mk", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "name", Required: true}, Value: &name})
	app.Root.AddCommand(cmd)

	prompted := false
	app.Interactive = InteractiveAlways
	app.Prompter = prompterFunc(func(context.Context, ...PromptOption) (string, error) {
		prompted = true
		return "x", nil
	})

	err := app.Run(context.Background(), []string{"mk", "--project", "p"})
	if prompted {
		t.Fatal("prompted even though a root flag was passed")
	}
	if err == nil || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("Run error = %v, want missing --name", err)
	}
}
