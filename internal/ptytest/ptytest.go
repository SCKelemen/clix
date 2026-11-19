// Package ptytest opens pseudo-terminals for tests that drive prompts and
// shells the way a terminal emulator would. It is imported only by tests,
// so it is never linked into programs that use clix.
package ptytest

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ErrUnsupported is returned by Open on platforms without a pty
// implementation here.
var ErrUnsupported = errors.New("ptytest: pseudo-terminals are not supported on this platform")

// Term is an open pseudo-terminal. TTY is the terminal device a program
// under test reads from and writes to. Writing to the controlling side
// (Type) is typing; everything the program writes to TTY is collected and
// available from Output.
type Term struct {
	TTY    *os.File
	master *os.File

	mu   sync.Mutex
	out  bytes.Buffer
	done chan struct{}
}

// New opens a pseudo-terminal of 24 rows by 80 columns and closes it when
// the test ends. It skips the test when ptys are unavailable.
func New(t testing.TB) *Term {
	t.Helper()
	master, tty, err := Open()
	if err != nil {
		t.Skipf("no pty available: %v", err)
	}
	term := &Term{TTY: tty, master: master, done: make(chan struct{})}
	if err := SetSize(tty, 24, 80); err != nil {
		t.Logf("set pty size: %v", err)
	}
	go term.drain()
	t.Cleanup(func() {
		_ = tty.Close()
		_ = master.Close()
		select {
		case <-term.done:
		case <-time.After(time.Second):
		}
	})
	return term
}

func (t *Term) drain() {
	defer close(t.done)
	buf := make([]byte, 4096)
	for {
		n, err := t.master.Read(buf)
		if n > 0 {
			t.mu.Lock()
			t.out.Write(buf[:n])
			t.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// Output returns everything written to the terminal so far, as raw bytes
// including escape sequences.
func (t *Term) Output() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.out.String()
}

// ResetOutput forgets the output collected so far.
func (t *Term) ResetOutput() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.out.Reset()
}

// Write sends raw bytes as if typed or pasted in one go.
func (t *Term) Write(p []byte) (int, error) {
	return t.master.Write(p)
}

// Type sends each key as a separate write, pausing between them like a
// person typing, so the program sees one keystroke per read.
func (t *Term) Type(tb testing.TB, keys ...string) {
	tb.Helper()
	for _, k := range keys {
		if _, err := io.WriteString(t.master, k); err != nil {
			tb.Fatalf("type %q: %v", k, err)
		}
		time.Sleep(40 * time.Millisecond)
	}
}

// WaitRaw waits until the program switched the terminal out of canonical
// mode (for example with term.MakeRaw), so typed keys arrive one at a time.
func (t *Term) WaitRaw(tb testing.TB) {
	tb.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if canon, err := Canonical(t.TTY); err == nil && !canon {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	tb.Fatal("the terminal never left canonical mode")
}

// WaitCanonical waits until the terminal is back in canonical mode.
func (t *Term) WaitCanonical(tb testing.TB) {
	tb.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if canon, err := Canonical(t.TTY); err == nil && canon {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	tb.Fatal("the terminal never returned to canonical mode")
}

// WaitOutput waits until the collected output contains want and returns
// the output. It fails the test after timeout.
func (t *Term) WaitOutput(tb testing.TB, want string, timeout time.Duration) string {
	tb.Helper()
	deadline := time.Now().Add(timeout)
	for {
		out := t.Output()
		if strings.Contains(out, want) {
			return out
		}
		if time.Now().After(deadline) {
			tb.Fatalf("timed out waiting for %q in terminal output:\n%q", want, out)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
