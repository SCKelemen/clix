//go:build darwin || linux

package prompt_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// A raw prompt inside another raw prompt's Validate function, then SIGINT
// or SIGTERM: both prompts are cancelled and the terminal is back in
// canonical mode every time, whichever prompt's watcher sees the signal
// first.
func TestNestedRawPromptsRestoreOnSignal(t *testing.T) {
	rawEnv(t)
	for i := 0; i < 15; i++ {
		sig := syscall.SIGINT
		if i%2 == 1 {
			sig = syscall.SIGTERM
		}
		term := ptytest.New(t)
		tp := prompt.TerminalPrompter{In: term.TTY, Out: term.TTY}
		innerErr := make(chan error, 1)
		outer := make(chan error, 1)
		go func() {
			_, err := tp.Prompt(context.Background(), clix.WithLabel("Name"), clix.WithValidate(func(string) error {
				_, err := tp.Prompt(context.Background(), clix.WithLabel("Sure?"), prompt.Select(envs))
				innerErr <- err
				return err
			}))
			outer <- err
		}()
		term.WaitRaw(t)
		term.Type(t, "x", "\r") // runs Validate, which opens the inner prompt
		term.WaitOutput(t, "production-eu", 3*time.Second)
		if err := syscall.Kill(os.Getpid(), sig); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-innerErr:
			if !errors.Is(err, clix.ErrCancelled) {
				t.Fatalf("run %d: inner err = %v", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("run %d: inner prompt did not return", i)
		}
		select {
		case err := <-outer:
			if !errors.Is(err, clix.ErrCancelled) {
				t.Fatalf("run %d: outer err = %v", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("run %d: outer prompt did not return", i)
		}
		if canon, _ := ptytest.Canonical(term.TTY); !canon {
			t.Fatalf("run %d (%v): terminal left in raw mode", i, sig)
		}
	}
}

// Without a signal, the inner prompt ending keeps the terminal raw for the
// outer prompt, and the outer one restores it.
func TestNestedRawPromptsKeepRawForOuter(t *testing.T) {
	rawEnv(t)
	term := ptytest.New(t)
	tp := prompt.TerminalPrompter{In: term.TTY, Out: term.TTY}
	outer := make(chan result, 1)
	calls := 0
	go func() {
		v, err := tp.Prompt(context.Background(), clix.WithLabel("Name"), clix.WithValidate(func(string) error {
			calls++
			if calls > 1 {
				return nil
			}
			if _, err := tp.Prompt(context.Background(), clix.WithLabel("Sure?"), prompt.Select(envs)); err != nil {
				return err
			}
			return errors.New("try again") // keeps the outer prompt open
		}))
		outer <- result{v, err}
	}()
	term.WaitRaw(t)
	term.Type(t, "x", "\r")
	term.WaitOutput(t, "production-eu", 3*time.Second)
	term.Type(t, "\r") // answer the inner prompt
	term.WaitOutput(t, "try again", 3*time.Second)
	if canon, _ := ptytest.Canonical(term.TTY); canon {
		t.Fatal("the inner prompt left raw mode while the outer prompt was open")
	}
	if out := term.Output(); strings.Count(out, "\x1b[?2004l") != 0 {
		t.Fatal("the inner prompt turned bracketed paste off for the outer one")
	}
	term.Type(t, "\r")
	select {
	case r := <-outer:
		if r.err != nil || r.val != "x" {
			t.Fatalf("outer = %q, %v", r.val, r.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("outer prompt did not return")
	}
	term.WaitCanonical(t)
}
