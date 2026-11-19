package prompt_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// After a lone Esc on an input that is read in a goroutine (any reader that
// is not a pollable file), the byte that goroutine reads next goes to the
// next reader, including clix.ReadLine, not into thin air.
func TestReadKeyHandsPendingByteToReadLine(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	go func() { _, _ = pw.Write([]byte{0x1b}) }()
	if k, err := prompt.ReadKey(pr); err != nil || k != prompt.KeyEscape {
		t.Fatalf("ReadKey = %#v, %v", k, err)
	}
	// ReadKey's one-byte read is still waiting; the next line goes through it.
	go func() { _, _ = pw.Write([]byte("hello\n")) }()
	got, err := clix.ReadLine(context.Background(), pr)
	if err != nil || got != "hello" {
		t.Fatalf("ReadLine = %q, %v; want hello", got, err)
	}
}

// The byte a raw prompt read after Enter (to treat CRLF as one key) is not
// lost: the next line read starts with it.
func TestCRLookaheadByteGoesToNextReader(t *testing.T) {
	rawEnv(t)
	term := ptytest.New(t)
	p := startOn(t, term, context.Background(), clix.WithLabel("Name"))
	term.WaitRaw(t)
	if _, err := term.Write([]byte("abc\rX")); err != nil {
		t.Fatal(err)
	}
	if r := p.mustFinish(t); r.val != "abc" {
		t.Fatalf("prompt = %q", r.val)
	}
	term.WaitCanonical(t)
	term.Type(t, "yz\r")
	got, err := clix.ReadLine(context.Background(), term.TTY)
	if err != nil || got != "Xyz" {
		t.Fatalf("ReadLine = %q, %v; want Xyz", got, err)
	}
}

func TestEscCancelThenReadLine(t *testing.T) {
	rawEnv(t)
	term := ptytest.New(t)
	in := highFd(t, term)
	tp := prompt.TerminalPrompter{In: in, Out: term.TTY}
	done := make(chan error, 1)
	go func() {
		_, err := tp.Prompt(context.Background(), clix.WithLabel("Pick"), prompt.Select(envs))
		done <- err
	}()
	term.WaitRaw(t)
	term.Type(t, "\x1b")
	select {
	case err := <-done:
		if !errors.Is(err, clix.ErrCancelled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Esc did not cancel")
	}
	term.WaitCanonical(t)
	term.Type(t, "hello\r")
	got, err := clix.ReadLine(context.Background(), in)
	if err != nil || got != "hello" {
		t.Fatalf("ReadLine = %q, %v; want hello", got, err)
	}
}
