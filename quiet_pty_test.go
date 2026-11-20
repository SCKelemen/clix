//go:build darwin || linux

package clix_test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// quietTerminalApp returns an app whose In and Err are a pseudo-terminal,
// in the default InteractiveAuto mode, with --quiet enabled.
func quietTerminalApp(t *testing.T, term *ptytest.Term, withPromptExtension bool) *clix.App {
	t.Helper()
	terminalEnv(t, "MYAPP_DISABLE_PROMPTS=")
	app := clix.NewApp("myapp", clix.WithAppIn(term.TTY), clix.WithAppErr(term.TTY), clix.WithAppOut(&bytes.Buffer{}))
	if withPromptExtension {
		app.AddExtension(prompt.Extension{})
	}
	app.EnableQuiet(clix.QuietOptions{Short: "q"})
	return app
}

// assertNothingDrawn waits for the drain goroutine and checks that the
// terminal received nothing.
func assertNothingDrawn(t *testing.T, term *ptytest.Term) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
	if out := term.Output(); out != "" {
		t.Fatalf("drew on the terminal under --quiet: %q", out)
	}
}

// On a real terminal, --quiet answers a handler's prompts with their
// defaults: nothing is drawn and nothing needs to be typed.
func TestQuietPromptsOnTerminal(t *testing.T) {
	for _, ext := range []bool{false, true} {
		name := "TextPrompter"
		if ext {
			name = "TerminalPrompter"
		}
		t.Run(name, func(t *testing.T) {
			term := ptytest.New(t)
			app := quietTerminalApp(t, term, ext)
			if !app.IsInteractive() {
				t.Fatal("not interactive on a pty before --quiet")
			}
			var confirm, deflt, text string
			app.Root.Run = func(ctx *clix.Context) error {
				var err error
				if confirm, err = ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{Label: "Delete everything?", Confirm: true}); err != nil {
					return err
				}
				if deflt, err = ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{Label: "Really?", Confirm: true, Default: "n"}); err != nil {
					return err
				}
				text, err = ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{Label: "Region", Default: "eu"})
				return err
			}
			done := make(chan error, 1)
			go func() { done <- app.Run(bg, []string{"-q"}) }()
			select {
			case err := <-done:
				if err != nil || confirm != "y" || deflt != "n" || text != "eu" {
					t.Fatalf("err %v, answers %q %q %q; want y n eu", err, confirm, deflt, text)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Run waited for input under --quiet")
			}
			assertNothingDrawn(t, term)

			// A prompt without a default fails instead of waiting.
			app.Root.Run = func(ctx *clix.Context) error {
				_, err := ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{Label: "Name"})
				return err
			}
			go func() { done <- app.Run(bg, []string{"--quiet"}) }()
			select {
			case err := <-done:
				if !errors.Is(err, clix.ErrPromptsDisabled) || !errors.Is(err, clix.ErrQuiet) {
					t.Fatalf("err = %v, want ErrPromptsDisabled and ErrQuiet", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Run waited for input under --quiet")
			}
			assertNothingDrawn(t, term)
		})
	}
}

// On a terminal a missing required flag is prompted for, but not under
// --quiet.
func TestQuietRequiredFlagsOnTerminal(t *testing.T) {
	term := ptytest.New(t)
	app := quietTerminalApp(t, term, false)
	var name string
	create := clix.NewCommand("create", clix.WithCommandRun(noop))
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
	app.Root.AddCommand(create)

	done := make(chan error, 1)
	go func() { done <- app.Run(bg, []string{"--quiet", "create"}) }()
	select {
	case err := <-done:
		var missing *clix.MissingFlagsError
		if !errors.As(err, &missing) || !reflect.DeepEqual(missing.Flags, []string{"name"}) {
			t.Fatalf("err = %v, want a *MissingFlagsError for --name", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run prompted under --quiet")
	}
	assertNothingDrawn(t, term)

	// Without --quiet the terminal gets the prompt.
	go func() { done <- app.Run(bg, []string{"create"}) }()
	term.WaitOutput(t, "Name", 5*time.Second)
	term.Type(t, "w1\n")
	select {
	case err := <-done:
		if err != nil || name != "w1" {
			t.Fatalf("err %v, name %q", err, name)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the answer")
	}
}

// Under --quiet a progress indicator draws nothing on a terminal: no
// spinner, no cursor control, no final line.
func TestQuietProgressOnTerminal(t *testing.T) {
	term := ptytest.New(t)
	app := quietTerminalApp(t, term, false)
	app.Root.Run = deploy
	if err := app.Run(bg, []string{"--quiet"}); err != nil {
		t.Fatal(err)
	}
	assertNothingDrawn(t, term)

	// The same handler draws a spinner without --quiet.
	if err := app.Run(bg, []string{}); err != nil {
		t.Fatal(err)
	}
	term.WaitOutput(t, "Deploying: deployed", 5*time.Second)
}
