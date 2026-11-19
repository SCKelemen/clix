package clix_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
)

// Confirm returns "y" or "n" and nothing else; "back" is not an answer.
func TestTextPrompterConfirmReturnsOnlyYesOrNo(t *testing.T) {
	out := &bytes.Buffer{}
	tp := clix.TextPrompter{In: strings.NewReader("back\nn\n"), Out: out}
	got, err := tp.Prompt(context.Background(), clix.WithLabel("Delete everything?"), clix.WithConfirm())
	if err != nil || got != "n" {
		t.Fatalf("got %q, %v", got, err)
	}
	if !strings.Contains(out.String(), "please enter 'y' or 'n'") {
		t.Fatalf("output %q", out.String())
	}
}

func TestTextPrompterConfirmAnswers(t *testing.T) {
	for in, want := range map[string]string{"y\n": "y", "YES\n": "y", " Yes \n": "y", "n\n": "n", "No\n": "n", "NO\n": "n"} {
		tp := clix.TextPrompter{In: strings.NewReader(in), Out: io.Discard}
		if got, err := tp.Prompt(context.Background(), clix.WithLabel("Sure?"), clix.WithConfirm()); err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
}

// A Default of "false", "0" or "off" (what a false bool flag prints) means No.
func TestTextPrompterConfirmDefaultNo(t *testing.T) {
	for _, def := range []string{"n", "no", "false", "FALSE", "0", "off", "No ", "f"} {
		out := &bytes.Buffer{}
		tp := clix.TextPrompter{In: strings.NewReader("\n"), Out: out}
		got, _ := tp.Prompt(context.Background(), clix.WithLabel("Delete?"), clix.WithConfirm(), clix.WithDefault(def))
		if got != "n" || !strings.Contains(out.String(), "(y/N)") {
			t.Errorf("Default %q: Enter answered %q, prompt %q", def, got, out.String())
		}
	}
	for _, def := range []string{"", "y", "yes", "true", "1", "on"} {
		tp := clix.TextPrompter{In: strings.NewReader("\n"), Out: io.Discard}
		if got, _ := tp.Prompt(context.Background(), clix.WithLabel("Delete?"), clix.WithConfirm(), clix.WithDefault(def)); got != "y" {
			t.Errorf("Default %q: Enter answered %q", def, got)
		}
	}
}

func TestTextPrompterConfirmRunsValidate(t *testing.T) {
	var seen []string
	out := &bytes.Buffer{}
	tp := clix.TextPrompter{In: strings.NewReader("n\ny\n"), Out: out}
	got, err := tp.Prompt(context.Background(), clix.WithLabel("Accept the terms?"), clix.WithConfirm(),
		clix.WithValidate(func(s string) error {
			seen = append(seen, s)
			if s != "y" {
				return errors.New("you must accept")
			}
			return nil
		}))
	if err != nil || got != "y" || strings.Join(seen, ",") != "n,y" || !strings.Contains(out.String(), "you must accept") {
		t.Fatalf("got %q, %v, validated %q, output %q", got, err, seen, out.String())
	}
}

// When input ends right after an invalid answer, the prompt reports why the
// answer was rejected (and still matches io.EOF).
func TestTextPrompterInvalidThenEOF(t *testing.T) {
	bad := errors.New("must be lowercase")
	tp := clix.TextPrompter{In: strings.NewReader("ABC\n"), Out: io.Discard}
	_, err := tp.Prompt(context.Background(), clix.WithLabel("Name"), clix.WithValidate(func(s string) error {
		if strings.ToLower(s) != s {
			return bad
		}
		return nil
	}))
	if !errors.Is(err, bad) || !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want the validation error (and io.EOF)", err)
	}
	tp = clix.TextPrompter{In: strings.NewReader("maybe\n"), Out: io.Discard}
	if _, err := tp.Prompt(context.Background(), clix.WithLabel("Sure?"), clix.WithConfirm()); err == nil || !strings.Contains(err.Error(), "please enter 'y' or 'n'") {
		t.Fatalf("confirm err = %v", err)
	}
}

// Setting one theme field keeps the defaults for the rest.
func TestPartialThemeKeepsDefaults(t *testing.T) {
	bold := clix.StyleFunc(func(s ...string) string { return "*" + strings.Join(s, "") + "*" })
	out := &bytes.Buffer{}
	tp := clix.TextPrompter{In: strings.NewReader("x\n"), Out: out}
	_, _ = tp.Prompt(context.Background(), clix.PromptRequest{Label: "Name", Theme: clix.PromptTheme{LabelStyle: bold}})
	if out.String() != "? *Name*: " {
		t.Fatalf("output %q", out.String())
	}

	out.Reset()
	tp = clix.TextPrompter{In: strings.NewReader("x\n"), Out: out}
	_, _ = tp.Prompt(context.Background(), clix.WithLabel("Name"), clix.WithTheme(clix.PromptTheme{}))
	if out.String() != "? Name: " {
		t.Fatalf("WithTheme(PromptTheme{}) wiped the default theme: %q", out.String())
	}

	// Later options override earlier ones field by field.
	out.Reset()
	tp = clix.TextPrompter{In: strings.NewReader("x\n"), Out: out}
	_, _ = tp.Prompt(context.Background(), clix.WithLabel("Name"),
		clix.WithTheme(clix.PromptTheme{Prefix: "> ", Hint: "(lowercase)"}),
		clix.PromptRequest{Theme: clix.PromptTheme{Prefix: ">> "}})
	if out.String() != ">> Name (lowercase): " {
		t.Fatalf("output %q", out.String())
	}
}

func TestPromptThemeMerge(t *testing.T) {
	s := clix.StyleFunc(func(v ...string) string { return v[0] })
	base := clix.PromptTheme{Prefix: "? ", Error: "! ", LabelStyle: s}
	got := base.Merge(clix.PromptTheme{Hint: "h", ErrorStyle: s})
	if got.Prefix != "? " || got.Error != "! " || got.Hint != "h" || got.LabelStyle == nil || got.ErrorStyle == nil {
		t.Fatalf("Merge = %+v", got)
	}
}

// App.DefaultTheme reaches prompts made from command code, also through a
// context derived from the command's.
func TestCommandPromptsUseAppDefaultTheme(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	errOut := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppIn(strings.NewReader("x\ny\n")), clix.WithAppErr(errOut), clix.WithAppOut(&bytes.Buffer{}))
	app.DefaultTheme = clix.PromptTheme{Prefix: ">> "}
	app.Root.AddCommand(clix.NewCommand("ask", clix.WithCommandRun(func(ctx *clix.Context) error {
		if _, err := ctx.App.Prompter.Prompt(ctx, clix.WithLabel("Name")); err != nil {
			return err
		}
		derived, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		_, err := ctx.App.Prompter.Prompt(derived, clix.WithLabel("Again"))
		return err
	})))
	if err := app.Run(context.Background(), []string{"ask"}); err != nil {
		t.Fatal(err)
	}
	if errOut.String() != ">> Name: >> Again: " {
		t.Fatalf("prompt output %q ignores App.DefaultTheme", errOut.String())
	}
}

// With InteractiveNever (or <PREFIX>_DISABLE_PROMPTS) a command's own
// prompt fails clearly instead of reading stdin.
func TestCommandPromptsRespectInteractive(t *testing.T) {
	for _, mode := range []string{"never", "env"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			in := strings.NewReader("yes\n")
			app := clix.NewApp("probe", clix.WithAppIn(in), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppOut(&bytes.Buffer{}))
			if mode == "never" {
				app.Interactive = clix.InteractiveNever
			} else {
				app.Interactive = clix.InteractiveAlways
				t.Setenv(app.DisablePromptsEnvVar(), "1")
			}
			app.Root.AddCommand(clix.NewCommand("rm", clix.WithCommandRun(func(ctx *clix.Context) error {
				_, err := ctx.App.Prompter.Prompt(ctx, clix.WithLabel("Really?"), clix.WithConfirm())
				return err
			})))
			err := app.Run(context.Background(), []string{"rm"})
			if !errors.Is(err, clix.ErrPromptsDisabled) || !strings.Contains(err.Error(), "Really?") {
				t.Fatalf("err = %v, want ErrPromptsDisabled naming the prompt", err)
			}
			if in.Len() != len("yes\n") {
				t.Fatal("the prompt read stdin")
			}
		})
	}
}

// InteractiveAuto keeps answering prompts from a pipe, as scripts do.
func TestCommandPromptsFromPipeUnderAuto(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	app := clix.NewApp("probe", clix.WithAppIn(strings.NewReader("y\n")), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppOut(&bytes.Buffer{}))
	var got string
	app.Root.AddCommand(clix.NewCommand("rm", clix.WithCommandRun(func(ctx *clix.Context) error {
		var err error
		got, err = ctx.App.Prompter.Prompt(ctx, clix.WithLabel("Really?"), clix.WithConfirm())
		return err
	})))
	if err := app.Run(context.Background(), []string{"rm"}); err != nil || got != "y" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestEmptyOptionsAndBadDefaults(t *testing.T) {
	opts := []clix.SelectOption{{Label: "A", Value: "a"}}
	cases := map[string][]clix.PromptOption{
		"empty options":     {clix.PromptRequest{Label: "Pick", Options: []clix.SelectOption{}}},
		"multi, no options": {clix.PromptRequest{Label: "Pick", MultiSelect: true}},
	}
	for name, o := range cases {
		if _, err := clix.NewPromptConfig(context.Background(), o...); !errors.Is(err, clix.ErrNoOptions) {
			t.Errorf("%s: err = %v, want ErrNoOptions", name, err)
		}
	}
	if _, err := clix.NewPromptConfig(context.Background(), clix.PromptRequest{Label: "Pick", Options: opts, Default: "dev"}); !errors.Is(err, clix.ErrInvalidDefault) {
		t.Errorf("err = %v, want ErrInvalidDefault", err)
	}
	if _, err := clix.NewPromptConfig(context.Background(), clix.PromptRequest{Label: "Pick", Options: opts, Default: "A"}); err != nil {
		t.Errorf("a label default: %v", err)
	}
	if idx, ok := clix.ResolveMultiSelectDefault([]clix.SelectOption{{Value: "443"}, {Value: "2"}, {Value: "1"}}, "1"); !ok || len(idx) != 1 || idx[0] != 2 {
		t.Errorf("numeric value default resolved to %v, %v", idx, ok)
	}
}

func TestReadLine(t *testing.T) {
	r := strings.NewReader("one\r\ntwo\nthree")
	for _, want := range []string{"one", "two", "three"} {
		if got, err := clix.ReadLine(context.Background(), r); err != nil || got != want {
			t.Fatalf("got %q, %v; want %q", got, err, want)
		}
	}
	if _, err := clix.ReadLine(context.Background(), r); !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want EOF", err)
	}
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := clix.ReadLine(ctx, pr); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
}

// Theme styles are written without ANSI escapes to a non-terminal or with
// NO_COLOR; plain-text styling survives.
func TestTextPrompterStripsANSIWithoutColor(t *testing.T) {
	t.Setenv("CLICOLOR_FORCE", "")
	red := clix.StyleFunc(func(s ...string) string { return "\x1b[31m<" + strings.Join(s, "") + ">\x1b[0m" })
	out := &bytes.Buffer{}
	tp := clix.TextPrompter{In: strings.NewReader("x\n"), Out: out}
	_, _ = tp.Prompt(context.Background(), clix.WithLabel("Name"), clix.WithTheme(clix.PromptTheme{LabelStyle: red}))
	if out.String() != "? <Name>: " {
		t.Fatalf("output %q", out.String())
	}
	t.Setenv("CLICOLOR_FORCE", "1")
	out.Reset()
	tp = clix.TextPrompter{In: strings.NewReader("x\n"), Out: out}
	_, _ = tp.Prompt(context.Background(), clix.WithLabel("Name"), clix.WithTheme(clix.PromptTheme{LabelStyle: red}))
	if !strings.Contains(out.String(), "\x1b[31m") {
		t.Fatalf("CLICOLOR_FORCE=1 output %q", out.String())
	}
}

func TestTextPrompterShowsPlaceholderOnlyWithoutDefault(t *testing.T) {
	out := &bytes.Buffer{}
	tp := clix.TextPrompter{In: strings.NewReader("\n"), Out: out}
	got, err := tp.Prompt(context.Background(), clix.WithLabel("Name"), clix.WithNoDefaultPlaceholder("optional"))
	if err != nil || got != "" || out.String() != "? Name [optional]: " {
		t.Fatalf("got %q, %v, output %q", got, err, out.String())
	}
	out.Reset()
	tp = clix.TextPrompter{In: strings.NewReader("\n"), Out: out}
	got, _ = tp.Prompt(context.Background(), clix.WithLabel("Name"), clix.WithNoDefaultPlaceholder("optional"), clix.WithDefault("bob"))
	if got != "bob" || out.String() != "? Name [bob]: " {
		t.Fatalf("got %q, output %q", got, out.String())
	}
}
