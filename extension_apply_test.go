package clix_test

import (
	"errors"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

type failingExtension struct{ calls *int }

func (e failingExtension) Extend(*clix.App) error {
	*e.calls++
	return errors.New("ext boom")
}

// An extension error used to be returned only by the first Run; later Runs
// carried on with half the extensions applied.
func TestExtensionErrorIsSticky(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	ran := false
	app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(mark(&ran))))
	calls := 0
	app.AddExtension(failingExtension{calls: &calls})
	for i := 0; i < 2; i++ {
		if err := app.Run(bg, []string{"ls"}); err == nil || err.Error() != "ext boom" {
			t.Fatalf("Run #%d = %v, want the extension error", i+1, err)
		}
	}
	if ran || calls != 1 {
		t.Fatalf("ran = %v, Extend called %d times", ran, calls)
	}
}

type addCommandExtension struct{ name string }

func (e addCommandExtension) Extend(app *clix.App) error {
	app.Root.AddCommand(clix.NewCommand(e.name, clix.WithCommandRun(noop)))
	return nil
}

// Extensions added after the first Run used to be ignored.
func TestExtensionAddedAfterRunIsApplied(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	app.AddExtension(addCommandExtension{name: "one"})
	if err := app.Run(bg, []string{"one"}); err != nil {
		t.Fatal(err)
	}
	app.AddExtension(addCommandExtension{name: "two"})
	if err := app.Run(bg, []string{"two"}); err != nil {
		t.Fatalf("Run(two) = %v", err)
	}
	if n := len(app.Root.Children); n != 2 {
		t.Fatalf("root has %d children, want 2 (each extension applied once)", n)
	}
}

type chainExtension struct{}

func (chainExtension) Extend(app *clix.App) error {
	app.AddExtension(addCommandExtension{name: "chained"})
	return nil
}

// An extension may add another extension, which is applied right after.
func TestExtensionCanAddExtension(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	app.AddExtension(chainExtension{})
	if err := app.Run(bg, []string{"chained"}); err != nil {
		t.Fatal(err)
	}
}
