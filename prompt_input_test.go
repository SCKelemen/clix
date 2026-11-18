package clix

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTextPrompterSharesPipedInput(t *testing.T) {
	in := strings.NewReader("one\ntwo\n")
	p := TextPrompter{In: in, Out: &bytes.Buffer{}}
	for _, want := range []string{"one", "two"} {
		got, err := p.Prompt(context.Background(), WithLabel("x"))
		if err != nil || got != want {
			t.Fatalf("Prompt() = %q, %v; want %q", got, err, want)
		}
	}
	if _, err := p.Prompt(context.Background(), WithLabel("x")); !errors.Is(err, io.EOF) {
		t.Fatalf("third Prompt() error = %v, want io.EOF", err)
	}
}

func TestTextPrompterAcceptsFinalLineWithoutNewline(t *testing.T) {
	p := TextPrompter{In: strings.NewReader("val"), Out: &bytes.Buffer{}}
	got, err := p.Prompt(context.Background(), WithLabel("x"))
	if err != nil || got != "val" {
		t.Fatalf("Prompt() = %q, %v; want val", got, err)
	}

	confirm := TextPrompter{In: strings.NewReader("n"), Out: &bytes.Buffer{}}
	if got, err := confirm.Prompt(context.Background(), WithLabel("ok?"), WithConfirm()); err != nil || got != "n" {
		t.Fatalf("confirm Prompt() = %q, %v; want n", got, err)
	}
}

func TestTextPrompterLeavesRestOfInputUnread(t *testing.T) {
	in := strings.NewReader("answer\nrest of stdin\n")
	p := TextPrompter{In: in, Out: &bytes.Buffer{}}
	if _, err := p.Prompt(context.Background(), WithLabel("x")); err != nil {
		t.Fatal(err)
	}
	rest, _ := io.ReadAll(in)
	if string(rest) != "rest of stdin\n" {
		t.Fatalf("prompt consumed more than one line; rest = %q", rest)
	}
}

func TestTwoRequiredFlagsFromPipedInput(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	var a, b string
	cmd := NewCommand("mk", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "a", Required: true}, Value: &a})
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "b", Required: true}, Value: &b})
	app.Root.AddCommand(cmd)
	app.Prompter = TextPrompter{In: strings.NewReader("one\ntwo"), Out: &bytes.Buffer{}}

	if err := app.Run(context.Background(), []string{"mk"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if a != "one" || b != "two" {
		t.Fatalf("a=%q b=%q, want one and two", a, b)
	}
}

func TestTextPrompterHonorsContextOnIdlePipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close(); r.Close() })
	p := TextPrompter{In: r, Out: &bytes.Buffer{}}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := p.Prompt(ctx, WithLabel("x"))
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Prompt() error = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Prompt blocked on an idle pipe despite a context deadline")
	}

	// The read that was in flight hands its line to the next prompt.
	if _, err := w.WriteString("late\n"); err != nil {
		t.Fatal(err)
	}
	got, err := p.Prompt(context.Background(), WithLabel("x"))
	if err != nil || got != "late" {
		t.Fatalf("next Prompt() = %q, %v; want late", got, err)
	}
}

func TestTextPrompterReturnsCanceledContextImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	in := strings.NewReader("unread\n")
	p := TextPrompter{In: in, Out: &bytes.Buffer{}}
	if _, err := p.Prompt(ctx, WithLabel("x")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Prompt() error = %v, want context.Canceled", err)
	}
	if in.Len() != len("unread\n") {
		t.Fatal("a cancelled prompt consumed input")
	}
}
