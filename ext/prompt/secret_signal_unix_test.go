//go:build darwin || linux

package prompt_test

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// Line mode on a terminal (TERM=dumb, or output that is not a terminal):
// SIGINT during a secret prompt turns echo back on.
func TestLineSecretRestoresEchoOnSignal(t *testing.T) {
	for name, dumb := range map[string]bool{"TERM=dumb": true, "Out not a terminal": false} {
		t.Run(name, func(t *testing.T) {
			rawEnv(t)
			if dumb {
				t.Setenv("TERM", "dumb")
			}
			term := ptytest.New(t)
			var out io.Writer = term.TTY
			if !dumb {
				out = io.Discard
			}
			tp := prompt.TerminalPrompter{In: term.TTY, Out: out}
			done := make(chan error, 1)
			go func() {
				_, err := tp.Prompt(context.Background(), clix.WithLabel("Token"), clix.WithSecret())
				done <- err
			}()
			deadline := time.Now().Add(5 * time.Second)
			for {
				if echo, _ := ptytest.Echo(term.TTY); !echo {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("echo never turned off")
				}
				time.Sleep(5 * time.Millisecond)
			}
			time.Sleep(50 * time.Millisecond)
			if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if !errors.Is(err, clix.ErrCancelled) {
					t.Fatalf("err = %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("prompt did not return")
			}
			if echo, _ := ptytest.Echo(term.TTY); !echo {
				t.Fatal("echo left off")
			}
		})
	}
}
