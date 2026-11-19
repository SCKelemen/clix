package clix_test

import (
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// Flags registered with app.Flags() before app.Root was replaced used to
// vanish, and "--project p1 ls" failed with "unknown command: myapp p1".
func TestFlagsRegisteredBeforeRootReplacedMove(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	var project string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Short: "p"}, Value: &project})
	app.Flags().SetStrict(true)
	app.Root = clix.NewGroup("myapp", "", clix.NewCommand("ls", clix.WithCommandRun(noop)))

	if err := app.Run(bg, []string{"--project", "p1", "ls"}); err != nil || project != "p1" {
		t.Fatalf("Run = %v, project = %q, want p1", err, project)
	}
	if err := app.Run(bg, []string{"-p", "p2", "ls"}); err != nil || project != "p2" {
		t.Fatalf("Run(-p) = %v, project = %q, want p2", err, project)
	}
	if !app.Flags().Strict() {
		t.Fatal("SetStrict(true) on the old root did not carry over")
	}
	if err := app.Flags().Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
}

// When the new root defines the same flag, the new root's definition wins
// and the conflict is reported.
func TestFlagsMovedToNewRootConflict(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	var oldP, newP string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project"}, Value: &oldP})
	root := clix.NewGroup("myapp", "", clix.NewCommand("ls", clix.WithCommandRun(noop)))
	root.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project"}, Value: &newP})
	app.Root = root

	if err := app.Run(bg, []string{"--project", "p1", "ls"}); err != nil {
		t.Fatal(err)
	}
	if newP != "p1" || oldP != "" {
		t.Fatalf("new root's project = %q, old = %q", newP, oldP)
	}
	if err := app.Flags().Err(); err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("Err() = %v, want the --project conflict", err)
	}
}
