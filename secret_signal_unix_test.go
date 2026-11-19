//go:build darwin || linux

package clix_test

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// waitEcho waits until the terminal's echo setting is want.
func waitEcho(t *testing.T, term *ptytest.Term, want bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if echo, err := ptytest.Echo(term.TTY); err == nil && echo == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("echo never became %v", want)
}

// SIGINT during a line-mode secret prompt turns echo back on and cancels
// the prompt, instead of leaving the terminal without echo.
func TestSecretPromptRestoresEchoOnSignal(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			term := ptytest.New(t)
			tp := clix.TextPrompter{In: term.TTY, Out: io.Discard}
			done := make(chan error, 1)
			go func() {
				_, err := tp.Prompt(context.Background(), clix.WithLabel("Token"), clix.WithSecret())
				done <- err
			}()
			waitEcho(t, term, false)
			time.Sleep(50 * time.Millisecond) // the signal watcher is installed right after
			if err := syscall.Kill(os.Getpid(), sig); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if !errors.Is(err, clix.ErrCancelled) {
					t.Fatalf("err = %v, want ErrCancelled", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("prompt did not return after the signal")
			}
			waitEcho(t, term, true)
		})
	}
}
