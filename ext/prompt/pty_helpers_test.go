package prompt_test

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

type result struct {
	val string
	err error
}

// rawEnv makes the environment look like an ordinary color terminal with a
// UTF-8 locale, whatever the test runner has.
func rawEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("LANG", "en_US.UTF-8")
}

// ptyPrompt runs one TerminalPrompter prompt on a pseudo-terminal used as
// both In and Out, like a program attached to a terminal.
type ptyPrompt struct {
	term *ptytest.Term
	ch   chan result
}

func startPTY(t *testing.T, ctx context.Context, opts ...clix.PromptOption) *ptyPrompt {
	t.Helper()
	term := ptytest.New(t)
	return startOn(t, term, ctx, opts...)
}

func startOn(t *testing.T, term *ptytest.Term, ctx context.Context, opts ...clix.PromptOption) *ptyPrompt {
	t.Helper()
	p := &ptyPrompt{term: term, ch: make(chan result, 1)}
	tp := prompt.TerminalPrompter{In: term.TTY, Out: term.TTY}
	go func() {
		v, err := tp.Prompt(ctx, opts...)
		p.ch <- result{v, err}
	}()
	return p
}

// raw starts a prompt and waits until it is in raw mode.
func raw(t *testing.T, opts ...clix.PromptOption) *ptyPrompt {
	t.Helper()
	rawEnv(t)
	p := startPTY(t, context.Background(), opts...)
	p.term.WaitRaw(t)
	return p
}

func (p *ptyPrompt) typeKeys(t *testing.T, keys ...string) { t.Helper(); p.term.Type(t, keys...) }

func (p *ptyPrompt) wait(t *testing.T, d time.Duration) (result, bool) {
	t.Helper()
	select {
	case r := <-p.ch:
		return r, true
	case <-time.After(d):
		return result{}, false
	}
}

func (p *ptyPrompt) mustFinish(t *testing.T) result {
	t.Helper()
	r, ok := p.wait(t, 5*time.Second)
	if !ok {
		t.Fatalf("prompt did not return; screen:\n%s", p.screen())
	}
	return r
}

// screen returns what the 24x80 terminal shows now.
func (p *ptyPrompt) screen() string {
	return ptytest.ScreenText(p.term.Output(), 24, 80)
}

// waitScreen waits until the screen shows want.
func (p *ptyPrompt) waitScreen(t *testing.T, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s := p.screen()
		if strings.Contains(s, want) {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("screen never showed %q:\n%s", want, s)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// checkGoroutines fails the test if goroutines started during it are
// still running shortly after it ends.
func checkGoroutines(t *testing.T) {
	t.Helper()
	before := runtime.NumGoroutine()
	t.Cleanup(func() {
		deadline := time.Now().Add(3 * time.Second)
		for {
			n := runtime.NumGoroutine()
			if n <= before {
				return
			}
			if time.Now().After(deadline) {
				buf := make([]byte, 1<<16)
				buf = buf[:runtime.Stack(buf, true)]
				t.Errorf("goroutines: %d before, %d after\n%s", before, n, buf)
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	})
}

var _ = tty.StripANSI

var envs = []clix.SelectOption{
	{Label: "production-eu", Value: "production-eu"},
	{Label: "production", Value: "production"},
	{Label: "staging", Value: "staging"},
}

// syncBuffer is a goroutine-safe bytes.Buffer.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
