package prompt_test

// Conformance suite for the prompt options documented in the README
// ("Core options" and "Advanced options"), in every mode:
//
//   - text: clix.TextPrompter reading a pipe (the core default)
//   - line: prompt.TerminalPrompter with In not a terminal
//   - dumb: prompt.TerminalPrompter on a terminal with TERM=dumb
//   - raw:  prompt.TerminalPrompter on a terminal (pseudo-terminal)
//
// Line modes take the typed input as lines; raw mode takes keystrokes.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

var lineModes = []string{"text", "line", "dumb"}

// runLine runs one prompt in a line mode with the given input lines and
// returns the answer and everything the prompt wrote. In dumb mode the
// output also holds the terminal's echo of the typed lines.
func runLine(t *testing.T, mode, input string, opts ...clix.PromptOption) (string, error, string) {
	t.Helper()
	switch mode {
	case "text", "line":
		out := &bytes.Buffer{}
		var p clix.Prompter = clix.TextPrompter{In: strings.NewReader(input), Out: out}
		if mode == "line" {
			p = prompt.TerminalPrompter{In: strings.NewReader(input), Out: out}
		}
		v, err := p.Prompt(context.Background(), opts...)
		return v, err, out.String()
	case "dumb":
		rawEnv(t)
		t.Setenv("TERM", "dumb")
		term := ptytest.New(t)
		p := startOn(t, term, context.Background(), opts...)
		// Type the lines into the cooked terminal, then end the input.
		time.Sleep(50 * time.Millisecond)
		if input != "" {
			if _, err := term.Write([]byte(strings.ReplaceAll(input, "\n", "\r"))); err != nil {
				t.Fatal(err)
			}
		}
		r, ok := p.wait(t, 3*time.Second)
		if !ok {
			term.Write([]byte{4}) // Ctrl-D: end of input
			r = p.mustFinish(t)
		}
		// Let the last output drain from the terminal before reading it.
		out := term.Output()
		for i := 0; i < 20; i++ {
			time.Sleep(20 * time.Millisecond)
			if next := term.Output(); next != out {
				out = next
				i = 0
				continue
			}
			break
		}
		return r.val, r.err, out
	}
	t.Fatalf("unknown mode %q", mode)
	return "", nil, ""
}

// runRaw runs one prompt in raw mode, types keys, and returns the answer
// and the final screen.
func runRaw(t *testing.T, keys []string, opts ...clix.PromptOption) (string, error, string) {
	t.Helper()
	p := raw(t, opts...)
	p.typeKeys(t, keys...)
	r := p.mustFinish(t)
	return r.val, r.err, p.screen()
}

func lowercase(s string) error {
	if strings.ToLower(s) != s {
		return errors.New("must be lowercase")
	}
	return nil
}

// --- WithLabel, WithDefault, WithNoDefaultPlaceholder ---

func TestConformanceLabelRendering(t *testing.T) {
	for _, mode := range lineModes {
		t.Run(mode, func(t *testing.T) {
			v, err, out := runLine(t, mode, "bob\n", clix.WithLabel("Name"))
			if err != nil || v != "bob" || !strings.HasPrefix(out, "? Name: ") {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
		})
	}
	t.Run("raw", func(t *testing.T) {
		v, err, screen := runRaw(t, []string{"b", "o", "b", "\r"}, clix.WithLabel("Name"))
		if err != nil || v != "bob" || screen != "? Name: bob" {
			t.Fatalf("got %q, %v, screen %q", v, err, screen)
		}
	})
}

func TestConformanceDefault(t *testing.T) {
	for _, mode := range lineModes {
		t.Run(mode, func(t *testing.T) {
			v, err, out := runLine(t, mode, "\n", clix.WithLabel("Name"), clix.WithDefault("bob"))
			if err != nil || v != "bob" || !strings.HasPrefix(out, "? Name [bob]: ") {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
			// A default is returned exactly, spaces included.
			if v, _, _ := runLine(t, mode, "\n", clix.WithLabel("Name"), clix.WithDefault(" b ")); v != " b " {
				t.Fatalf("default changed to %q", v)
			}
		})
	}
	t.Run("raw", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Name"), clix.WithDefault("bob"))
		p.waitScreen(t, "? Name: bob") // shown as ghost text after the cursor
		p.typeKeys(t, "\r")
		if r := p.mustFinish(t); r.err != nil || r.val != "bob" {
			t.Fatalf("got %q, %v", r.val, r.err)
		}
		// Typing replaces the default; Tab accepts it.
		if v, _, _ := runRaw(t, []string{"b", "\t", "\r"}, clix.WithLabel("Name"), clix.WithDefault("bob")); v != "bob" {
			t.Fatalf("Tab: %q", v)
		}
		if v, _, _ := runRaw(t, []string{"x", "\r"}, clix.WithLabel("Name"), clix.WithDefault("bob")); v != "x" {
			t.Fatalf("typed: %q", v)
		}
	})
}

func TestConformancePlaceholder(t *testing.T) {
	for _, mode := range lineModes {
		t.Run(mode, func(t *testing.T) {
			v, err, out := runLine(t, mode, "\n", clix.WithLabel("Name"), clix.WithNoDefaultPlaceholder("optional"))
			if err != nil || v != "" || !strings.HasPrefix(out, "? Name [optional]: ") {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
			_, _, out = runLine(t, mode, "\n", clix.WithLabel("Name"), clix.WithNoDefaultPlaceholder("optional"), clix.WithDefault("bob"))
			if strings.Contains(out, "optional") {
				t.Fatalf("placeholder shown with a default: %q", out)
			}
		})
	}
	t.Run("raw", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Name"), clix.WithNoDefaultPlaceholder("optional"))
		p.waitScreen(t, "? Name: optional")
		p.typeKeys(t, "\r")
		if r := p.mustFinish(t); r.err != nil || r.val != "" {
			t.Fatalf("got %q, %v; the placeholder is never an answer", r.val, r.err)
		}
	})
}

func TestConformanceSecretDefaultNotTrimmedOrShown(t *testing.T) {
	for _, mode := range lineModes {
		t.Run(mode, func(t *testing.T) {
			v, err, out := runLine(t, mode, "\n", clix.WithLabel("Token"), clix.WithSecret(), clix.WithDefault("  keep  "))
			if err != nil || v != "  keep  " || strings.Contains(out, "keep") {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
		})
	}
	t.Run("raw", func(t *testing.T) {
		v, err, screen := runRaw(t, []string{"\r"}, clix.WithLabel("Token"), clix.WithSecret(), clix.WithDefault("  keep  "))
		if err != nil || v != "  keep  " || strings.Contains(screen, "keep") {
			t.Fatalf("got %q, %v, screen %q", v, err, screen)
		}
	})
}

// --- WithValidate ---

func TestConformanceValidateReprompts(t *testing.T) {
	for _, mode := range lineModes {
		t.Run(mode, func(t *testing.T) {
			v, err, out := runLine(t, mode, "BAD\ngood\n", clix.WithLabel("Name"), clix.WithValidate(lowercase))
			if err != nil || v != "good" || !strings.Contains(out, "! must be lowercase") {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
		})
	}
	t.Run("raw", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Name"), clix.WithValidate(lowercase))
		p.typeKeys(t, "B", "A", "D", "\r")
		p.waitScreen(t, "! must be lowercase")
		p.typeKeys(t, "\x15", "g", "o", "o", "d", "\r")
		if r := p.mustFinish(t); r.err != nil || r.val != "good" {
			t.Fatalf("got %q, %v", r.val, r.err)
		}
		if s := p.screen(); strings.Contains(s, "must be lowercase") {
			t.Fatalf("error left on screen:\n%s", s)
		}
	})
}

// On a pipe, input that ends after an invalid answer returns the
// validation error instead of looping or a bare EOF.
func TestConformanceValidateEndOfInput(t *testing.T) {
	for _, mode := range []string{"text", "line"} {
		t.Run(mode, func(t *testing.T) {
			_, err, _ := runLine(t, mode, "BAD\n", clix.WithLabel("Name"), clix.WithValidate(lowercase))
			if err == nil || !strings.Contains(err.Error(), "must be lowercase") || !errors.Is(err, io.EOF) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestConformanceValidateRunsOnDefault(t *testing.T) {
	for _, mode := range lineModes {
		t.Run(mode, func(t *testing.T) {
			v, err, out := runLine(t, mode, "\nok\n", clix.WithLabel("Name"), clix.WithDefault("BAD"), clix.WithValidate(lowercase))
			if err != nil || v != "ok" || !strings.Contains(out, "must be lowercase") {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
		})
	}
	t.Run("raw", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Name"), clix.WithDefault("BAD"), clix.WithValidate(lowercase))
		p.typeKeys(t, "\r")
		p.waitScreen(t, "must be lowercase")
		p.typeKeys(t, "o", "k", "\r")
		if r := p.mustFinish(t); r.val != "ok" {
			t.Fatalf("got %q", r.val)
		}
	})
}

func notValue(bad string) func(string) error {
	return func(s string) error {
		if s == bad {
			return errors.New("not " + bad)
		}
		return nil
	}
}

func TestConformanceValidateChoices(t *testing.T) {
	two := []clix.SelectOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}
	for _, mode := range []string{"line", "dumb"} {
		t.Run(mode+"/select", func(t *testing.T) {
			v, err, out := runLine(t, mode, "1\n2\n", clix.WithLabel("Pick"), prompt.Select(two), clix.WithValidate(notValue("a")))
			if err != nil || v != "b" || !strings.Contains(out, "! not a") {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
		})
		t.Run(mode+"/multi-select", func(t *testing.T) {
			v, err, out := runLine(t, mode, "1\ndone\n1,2\ndone\n", clix.WithLabel("Pick"), prompt.MultiSelect(two), clix.WithValidate(notValue("a")))
			if err != nil || v != "b" || !strings.Contains(out, "! not a") {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
		})
	}
	for _, mode := range lineModes {
		t.Run(mode+"/confirm", func(t *testing.T) {
			v, err, out := runLine(t, mode, "n\ny\n", clix.WithLabel("Accept?"), clix.WithConfirm(), clix.WithValidate(notValue("n")))
			if err != nil || v != "y" || !strings.Contains(out, "! not n") {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
		})
	}
	t.Run("raw/select", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Pick"), prompt.Select(two), clix.WithValidate(notValue("a")))
		p.typeKeys(t, "\r")
		p.waitScreen(t, "! not a")
		p.typeKeys(t, "j", "\r")
		if r := p.mustFinish(t); r.val != "b" {
			t.Fatalf("got %q", r.val)
		}
	})
	t.Run("raw/multi-select", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Pick"), prompt.MultiSelect(two), clix.WithValidate(notValue("a")))
		p.typeKeys(t, " ", "\x1b[F", "\r") // select a, go to Continue, finish
		p.waitScreen(t, "! not a")
		p.typeKeys(t, "1", "2", "\x1b[F", "\r") // toggle a off and b on, back to Continue
		if r := p.mustFinish(t); r.val != "b" {
			t.Fatalf("got %q", r.val)
		}
	})
	t.Run("raw/confirm", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Accept?"), clix.WithConfirm(), clix.WithValidate(notValue("n")))
		p.typeKeys(t, "n", "\r")
		p.waitScreen(t, "! not n")
		p.typeKeys(t, "\x15", "y", "\r")
		if r := p.mustFinish(t); r.val != "y" {
			t.Fatalf("got %q", r.val)
		}
	})
}

// --- WithTheme ---

func marker(tag string) clix.TextStyle {
	return clix.StyleFunc(func(s ...string) string { return "<" + tag + ">" + strings.Join(s, "") + "</" + tag + ">" })
}

func TestConformanceThemeStyles(t *testing.T) {
	theme := clix.PromptTheme{
		PrefixStyle: marker("p"),
		LabelStyle:  marker("l"),
		ErrorStyle:  marker("e"),
		HintStyle:   marker("h"),
		Hint:        "hint",
	}
	for _, mode := range []string{"text", "line"} {
		t.Run(mode, func(t *testing.T) {
			_, _, out := runLine(t, mode, "BAD\nok\n", clix.WithLabel("Name"), clix.WithTheme(theme), clix.WithValidate(lowercase))
			for _, want := range []string{"<p>? </p><l>Name</l> <h>hint</h>: ", "<e>! </e><e>must be lowercase</e>"} {
				if !strings.Contains(out, want) {
					t.Fatalf("output %q lacks %q", out, want)
				}
			}
		})
	}
	t.Run("raw", func(t *testing.T) {
		buttons := clix.PromptButtonStyles{Active: marker("b")}
		km := clix.PromptKeyMap{Bindings: []clix.PromptKeyBinding{{Command: clix.PromptCommand{Type: clix.PromptCommandFunction, FunctionKey: 1}, Description: "Help"}}}
		th := theme
		th.Buttons = buttons
		p := raw(t, clix.WithLabel("Name"), clix.WithTheme(th), clix.WithKeyMap(km))
		s := p.waitScreen(t, "<b>[ F1 ] Help</b>")
		if !strings.Contains(s, "<p>? </p><l>Name</l> <h>hint</h>:") {
			t.Fatalf("screen:\n%s", s)
		}
		p.typeKeys(t, "x", "\r")
		p.mustFinish(t)
	})
}

// A partial theme merges field by field over App.DefaultTheme, in prompts
// made from command code, for both built-in prompters.
func TestConformanceThemeMergesOverAppDefault(t *testing.T) {
	for _, withExt := range []bool{false, true} {
		name := "TextPrompter"
		if withExt {
			name = "TerminalPrompter"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			errOut, out := &bytes.Buffer{}, &bytes.Buffer{}
			app := clix.NewApp("probe", clix.WithAppIn(strings.NewReader("x\n")), clix.WithAppErr(errOut), clix.WithAppOut(out))
			app.DefaultTheme = clix.PromptTheme{Prefix: ">> ", Error: "E: "}
			if withExt {
				app.AddExtension(prompt.Extension{})
			}
			app.Root.AddCommand(clix.NewCommand("ask", clix.WithCommandRun(func(ctx *clix.Context) error {
				_, err := ctx.App.Prompter.Prompt(ctx, clix.WithLabel("Name"), clix.WithTheme(clix.PromptTheme{Hint: "(lowercase)"}))
				return err
			})))
			if err := app.Run(context.Background(), []string{"ask"}); err != nil {
				t.Fatal(err)
			}
			if errOut.String() != ">> Name (lowercase): " {
				t.Fatalf("prompt output %q", errOut.String())
			}
			if out.Len() != 0 {
				t.Fatalf("prompt wrote to App.Out: %q", out.String())
			}
		})
	}
}

func TestConformanceThemeNoANSIWithoutColor(t *testing.T) {
	red := clix.StyleFunc(func(s ...string) string { return "\x1b[31m" + strings.Join(s, "") + "\x1b[0m" })
	t.Setenv("CLICOLOR_FORCE", "")
	for _, mode := range []string{"text", "line"} {
		t.Run(mode+"/pipe", func(t *testing.T) {
			_, _, out := runLine(t, mode, "x\n", clix.WithLabel("Name"), clix.WithTheme(clix.PromptTheme{LabelStyle: red}))
			if strings.Contains(out, "\x1b") || out != "? Name: " {
				t.Fatalf("output %q", out)
			}
		})
	}
	t.Run("raw/NO_COLOR", func(t *testing.T) {
		rawEnv(t)
		t.Setenv("NO_COLOR", "1")
		p := startPTY(t, context.Background(), clix.WithLabel("Name"), clix.WithTheme(clix.PromptTheme{LabelStyle: red}))
		p.term.WaitRaw(t)
		p.typeKeys(t, "\r")
		p.mustFinish(t)
		if strings.Contains(p.term.Output(), "\x1b[31m") {
			t.Fatal("color with NO_COLOR")
		}
	})
	t.Run("raw/color", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Name"), clix.WithTheme(clix.PromptTheme{LabelStyle: red}))
		p.typeKeys(t, "\r")
		p.mustFinish(t)
		if !strings.Contains(p.term.Output(), "\x1b[31mName") {
			t.Fatal("no color on a color terminal")
		}
	})
}

// --- WithConfirm / prompt.Confirm ---

func TestConformanceConfirmAnswers(t *testing.T) {
	answers := map[string]string{"y": "y", "Y": "y", "yes": "y", "YES": "y", "Yes": "y", "n": "n", "N": "n", "no": "n", "NO": "n"}
	for _, mode := range lineModes {
		for in, want := range answers {
			for name, opt := range map[string]clix.PromptOption{"WithConfirm": clix.WithConfirm(), "prompt.Confirm": prompt.Confirm()} {
				if mode == "dumb" && (in != "Yes" || name != "WithConfirm") {
					continue // the terminal case is covered once; it is slow
				}
				v, err, out := runLine(t, mode, in+"\n", clix.WithLabel("Sure?"), opt)
				if err != nil || v != want || !strings.HasPrefix(out, "? Sure? (Y/n): ") {
					t.Errorf("%s %s %q: got %q, %v, output %q", mode, name, in, v, err, out)
				}
			}
		}
	}
	t.Run("raw", func(t *testing.T) {
		for in, want := range map[string]string{"YES": "y", "no": "n"} {
			keys := append(strings.Split(in, ""), "\r")
			v, err, screen := runRaw(t, keys, clix.WithLabel("Sure?"), prompt.Confirm())
			if err != nil || v != want {
				t.Errorf("%q: got %q, %v", in, v, err)
			}
			if wantScreen := "? Sure? (Y/n): " + map[string]string{"y": "yes", "n": "no"}[want]; screen != wantScreen {
				t.Errorf("screen %q, want %q", screen, wantScreen)
			}
		}
	})
}

func TestConformanceConfirmDefault(t *testing.T) {
	for _, mode := range []string{"text", "line"} {
		for def, want := range map[string]string{"": "y", "y": "y", "true": "y", "n": "n", "no": "n", "false": "n", "0": "n", "off": "n", "OFF": "n"} {
			v, err, out := runLine(t, mode, "\n", clix.WithLabel("Sure?"), clix.WithConfirm(), clix.WithDefault(def))
			hint := "(Y/n)"
			if want == "n" {
				hint = "(y/N)"
			}
			if err != nil || v != want || !strings.Contains(out, hint) {
				t.Errorf("%s Default %q: got %q, %v, output %q", mode, def, v, err, out)
			}
		}
	}
	t.Run("raw", func(t *testing.T) {
		v, _, screen := runRaw(t, []string{"\r"}, clix.WithLabel("Sure?"), clix.WithConfirm(), clix.WithDefault("false"))
		if v != "n" || screen != "? Sure? (y/N): no" {
			t.Fatalf("got %q, screen %q", v, screen)
		}
	})
}

func TestConformanceConfirmGarbageAndBack(t *testing.T) {
	for _, mode := range lineModes {
		t.Run(mode, func(t *testing.T) {
			v, err, out := runLine(t, mode, "maybe\nback\nn\n", clix.WithLabel("Sure?"), clix.WithConfirm())
			if err != nil || v != "n" || strings.Count(out, "please enter 'y' or 'n'") != 2 {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
		})
	}
	for _, mode := range []string{"text", "line"} {
		t.Run(mode+"/pipe ends", func(t *testing.T) {
			v, err, _ := runLine(t, mode, "back\n", clix.WithLabel("Sure?"), clix.WithConfirm())
			if err == nil || v != "" || !strings.Contains(err.Error(), "please enter 'y' or 'n'") {
				t.Fatalf("got %q, %v", v, err)
			}
		})
	}
	t.Run("raw", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Sure?"), clix.WithConfirm())
		p.typeKeys(t, "b", "a", "c", "k", "\r")
		p.waitScreen(t, "please enter 'y' or 'n'")
		p.typeKeys(t, "\x15", "y", "\r")
		if r := p.mustFinish(t); r.val != "y" {
			t.Fatalf("got %q", r.val)
		}
	})
}

// --- WithCommandHandler / WithKeyMap ---

// In raw mode Esc, Tab, Enter and F1-F12 reach bindings and handlers.
func TestConformanceBindableKeys(t *testing.T) {
	keys := map[string]clix.PromptCommand{
		"\x1b":     {Type: clix.PromptCommandEscape},
		"\t":       {Type: clix.PromptCommandTab},
		"\r":       {Type: clix.PromptCommandEnter},
		"\x1bOP":   {Type: clix.PromptCommandFunction, FunctionKey: 1},
		"\x1b[[B":  {Type: clix.PromptCommandFunction, FunctionKey: 2},
		"\x1b[15~": {Type: clix.PromptCommandFunction, FunctionKey: 5},
		"\x1b[24~": {Type: clix.PromptCommandFunction, FunctionKey: 12},
	}
	for seq, want := range keys {
		var got clix.PromptCommand
		handler := func(ctx clix.PromptCommandContext) clix.PromptCommandAction {
			got = ctx.Command
			return clix.PromptCommandAction{Exit: true, ExitErr: errors.New("bound")}
		}
		_, err, _ := runRaw(t, []string{seq}, clix.WithLabel("Name"), clix.WithCommandHandler(handler))
		if err == nil || err.Error() != "bound" || got != want {
			t.Errorf("%q: command %+v, err %v; want %+v", seq, got, err, want)
		}
	}
}

func TestConformanceKeyBindingSetsInput(t *testing.T) {
	km := clix.PromptKeyMap{Bindings: []clix.PromptKeyBinding{{
		Command:     clix.PromptCommand{Type: clix.PromptCommandFunction, FunctionKey: 2},
		Description: "Fill",
		Handler: func(ctx clix.PromptCommandContext) clix.PromptCommandAction {
			ctx.SetInput(ctx.Input + "-filled")
			return clix.PromptCommandAction{Handled: true}
		},
	}}}
	p := raw(t, clix.WithLabel("Name"), clix.WithKeyMap(km))
	p.waitScreen(t, "[ F2 ] Fill")
	p.typeKeys(t, "a", "\x1bOQ", "\r")
	if r := p.mustFinish(t); r.val != "a-filled" {
		t.Fatalf("got %q", r.val)
	}
}

// Ctrl-C cannot be bound: it cancels even when a handler claims every key.
func TestConformanceBindingsCannotBreakCtrlC(t *testing.T) {
	greedy := func(clix.PromptCommandContext) clix.PromptCommandAction {
		return clix.PromptCommandAction{Handled: true}
	}
	for name, opts := range map[string][]clix.PromptOption{
		"text":   {clix.WithLabel("Name")},
		"select": {clix.WithLabel("Pick"), prompt.Select(envs)},
	} {
		t.Run(name, func(t *testing.T) {
			_, err, _ := runRaw(t, []string{"\x03"}, append(opts, clix.WithCommandHandler(greedy))...)
			if !errors.Is(err, clix.ErrCancelled) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// The first binding for a command wins; an inactive binding still
// consumes the key; the command handler runs only when no binding handled
// the key.
func TestConformanceConflictingBindings(t *testing.T) {
	var calls []string
	bind := func(name string, active bool, action clix.PromptCommandAction) clix.PromptKeyBinding {
		return clix.PromptKeyBinding{
			Command: clix.PromptCommand{Type: clix.PromptCommandTab},
			Active:  func(clix.PromptKeyState) bool { return active },
			Handler: func(clix.PromptCommandContext) clix.PromptCommandAction {
				calls = append(calls, name)
				return action
			},
		}
	}
	handler := clix.WithCommandHandler(func(ctx clix.PromptCommandContext) clix.PromptCommandAction {
		calls = append(calls, "handler")
		return clix.PromptCommandAction{}
	})
	// first active binding handles: second binding and handler not called
	km := clix.PromptKeyMap{Bindings: []clix.PromptKeyBinding{
		bind("first", true, clix.PromptCommandAction{Handled: true}),
		bind("second", true, clix.PromptCommandAction{Handled: true}),
	}}
	runRaw(t, []string{"\t", "\r"}, clix.WithLabel("Name"), clix.WithKeyMap(km), handler)
	// Enter has no binding, so it reaches the handler (which does not
	// handle it, so the prompt submits).
	if strings.Join(calls, ",") != "first,handler" {
		t.Fatalf("calls = %q", calls)
	}
	// inactive first binding consumes the key
	calls = nil
	km = clix.PromptKeyMap{Bindings: []clix.PromptKeyBinding{
		bind("first", false, clix.PromptCommandAction{Handled: true}),
		bind("second", true, clix.PromptCommandAction{Handled: true}),
	}}
	runRaw(t, []string{"\t", "\r"}, clix.WithLabel("Name"), clix.WithKeyMap(km), handler)
	if strings.Join(calls, ",") != "handler" {
		t.Fatalf("calls = %q", calls)
	}
	// a binding that does not handle passes the key to the handler
	calls = nil
	km = clix.PromptKeyMap{Bindings: []clix.PromptKeyBinding{bind("first", true, clix.PromptCommandAction{})}}
	runRaw(t, []string{"\t", "\r"}, clix.WithLabel("Name"), clix.WithKeyMap(km), handler)
	if strings.Join(calls, ",") != "first,handler,handler" {
		t.Fatalf("calls = %q", calls)
	}
}

// Line modes have no keys to bind: every line is an answer, including "?"
// and "back", and neither bindings nor the handler run.
func TestConformanceLineModeIgnoresBindings(t *testing.T) {
	called := false
	handler := func(clix.PromptCommandContext) clix.PromptCommandAction {
		called = true
		return clix.PromptCommandAction{Exit: true}
	}
	km := clix.PromptKeyMap{Bindings: []clix.PromptKeyBinding{{Command: clix.PromptCommand{Type: clix.PromptCommandEscape}, Handler: handler}}}
	for _, mode := range lineModes {
		for _, typed := range []string{"?", "back"} {
			v, err, _ := runLine(t, mode, typed+"\n", clix.WithLabel("Name"), clix.WithKeyMap(km), clix.WithCommandHandler(handler))
			if err != nil || v != typed || called {
				t.Errorf("%s %q: got %q, %v, handler called %v", mode, typed, v, err, called)
			}
		}
	}
}

// --- prompt.Select ---

func TestConformanceSelectNavigation(t *testing.T) {
	opts := longOptions(5)
	cases := []struct {
		name string
		keys []string
		want string
	}{
		{"down", []string{"\x1b[B", "\r"}, "v02"},
		{"j", []string{"j", "j", "\r"}, "v03"},
		{"k wraps to the end", []string{"k", "\r"}, "v05"},
		{"up wraps", []string{"\x1b[A", "\r"}, "v05"},
		{"down wraps", []string{"\x1b[F", "\x1b[B", "\r"}, "v01"},
		{"End", []string{"\x1b[4~", "\r"}, "v05"},
		{"Home", []string{"\x1b[F", "\x1b[H", "\r"}, "v01"},
		{"PgDn", []string{"\x1b[6~", "\r"}, "v05"},
		{"PgUp", []string{"\x1b[F", "\x1b[5~", "\r"}, "v01"},
		{"app cursor down", []string{"\x1bOB", "\r"}, "v02"},
		{"Ctrl-N", []string{"\x0e", "\r"}, "v02"},
		{"number", []string{"4"}, "v04"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, err, _ := runRaw(t, c.keys, clix.WithLabel("Pick"), prompt.Select(opts))
			if err != nil || v != c.want {
				t.Fatalf("got %q, %v; want %q", v, err, c.want)
			}
		})
	}
}

func TestConformanceSelectDefault(t *testing.T) {
	for _, mode := range []string{"line", "dumb"} {
		t.Run(mode, func(t *testing.T) {
			v, err, out := runLine(t, mode, "\n", clix.WithLabel("Env"), prompt.Select(envs), clix.WithDefault("staging"))
			if err != nil || v != "staging" || !strings.Contains(out, "> staging") {
				t.Fatalf("got %q, %v, output %q", v, err, out)
			}
		})
	}
	t.Run("raw", func(t *testing.T) {
		v, _, _ := runRaw(t, []string{"\r"}, clix.WithLabel("Env"), prompt.Select(envs), clix.WithDefault("production"))
		if v != "production" {
			t.Fatalf("got %q", v)
		}
	})
	// A default that is not an option is an error, in every mode.
	for _, mode := range []string{"line", "dumb"} {
		if _, err, _ := runLine(t, mode, "\n", clix.WithLabel("Env"), prompt.Select(envs), clix.WithDefault("dev")); !errors.Is(err, clix.ErrInvalidDefault) {
			t.Errorf("%s: err = %v", mode, err)
		}
	}
	rawEnv(t)
	p := startPTY(t, context.Background(), clix.WithLabel("Env"), prompt.Select(envs), clix.WithDefault("dev"))
	if r := p.mustFinish(t); !errors.Is(r.err, clix.ErrInvalidDefault) {
		t.Errorf("raw: err = %v", r.err)
	}
}

func TestConformanceEmptyOptions(t *testing.T) {
	for name, opt := range map[string]clix.PromptOption{
		"Select(nil)":      prompt.Select(nil),
		"Select(empty)":    prompt.Select([]clix.SelectOption{}),
		"MultiSelect(nil)": prompt.MultiSelect(nil),
		"struct":           clix.PromptRequest{Options: []clix.SelectOption{}},
	} {
		for _, mode := range []string{"text", "line"} {
			if _, err, _ := runLine(t, mode, "anything\n", clix.WithLabel("Region"), opt); !errors.Is(err, clix.ErrNoOptions) {
				t.Errorf("%s %s: err = %v", mode, name, err)
			}
		}
		rawEnv(t)
		p := startPTY(t, context.Background(), clix.WithLabel("Region"), opt)
		if r := p.mustFinish(t); !errors.Is(r.err, clix.ErrNoOptions) {
			t.Errorf("raw %s: err = %v", name, r.err)
		}
	}
}

func TestConformanceSelectLineMatching(t *testing.T) {
	cases := []struct {
		input, want, msg string
	}{
		{"production\n", "production", ""},       // exact beats an earlier prefix match
		{"PRODUCTION-EU\n", "production-eu", ""}, // case-insensitive exact
		{"3\n", "staging", ""},                   // number
		{"stag\n", "staging", ""},                // unique prefix
		{"3rd-party\nstaging\n", "staging", "matches no option"},
		{"prod\n2\n", "production", "matches more than one option"},
		{"9\n1\n", "production-eu", "no option 9"},
	}
	for _, mode := range []string{"line", "dumb"} {
		for _, c := range cases {
			v, err, out := runLine(t, mode, c.input, clix.WithLabel("Env"), prompt.Select(envs))
			if err != nil || v != c.want || (c.msg != "" && !strings.Contains(out, c.msg)) {
				t.Errorf("%s %q: got %q, %v, output %q", mode, c.input, v, err, out)
			}
		}
	}
	// TextPrompter has no select prompt.
	if _, err, _ := runLine(t, "text", "1\n", clix.WithLabel("Env"), prompt.Select(envs)); err == nil || !strings.Contains(err.Error(), "prompt extension") {
		t.Errorf("TextPrompter select: %v", err)
	}
}

// --- prompt.MultiSelect and prompt.WithContinueText ---

func TestConformanceMultiSelectRaw(t *testing.T) {
	t.Run("space toggles, continue finishes", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Envs"), prompt.MultiSelect(envs), prompt.WithContinueText("Deploy"))
		s := p.waitScreen(t, "  Deploy")
		if !strings.Contains(s, "> [ ] 1. production-eu") {
			t.Fatalf("screen:\n%s", s)
		}
		p.typeKeys(t, " ", "j", "j", " ", "j")
		p.waitScreen(t, "> Deploy")
		p.typeKeys(t, "\r")
		r := p.mustFinish(t)
		if r.err != nil || r.val != "production-eu,staging" {
			t.Fatalf("got %q, %v", r.val, r.err)
		}
		if s := p.screen(); s != "? Envs: production-eu, staging" {
			t.Fatalf("screen %q", s)
		}
	})
	t.Run("empty finish refused without Validate", func(t *testing.T) {
		p := raw(t, clix.WithLabel("Envs"), prompt.MultiSelect(envs))
		p.typeKeys(t, "\x1b[F", "\r")
		p.waitScreen(t, "Please select at least one option")
		p.typeKeys(t, "2", "\x1b[F", "\r")
		if r := p.mustFinish(t); r.val != "production" {
			t.Fatalf("got %q", r.val)
		}
	})
	t.Run("empty finish allowed by Validate", func(t *testing.T) {
		v, err, _ := runRaw(t, []string{"\x1b[F", "\r"}, clix.WithLabel("Envs"), prompt.MultiSelect(envs), clix.WithValidate(func(string) error { return nil }))
		if err != nil || v != "" {
			t.Fatalf("got %q, %v", v, err)
		}
	})
	t.Run("default preselects", func(t *testing.T) {
		v, _, _ := runRaw(t, []string{"\x1b[F", "\r"}, clix.WithLabel("Envs"), prompt.MultiSelect(envs), clix.WithDefault("staging,production"))
		if v != "production,staging" {
			t.Fatalf("got %q", v)
		}
	})
}

func TestConformanceMultiSelectLine(t *testing.T) {
	ports := []clix.SelectOption{{Label: "443", Value: "443"}, {Label: "2", Value: "2"}, {Label: "1", Value: "1"}}
	cases := []struct {
		name  string
		opts  []clix.PromptOption
		input string
		want  string
		msg   string
	}{
		{"numbers", nil, "1,3\ndone\n", "production-eu,staging", ""},
		{"spaces", nil, "1 3\n\n", "production-eu,staging", ""},
		{"by value", nil, "staging\nfinish\n", "staging", ""},
		{"toggle off counts", nil, "1\n1\ndone\n2\ndone\n", "production", "Please select at least one option"},
		{"3rd is not 3", nil, "3rd\n2\nq\n", "production", "Invalid selection"},
		{"default by value", []clix.PromptOption{clix.WithDefault("production")}, "\n", "production", ""},
		{"empty allowed by Validate", []clix.PromptOption{clix.WithValidate(func(string) error { return nil })}, "done\n", "", ""},
	}
	for _, mode := range []string{"line", "dumb"} {
		for _, c := range cases {
			opts := append([]clix.PromptOption{clix.WithLabel("Envs"), prompt.MultiSelect(envs)}, c.opts...)
			v, err, out := runLine(t, mode, c.input, opts...)
			if err != nil || v != c.want || (c.msg != "" && !strings.Contains(out, c.msg)) {
				t.Errorf("%s %s: got %q, %v, output %q", mode, c.name, v, err, out)
			}
		}
	}
	v, err, _ := runLine(t, "line", "\n", clix.WithLabel("Ports"), prompt.MultiSelect(ports), clix.WithDefault("1"))
	if err != nil || v != "1" {
		t.Errorf("numeric value default: got %q, %v", v, err)
	}
	if _, err, _ := runLine(t, "line", "\n", clix.WithLabel("Envs"), prompt.MultiSelect(envs), clix.WithDefault("dev")); !errors.Is(err, clix.ErrInvalidDefault) {
		t.Errorf("bad default: %v", err)
	}
}

// --- Option order and the struct API ---

func TestConformanceOptionOrder(t *testing.T) {
	for _, mode := range []string{"text", "line"} {
		t.Run(mode, func(t *testing.T) {
			// Later options override earlier ones.
			v, _, out := runLine(t, mode, "\n", clix.WithLabel("First"), clix.WithDefault("a"),
				clix.PromptRequest{Label: "Second"}, clix.WithDefault("b"))
			if v != "b" || !strings.HasPrefix(out, "? Second [b]: ") {
				t.Fatalf("got %q, output %q", v, out)
			}
			// A struct's zero fields keep earlier values.
			v, _, out = runLine(t, mode, "\n", clix.WithLabel("Name"), clix.WithDefault("a"), clix.PromptRequest{Validate: lowercase})
			if v != "a" || !strings.HasPrefix(out, "? Name [a]: ") {
				t.Fatalf("got %q, output %q", v, out)
			}
			// Themes merge in order.
			_, _, out = runLine(t, mode, "x\n", clix.WithLabel("Name"), clix.WithTheme(clix.PromptTheme{Prefix: "> "}), clix.PromptRequest{Theme: clix.PromptTheme{Hint: "h"}})
			if !strings.HasPrefix(out, "> Name h: ") {
				t.Fatalf("output %q", out)
			}
		})
	}
	// Select after MultiSelect is a single select.
	v, err, out := runLine(t, "line", "2\n", clix.WithLabel("Pick"), prompt.MultiSelect(envs), prompt.Select(envs))
	if err != nil || v != "production" || strings.Contains(out, "[ ]") {
		t.Fatalf("got %q, %v, output %q", v, err, out)
	}
}

// --- Every mode: ctx, writes to App.Err only, no goroutine leaks ---

func TestConformanceLineContextCancel(t *testing.T) {
	cases := map[string][]clix.PromptOption{
		"text":         {clix.WithLabel("Name")},
		"secret":       {clix.WithLabel("Token"), clix.WithSecret()},
		"confirm":      {clix.WithLabel("Sure?"), clix.WithConfirm()},
		"select":       {clix.WithLabel("Pick"), prompt.Select(envs)},
		"multi-select": {clix.WithLabel("Pick"), prompt.MultiSelect(envs)},
	}
	for name, opts := range cases {
		for _, kind := range []string{"TextPrompter", "TerminalPrompter"} {
			if kind == "TextPrompter" && (name == "select" || name == "multi-select") {
				continue
			}
			t.Run(kind+"/"+name, func(t *testing.T) {
				checkGoroutines(t)
				pr, pw := io.Pipe()
				var p clix.Prompter = clix.TextPrompter{In: pr, Out: io.Discard}
				if kind == "TerminalPrompter" {
					p = prompt.TerminalPrompter{In: pr, Out: io.Discard}
				}
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				start := time.Now()
				_, err := p.Prompt(ctx, opts...)
				if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 500*time.Millisecond {
					t.Fatalf("err = %v after %v", err, time.Since(start))
				}
				pw.Close() // ends the read still parked for the next prompt
			})
		}
	}
}

func TestConformancePromptsGoToAppErr(t *testing.T) {
	for _, withExt := range []bool{false, true} {
		t.Run(map[bool]string{false: "TextPrompter", true: "TerminalPrompter"}[withExt], func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			errOut, out := &bytes.Buffer{}, &bytes.Buffer{}
			app := clix.NewApp("probe", clix.WithAppIn(strings.NewReader("bob\ny\n2\n")), clix.WithAppErr(errOut), clix.WithAppOut(out))
			app.Interactive = clix.InteractiveAlways
			if withExt {
				app.AddExtension(prompt.Extension{})
			}
			var name string
			cmd := clix.NewCommand("ask", clix.WithCommandRun(func(ctx *clix.Context) error {
				if _, err := ctx.App.Prompter.Prompt(ctx, clix.WithLabel("Sure?"), clix.WithConfirm()); err != nil {
					return err
				}
				if withExt {
					_, err := ctx.App.Prompter.Prompt(ctx, clix.WithLabel("Env"), prompt.Select(envs))
					return err
				}
				return nil
			}))
			cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
			app.Root.AddCommand(cmd)
			if err := app.Run(context.Background(), []string{"ask"}); err != nil {
				t.Fatal(err)
			}
			if out.Len() != 0 || !strings.Contains(errOut.String(), "? Name: ") || !strings.Contains(errOut.String(), "? Sure? (Y/n): ") {
				t.Fatalf("Out %q, Err %q", out.String(), errOut.String())
			}
		})
	}
}

func TestConformanceNoGoroutineLeaks(t *testing.T) {
	checkGoroutines(t)
	for _, mode := range []string{"text", "line"} {
		runLine(t, mode, "x\n", clix.WithLabel("Name"))
		runLine(t, mode, "y\n", clix.WithLabel("Sure?"), clix.WithConfirm())
	}
	runLine(t, "line", "1\n", clix.WithLabel("Pick"), prompt.Select(envs))
	runLine(t, "line", "1\n\n", clix.WithLabel("Pick"), prompt.MultiSelect(envs))
	runRaw(t, []string{"x", "\r"}, clix.WithLabel("Name"))
	runRaw(t, []string{"\r"}, clix.WithLabel("Pick"), prompt.Select(envs))
}
