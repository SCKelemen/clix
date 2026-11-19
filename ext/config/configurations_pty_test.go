//go:build darwin || linux

package config

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// On a real terminal, in the default mode, delete asks before deleting,
// and a script (stdin not a terminal) deletes without asking.
func TestConfigurationsDeleteAsksOnTerminal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_CONFIGURATION", "")
	t.Setenv("PROBE_DISABLE_PROMPTS", "")
	term := ptytest.New(t)
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppIn(term.TTY), clix.WithAppErr(term.TTY), clix.WithAppOut(out))
	app.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	app.AddExtension(Extension{Configurations: true})
	ctx := context.Background()
	for _, n := range []string{"a", "b"} {
		if err := app.Run(ctx, []string{"config", "configurations", "create", n, "--no-activate"}); err != nil {
			t.Fatal(err)
		}
	}

	ask := func(answer string) error {
		t.Helper()
		term.ResetOutput()
		done := make(chan error, 1)
		go func() { done <- app.Run(ctx, []string{"config", "configurations", "delete", "a", "b"}) }()
		term.WaitOutput(t, `Delete configurations "a", "b"`, 5*time.Second)
		term.Type(t, answer+"\n")
		select {
		case err := <-done:
			return err
		case <-time.After(5 * time.Second):
			t.Fatal("delete did not return after the answer")
		}
		return nil
	}
	if err := ask("n"); err == nil || err.Error() != "nothing deleted" {
		t.Fatalf("answered n: %v", err)
	}
	if names, _ := app.Configurations(); len(names) != 3 {
		t.Fatalf("deleted after n: %q", names)
	}
	if err := ask("y"); err != nil {
		t.Fatalf("answered y: %v", err)
	}
	if names, _ := app.Configurations(); len(names) != 1 {
		t.Fatalf("configurations after y: %q", names)
	}

	// Not a terminal: no question.
	if err := app.Run(ctx, []string{"config", "configurations", "create", "c", "--no-activate"}); err != nil {
		t.Fatal(err)
	}
	app.In = strings.NewReader("")
	term.ResetOutput()
	if err := app.Run(ctx, []string{"config", "configurations", "delete", "c"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if got := term.Output(); got != "" {
		t.Fatalf("asked without a terminal: %q", got)
	}
}
