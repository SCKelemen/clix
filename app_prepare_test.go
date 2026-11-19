package clix_test

import (
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// A child appended to Children after a Run used to keep nil Flags, and Run
// panicked on it.
func TestRunPreparesChildrenAddedAfterRun(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(noop)))
	if err := app.Run(bg, []string{"ls"}); err != nil {
		t.Fatal(err)
	}
	ran := false
	app.Root.Children = append(app.Root.Children, &clix.Command{Name: "late", Run: mark(&ran)})
	if err := app.Run(bg, []string{"late"}); err != nil || !ran {
		t.Fatalf("Run(late) = %v, ran = %v", err, ran)
	}
}

// Replacing Root after a Run used to skip preparing the new tree.
func TestRunPreparesRootReplacedAfterRun(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	_ = app.Run(bg, []string{})
	ran := false
	app.Root = &clix.Command{Name: "myapp", Children: []*clix.Command{{Name: "ls", Run: mark(&ran)}}}
	if err := app.Run(bg, []string{"ls"}); err != nil || !ran {
		t.Fatalf("Run(ls) = %v, ran = %v", err, ran)
	}
}

// A zero-value App has nil Out and Err; --help used to panic.
func TestZeroValueAppDoesNotPanic(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stdout := captureStdout(t)
	app := &clix.App{Name: "z", Root: &clix.Command{Name: "z", Run: noop}}
	if err := app.Run(bg, []string{"--help"}); err != nil {
		t.Fatalf("Run(--help) = %v", err)
	}
	if got := stdout(); !strings.Contains(got, "USAGE") {
		t.Fatalf("help went elsewhere; stdout = %q", got)
	}
}
