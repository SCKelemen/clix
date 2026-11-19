//go:build darwin || linux

package clix_test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// In the default mode (InteractiveAuto) on a terminal, only Ambient flags
// on the command line still lead to prompts, answered here by typing into
// the terminal through the default TextPrompter.
func TestAmbientFlagsPromptOnTerminalInAutoMode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MOLN_DISABLE_PROMPTS", "")
	term := ptytest.New(t)
	app := clix.NewApp("moln", clix.WithAppIn(term.TTY), clix.WithAppErr(term.TTY), clix.WithAppOut(&bytes.Buffer{}))
	if app.Interactive != clix.InteractiveAuto || !app.IsInteractive() {
		t.Fatalf("Interactive = %v, IsInteractive = %v; want auto, true on a pty", app.Interactive, app.IsInteractive())
	}
	var target, name string
	var size int
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "target", Ambient: true}, Value: &target})
	create := clix.NewCommand("create", clix.WithCommandRun(noop))
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
	create.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "size", Required: true}, Value: &size})
	create.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "dry-run"}})
	app.Root.AddCommand(clix.NewGroup("widgets", "Manage widgets", create))

	done := make(chan error, 1)
	go func() { done <- app.Run(bg, []string{"--target", "X", "widgets", "create"}) }()
	term.WaitOutput(t, "Name", 5*time.Second)
	term.Type(t, "w1\n")
	term.WaitOutput(t, "Size", 5*time.Second)
	term.Type(t, "3\n")
	select {
	case err := <-done:
		if err != nil || target != "X" || name != "w1" || size != 3 {
			t.Fatalf("err = %v, target %q, name %q, size %d", err, target, name, size)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after both answers")
	}

	// A flag that is not Ambient still means a scripted call: no prompt.
	term.ResetOutput()
	err := app.Run(bg, []string{"--target", "X", "widgets", "create", "--dry-run"})
	var missing *clix.MissingFlagsError
	if !errors.As(err, &missing) || !reflect.DeepEqual(missing.Flags, []string{"name", "size"}) {
		t.Fatalf("with --dry-run: err = %v, want a *MissingFlagsError for name and size", err)
	}
	time.Sleep(50 * time.Millisecond) // let the drain goroutine catch up
	if out := term.Output(); out != "" {
		t.Fatalf("prompted with --dry-run: %q", out)
	}
}
