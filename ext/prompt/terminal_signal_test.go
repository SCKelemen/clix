//go:build unix

package prompt

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
)

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func TestRestoreStopsSignalWatcher(t *testing.T) {
	ts := &TerminalState{}
	ts.watchSignals()
	if err := ts.Restore(); err != nil {
		t.Fatal(err)
	}
	waitClosed(t, ts.watchDone, "the signal watcher to exit after Restore")
	if err := ts.Restore(); err != nil {
		t.Fatalf("second Restore: %v", err)
	}
}

func TestSignalInRawModeCancelsPromptWithoutExiting(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })

	ts := &TerminalState{}
	ts.watchSignals()
	defer func() {
		if err := ts.Restore(); err != nil {
			t.Error(err)
		}
	}()

	done := make(chan error, 1)
	go func() {
		_, err := ts.readKey(context.Background(), r)
		done <- err
	}()

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, clix.ErrCancelled) {
			t.Fatalf("readKey error = %v, want clix.ErrCancelled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("readKey did not return after SIGTERM")
	}
	waitClosed(t, ts.watchDone, "the signal watcher to exit after the signal")

	// The keystroke that was being waited for goes to the next prompt.
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	var next *TerminalState
	key, err := next.readKey(context.Background(), r)
	if err != nil || key.Rune != 'x' {
		t.Fatalf("next readKey = %v, %v; want x", key, err)
	}
}

func TestReadKeyHonorsContext(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	var ts *TerminalState
	if _, err := ts.readKey(ctx, r); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("readKey error = %v, want context.DeadlineExceeded", err)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		t.Fatal(err)
	}
	if key, err := ts.readKey(context.Background(), r); err != nil || key != KeyEnter {
		t.Fatalf("readKey after timeout = %v, %v; want Enter", key, err)
	}
}
