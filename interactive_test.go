package clix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func newGreetApp(t *testing.T, opts ...AppOption) (app *App, out, errOut *bytes.Buffer, prompted *bool) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}
	app = NewApp("myapp", append([]AppOption{WithAppOut(out), WithAppErr(errOut)}, opts...)...)
	cmd := NewCommand("greet", WithCommandRun(func(ctx *Context) error {
		name, _ := ctx.String("name")
		_, err := fmt.Fprintf(ctx.App.Out, "hello %s\n", name)
		return err
	}))
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "name", Required: true, EnvVars: []string{"GREET_NAME"}}})
	app.Root.AddCommand(cmd)
	prompted = new(bool)
	return app, out, errOut, prompted
}

func TestNonInteractiveReturnsMissingFlagsError(t *testing.T) {
	app, _, _, prompted := newGreetApp(t, WithAppIn(strings.NewReader("bob\n")))
	app.Prompter = prompterFunc(func(context.Context, ...PromptOption) (string, error) {
		*prompted = true
		return "bob", nil
	})

	if app.IsInteractive() {
		t.Fatal("an in-memory reader must not count as a terminal")
	}
	err := app.Run(context.Background(), []string{"greet"})
	if *prompted {
		t.Fatal("prompted although the app is not interactive")
	}
	var missing *MissingFlagsError
	if !errors.As(err, &missing) {
		t.Fatalf("Run error = %v (%T), want *MissingFlagsError", err, err)
	}
	if len(missing.Flags) != 1 || missing.Flags[0] != "name" {
		t.Fatalf("Flags = %v, want [name]", missing.Flags)
	}
	want := "missing required flags: --name (or $GREET_NAME, $MYAPP_NAME)"
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("ExitCode = %d, want 2", code)
	}
}

func TestNonInteractivePipeIsNotATerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	app, _, _, prompted := newGreetApp(t, WithAppIn(r))
	app.Prompter = prompterFunc(func(context.Context, ...PromptOption) (string, error) {
		*prompted = true
		return "", nil
	})
	err = app.Run(context.Background(), []string{"greet"})
	var missing *MissingFlagsError
	if *prompted || !errors.As(err, &missing) {
		t.Fatalf("prompted=%v err=%v; an idle pipe must not be prompted", *prompted, err)
	}
}

func TestDisablePromptsEnvOverridesInteractiveAlways(t *testing.T) {
	for _, v := range []string{"1", "true", "YES"} {
		t.Run(v, func(t *testing.T) {
			app, _, _, prompted := newGreetApp(t, WithAppInteractive(InteractiveAlways))
			t.Setenv("MYAPP_DISABLE_PROMPTS", v)
			if name := app.DisablePromptsEnvVar(); name != "MYAPP_DISABLE_PROMPTS" {
				t.Fatalf("DisablePromptsEnvVar() = %q", name)
			}
			app.Prompter = prompterFunc(func(context.Context, ...PromptOption) (string, error) {
				*prompted = true
				return "bob", nil
			})
			err := app.Run(context.Background(), []string{"greet"})
			var missing *MissingFlagsError
			if *prompted || !errors.As(err, &missing) {
				t.Fatalf("prompted=%v err=%v; MYAPP_DISABLE_PROMPTS=%s must disable prompts", *prompted, err, v)
			}
		})
	}

	app, _, _, _ := newGreetApp(t, WithAppInteractive(InteractiveAlways))
	t.Setenv("MYAPP_DISABLE_PROMPTS", "0")
	if !app.IsInteractive() {
		t.Fatal("MYAPP_DISABLE_PROMPTS=0 should not disable prompts")
	}
}

func TestInteractiveNever(t *testing.T) {
	app, _, _, _ := newGreetApp(t, WithAppInteractive(InteractiveNever))
	if app.IsInteractive() {
		t.Fatal("InteractiveNever reported interactive")
	}
}

func TestMissingFlagsErrorWhenSomeFlagsPassed(t *testing.T) {
	app, _, _, _ := newGreetApp(t, WithAppInteractive(InteractiveAlways))
	app.Flags().BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "quiet"}})
	err := app.Run(context.Background(), []string{"greet", "--quiet"})
	var missing *MissingFlagsError
	if !errors.As(err, &missing) || !strings.Contains(err.Error(), "--name") {
		t.Fatalf("Run error = %v, want *MissingFlagsError for --name", err)
	}
}

func TestPromptsGoToErrNotOut(t *testing.T) {
	app, out, errOut, _ := newGreetApp(t, WithAppIn(strings.NewReader("bob\n")), WithAppInteractive(InteractiveAlways))
	if err := app.Run(context.Background(), []string{"greet"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if out.String() != "hello bob\n" {
		t.Fatalf("stdout = %q, want only the command's output", out.String())
	}
	if !strings.Contains(errOut.String(), "Name") {
		t.Fatalf("prompt not written to stderr: %q", errOut.String())
	}
}

func TestExitCode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	deadline, cancelDeadline := context.WithTimeout(context.Background(), 0)
	defer cancelDeadline()
	<-deadline.Done()

	app, _, _ := newIsolatedApp(t, "myapp")
	app.Root.AddCommand(NewGroup("projects", "Projects", NewCommand("list", WithCommandRun(func(*Context) error { return nil }))))
	unknownFlag := app.Run(context.Background(), []string{"projects", "list", "--bogus"})
	unknownCmd := app.Run(context.Background(), []string{"projects", "lsit"})

	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"cancelled", fmt.Errorf("prompt: %w", ErrCancelled), 130},
		{"context canceled", ctx.Err(), 130},
		{"deadline", deadline.Err(), 1},
		{"missing flags", &MissingFlagsError{Flags: []string{"x"}}, 2},
		{"unknown flag", unknownFlag, 2},
		{"unknown command", unknownCmd, 2},
		{"other", errors.New("boom"), 1},
	} {
		if got := ExitCode(tc.err); got != tc.want {
			t.Errorf("ExitCode(%s: %v) = %d, want %d", tc.name, tc.err, got, tc.want)
		}
	}
	if unknownFlag == nil || unknownFlag.Error() != "unknown flag: --bogus" {
		t.Fatalf("usage error message changed: %v", unknownFlag)
	}
}
