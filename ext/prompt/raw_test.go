package prompt_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

func TestRawTextKeepsNonASCII(t *testing.T) {
	p := raw(t, clix.WithLabel("Name"))
	p.typeKeys(t, "J", "o", "s", "é", " ", "日本", "\r")
	if r := p.mustFinish(t); r.err != nil || r.val != "José 日本" {
		t.Fatalf("got %q, %v; want %q", r.val, r.err, "José 日本")
	}
}

func TestRawTextBackspaceRemovesWholeCharacter(t *testing.T) {
	p := raw(t, clix.WithLabel("Name"), clix.WithDefault("Café"))
	// Tab accepts the default; Backspace removes "é", not one of its bytes.
	p.typeKeys(t, "\t", "\x7f", "e", "\r")
	if r := p.mustFinish(t); r.val != "Cafe" {
		t.Fatalf("got %q, want Cafe", r.val)
	}
}

func TestRawTextBackspaceRemovesGraphemeCluster(t *testing.T) {
	p := raw(t, clix.WithLabel("Name"))
	p.typeKeys(t, "a", "é", "👍🏽", "\x7f", "\x7f", "b", "\r")
	if r := p.mustFinish(t); r.val != "ab" {
		t.Fatalf("got %q, want ab", r.val)
	}
}

func TestRawTextEditingKeys(t *testing.T) {
	p := raw(t, clix.WithLabel("Name"))
	p.typeKeys(t,
		"h", "e", "l", "l", "o",
		"\x1b[3~",          // Delete at the end does nothing
		"\x1b[D", "\x1b[D", // Left twice
		"X",         // hel|X|lo
		"\x01", "[", // Ctrl-A, then insert at the start
		"\x05", "]", // Ctrl-E, then insert at the end
		"\x1b[H", "\x1b[3~", // Home, Delete removes "["
		"\x1bf", " ", "\x1bb", // Alt-f to the end of the word, space, Alt-b back
		"\x1b[1;5C", "!", // Ctrl-Right to the end of the next word
		"\x1bOH", "\x1bOF", "?", // SS3 Home and End (application cursor mode)
		"\r")
	if r := p.mustFinish(t); r.val != "helXlo]! ?" {
		t.Fatalf("got %q, want %q", r.val, "helXlo]! ?")
	}
}

func TestRawTextKillKeys(t *testing.T) {
	p := raw(t, clix.WithLabel("Name"))
	p.typeKeys(t, "a", "b", " ", "c", "d", "\x17", "x", "\x1b[D", "\x0b", "\x01", "\x1b[C", "\x15", "\r")
	// "ab cd" Ctrl-W -> "ab " ; x -> "ab x" ; Left ; Ctrl-K -> "ab " ; Ctrl-A, Right, Ctrl-U -> "b "
	if r := p.mustFinish(t); r.val != "b" {
		t.Fatalf("got %q, want b (trimmed)", r.val)
	}
}

func TestRawTextDeleteKeyIsNotEscape(t *testing.T) {
	p := raw(t, clix.WithLabel("Name"))
	p.typeKeys(t, "h", "e", "l", "l", "o", "\x1b[3~", "\r")
	if r := p.mustFinish(t); r.val != "hello" {
		t.Fatalf("got %q, want hello (Delete read as Esc clears the input)", r.val)
	}
}

func TestRawTextUnknownKeysAreIgnored(t *testing.T) {
	p := raw(t, clix.WithLabel("Name"))
	p.typeKeys(t, "a", "b", "c", "\x1bx", "\x1b[5~", "\x1b[Z", "\x1b[1;2P", "\x1b[I", "\x1bOA", "\r")
	if r := p.mustFinish(t); r.val != "abc" {
		t.Fatalf("got %q, want abc", r.val)
	}
}

func TestRawTextLoneEscClearsAndKeepsNextKey(t *testing.T) {
	p := raw(t, clix.WithLabel("Name"))
	p.typeKeys(t, "a", "b", "\x1b")
	time.Sleep(150 * time.Millisecond)
	p.typeKeys(t, "c", "\r")
	if r := p.mustFinish(t); r.val != "c" {
		t.Fatalf("got %q, want c (Esc clears; the next key must not be swallowed)", r.val)
	}
}

func TestRawCursorUsesDisplayWidth(t *testing.T) {
	p := raw(t, clix.WithLabel("名前"))
	p.waitScreen(t, "? 名前:")
	p.typeKeys(t, "\x03")
	p.mustFinish(t)
	m := regexp.MustCompile("\x1b\\[(\\d+)C").FindStringSubmatch(p.term.Output())
	if m == nil || m[1] != "8" {
		t.Fatalf("cursor moved %v columns; the display width of %q is 8", m, "? 名前: ")
	}
}

func TestRawPastedCRLFIsOneEnter(t *testing.T) {
	rawEnv(t)
	term := ptytest.New(t)
	p := startOn(t, term, context.Background(), clix.WithLabel("First"))
	term.WaitRaw(t)
	// A paste without bracketed paste markers, as from a terminal that
	// does not support them.
	p.typeKeys(t, "abc\r\n")
	if r := p.mustFinish(t); r.val != "abc" {
		t.Fatalf("first = %q", r.val)
	}
	p2 := startOn(t, term, context.Background(), clix.WithLabel("Second"), clix.WithDefault("prod"))
	if r, done := p2.wait(t, 300*time.Millisecond); done {
		t.Fatalf("second prompt answered itself with %q from the pasted LF", r.val)
	}
	p2.typeKeys(t, "\x03")
	p2.mustFinish(t)
}

func TestRawBracketedPasteInsertsFirstLine(t *testing.T) {
	p := raw(t, clix.WithLabel("Name"))
	if !strings.Contains(p.term.Output(), "\x1b[?2004h") {
		t.Fatal("bracketed paste not enabled while prompting")
	}
	// A paste that contains a newline neither submits nor carries on.
	p.typeKeys(t, "x", "\x1b[200~pasted line\rsecond line\x1b[201~")
	if r, done := p.wait(t, 300*time.Millisecond); done {
		t.Fatalf("paste submitted the prompt: %q", r.val)
	}
	p.typeKeys(t, "\r")
	if r := p.mustFinish(t); r.val != "xpasted line" {
		t.Fatalf("got %q, want %q", r.val, "xpasted line")
	}
	if !strings.Contains(p.term.Output(), "\x1b[?2004l") {
		t.Fatal("bracketed paste not disabled afterwards")
	}
}

func TestRawSecretIsNotEchoed(t *testing.T) {
	p := raw(t, clix.WithLabel("API token"), clix.WithSecret())
	p.typeKeys(t, " ", "t", "o", "p", "s", "e", "c", "r", "e", "t", " ", "\r")
	r := p.mustFinish(t)
	if r.val != " topsecret " {
		t.Fatalf("got %q; secrets are returned exactly as typed", r.val)
	}
	if strings.Contains(p.term.Output(), "topsecret") {
		t.Fatal("the secret was echoed")
	}
	if s := p.screen(); !strings.Contains(s, "? API token:") {
		t.Fatalf("screen:\n%s", s)
	}
}

func TestRawSelectPageDownDoesNotCancel(t *testing.T) {
	opts := []clix.SelectOption{{Label: "a", Value: "a"}, {Label: "b", Value: "b"}}
	p := raw(t, clix.PromptRequest{Label: "Pick", Options: opts})
	p.typeKeys(t, "\x1b[6~")
	if r, done := p.wait(t, 300*time.Millisecond); done {
		t.Fatalf("PageDown ended the select: %q, %v", r.val, r.err)
	}
	p.typeKeys(t, "\r")
	if r := p.mustFinish(t); r.val != "b" {
		t.Fatalf("got %q, want b (PageDown moves to the last option)", r.val)
	}
}

func TestRawSelectUnboundFunctionKeyDoesNotCancel(t *testing.T) {
	opts := []clix.SelectOption{{Label: "a", Value: "a"}, {Label: "b", Value: "b"}}
	p := raw(t, clix.PromptRequest{Label: "Pick", Options: opts})
	p.typeKeys(t, "\x1bOP", "\x1b[15~", "\x1b[Z")
	if r, done := p.wait(t, 300*time.Millisecond); done {
		t.Fatalf("an unbound key ended the select: %q, %v", r.val, r.err)
	}
	p.typeKeys(t, "\r")
	if r := p.mustFinish(t); r.val != "a" || r.err != nil {
		t.Fatalf("got %q, %v", r.val, r.err)
	}
}

// A binding that exits without ExitErr cancels: it must not produce an
// empty answer that Validate never saw.
func TestRawKeyBindingExitWithoutErrCancels(t *testing.T) {
	km := clix.PromptKeyMap{Bindings: []clix.PromptKeyBinding{{
		Command: clix.PromptCommand{Type: clix.PromptCommandEscape},
		Handler: func(clix.PromptCommandContext) clix.PromptCommandAction {
			return clix.PromptCommandAction{Exit: true}
		},
	}}}
	notEmpty := func(s string) error {
		if s == "" {
			return errors.New("required")
		}
		return nil
	}
	p := raw(t, clix.WithLabel("Token"), clix.WithValidate(notEmpty), clix.WithKeyMap(km))
	p.typeKeys(t, "\x1b")
	if r := p.mustFinish(t); !errors.Is(r.err, clix.ErrCancelled) {
		t.Fatalf("got (%q, %v), want ErrCancelled", r.val, r.err)
	}
}

func TestRawCtrlCCancelsAndRestoresTerminal(t *testing.T) {
	cases := map[string][]clix.PromptOption{
		"text":         {clix.WithLabel("Name")},
		"secret":       {clix.WithLabel("Token"), clix.WithSecret()},
		"confirm":      {clix.WithLabel("Sure?"), clix.WithConfirm()},
		"select":       {clix.WithLabel("Pick"), prompt.Select(envs)},
		"multi-select": {clix.WithLabel("Pick"), prompt.MultiSelect(envs)},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			checkGoroutines(t)
			p := raw(t, opts...)
			p.typeKeys(t, "\x03")
			if r := p.mustFinish(t); !errors.Is(r.err, clix.ErrCancelled) {
				t.Fatalf("err = %v, want ErrCancelled", r.err)
			}
			p.term.WaitCanonical(t)
			if out := p.term.Output(); !strings.HasSuffix(strings.TrimRight(out, "\r\n"), "\x1b[?2004l") && !strings.Contains(out, "\x1b[?2004l") {
				t.Fatal("bracketed paste left on")
			}
		})
	}
}

func TestRawContextCancelReturnsPromptly(t *testing.T) {
	cases := map[string][]clix.PromptOption{
		"text":         {clix.WithLabel("Name")},
		"confirm":      {clix.WithLabel("Sure?"), clix.WithConfirm()},
		"select":       {clix.WithLabel("Pick"), prompt.Select(envs)},
		"multi-select": {clix.WithLabel("Pick"), prompt.MultiSelect(envs)},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			checkGoroutines(t)
			rawEnv(t)
			ctx, cancel := context.WithCancel(context.Background())
			p := startPTY(t, ctx, opts...)
			p.term.WaitRaw(t)
			cancel() // what signal.NotifyContext does on Ctrl-C
			r, done := p.wait(t, 500*time.Millisecond)
			if !done {
				t.Fatal("prompt ignores ctx cancellation")
			}
			if !errors.Is(r.err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", r.err)
			}
			p.term.WaitCanonical(t)
		})
	}
}

// With a terminal on In but not on Out (app 2>log), raw mode would write
// cursor control into the log: fall back to line mode.
func TestRawNeedsTerminalOutput(t *testing.T) {
	rawEnv(t)
	term := ptytest.New(t)
	out := &syncBuffer{}
	tp := prompt.TerminalPrompter{In: term.TTY, Out: out}
	ch := make(chan result, 1)
	go func() {
		v, err := tp.Prompt(context.Background(), clix.PromptRequest{Label: "Pick", Options: envs})
		ch <- result{v, err}
	}()
	time.Sleep(100 * time.Millisecond)
	if canon, _ := ptytest.Canonical(term.TTY); !canon {
		t.Fatal("raw mode entered although Out is not a terminal")
	}
	term.Type(t, "2\r")
	select {
	case r := <-ch:
		if r.val != "production" {
			t.Fatalf("got %q", r.val)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no answer")
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("escape sequences written to a non-terminal Out: %q", out.String())
	}
}

func TestRawTermDumbUsesLineMode(t *testing.T) {
	rawEnv(t)
	t.Setenv("TERM", "dumb")
	p := startPTY(t, context.Background(), clix.WithLabel("Name"))
	time.Sleep(100 * time.Millisecond)
	if canon, _ := ptytest.Canonical(p.term.TTY); !canon {
		t.Fatal("TERM=dumb must not use raw mode")
	}
	p.typeKeys(t, "bob\r")
	if r := p.mustFinish(t); r.val != "bob" {
		t.Fatalf("got %q", r.val)
	}
	if strings.Contains(p.term.Output(), "\x1b[") {
		t.Fatalf("escape sequences written with TERM=dumb: %q", p.term.Output())
	}
}

func TestRawNoColorKeepsCursorControlOnly(t *testing.T) {
	rawEnv(t)
	t.Setenv("NO_COLOR", "1")
	red := clix.StyleFunc(func(s ...string) string { return "\x1b[31m" + strings.Join(s, "") + "\x1b[0m" })
	p := startPTY(t, context.Background(), clix.WithLabel("Name"), clix.WithTheme(clix.PromptTheme{LabelStyle: red}))
	p.term.WaitRaw(t)
	p.typeKeys(t, "x", "\r")
	p.mustFinish(t)
	if strings.Contains(p.term.Output(), "\x1b[31m") {
		t.Fatal("color written with NO_COLOR set")
	}
	if !strings.Contains(p.screen(), "? Name: x") {
		t.Fatalf("screen:\n%s", p.screen())
	}
}

func longOptions(n int) []clix.SelectOption {
	opts := make([]clix.SelectOption, n)
	for i := range opts {
		opts[i] = clix.SelectOption{Label: fmt.Sprintf("item %02d", i+1), Value: fmt.Sprintf("v%02d", i+1)}
	}
	return opts
}

// A list taller than the terminal scrolls in a window; the redraw never
// leaves stray rows on the 24-line screen.
func TestRawLongListScrolls(t *testing.T) {
	p := raw(t, clix.PromptRequest{Label: "Pick", Options: longOptions(60)})
	s := p.waitScreen(t, "more")
	if lines := strings.Count(s, "\n") + 1; lines > 24 {
		t.Fatalf("frame taller than the terminal:\n%s", s)
	}
	if !strings.Contains(s, "item 01") || strings.Contains(s, "item 60") {
		t.Fatalf("first window:\n%s", s)
	}
	p.typeKeys(t, "\x1b[6~", "\x1b[6~", "\x1b[F") // PageDown twice, then End
	s = p.waitScreen(t, "> item 60")
	if strings.Contains(s, "item 01") || strings.Count(s, "? Pick") != 1 {
		t.Fatalf("after End:\n%s", s)
	}
	p.typeKeys(t, "\x1b[A", "\r")
	if r := p.mustFinish(t); r.val != "v59" {
		t.Fatalf("got %q, want v59", r.val)
	}
	if s := p.screen(); s != "? Pick: item 59" {
		t.Fatalf("screen after answering:\n%q", s)
	}
}

func TestRawListRedrawsOnResize(t *testing.T) {
	p := raw(t, clix.PromptRequest{Label: "Pick", Options: longOptions(20)})
	p.waitScreen(t, "item 20")
	if err := ptytest.SetSize(p.term.TTY, 10, 80); err != nil {
		t.Skip(err)
	}
	// SIGWINCH goes to the terminal's foreground process group; the test
	// process is not attached to this pty, so deliver it directly.
	sendWinch(t)
	deadline := time.Now().Add(3 * time.Second)
	for {
		s := ptytest.ScreenText(p.term.Output(), 10, 80)
		if strings.Contains(s, "more") && !strings.Contains(s, "item 20") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no redraw for 10 rows:\n%s", s)
		}
		time.Sleep(20 * time.Millisecond)
	}
	p.typeKeys(t, "\r")
	if r := p.mustFinish(t); r.val != "v01" {
		t.Fatalf("got %q", r.val)
	}
}

func TestRawNonUTF8LocaleUsesASCII(t *testing.T) {
	rawEnv(t)
	t.Setenv("LANG", "C")
	p := startPTY(t, context.Background(), clix.PromptRequest{Label: "Pick", Options: longOptions(40)})
	p.term.WaitRaw(t)
	s := p.waitScreen(t, "more")
	if strings.ContainsAny(s, "↑↓…") || !strings.Contains(s, "v 20 more") {
		t.Fatalf("non-ASCII symbols in a C locale:\n%s", s)
	}
	p.typeKeys(t, "\x03")
	p.mustFinish(t)
}

func TestRawSelectRendering(t *testing.T) {
	opts := []clix.SelectOption{
		{Label: "Alpha", Value: "a", Description: "first"},
		{Label: "Beta", Value: "b"},
	}
	p := raw(t, clix.PromptRequest{Label: "Pick", Options: opts, Default: "b"})
	s := p.waitScreen(t, "> Beta")
	want := "? Pick\n  Alpha - first\n> Beta"
	if s != want {
		t.Fatalf("screen:\n%q\nwant:\n%q", s, want)
	}
	p.typeKeys(t, "k", "\r") // k moves up
	if r := p.mustFinish(t); r.val != "a" {
		t.Fatalf("got %q", r.val)
	}
	if s := p.screen(); s != "? Pick: Alpha" {
		t.Fatalf("after: %q", s)
	}
}
