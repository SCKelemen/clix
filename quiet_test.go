package clix_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
)

// failReader fails the test when anything reads from it: under --quiet no
// prompter may read input.
type failReader struct{ t *testing.T }

func (r failReader) Read([]byte) (int, error) {
	r.t.Helper()
	r.t.Error("a prompt read input under --quiet")
	return 0, errors.New("read under --quiet")
}

// failPrompter fails the test when Run prompts.
type failPrompter struct{ t *testing.T }

func (p failPrompter) Prompt(context.Context, ...clix.PromptOption) (string, error) {
	p.t.Helper()
	p.t.Error("prompted under --quiet")
	return "", errors.New("prompted")
}

func newQuietApp(t *testing.T, opts clix.QuietOptions) (*clix.App, *bytes.Buffer) {
	t.Helper()
	app, out := newTestApp(t, "myapp", clix.WithAppInteractive(clix.InteractiveAlways))
	app.EnableQuiet(opts)
	return app, out
}

func TestEnableQuietRegistersFlag(t *testing.T) {
	t.Run("no short name by default", func(t *testing.T) {
		app, _ := newQuietApp(t, clix.QuietOptions{})
		app.EnableQuiet(clix.QuietOptions{Short: "q"}) // a second call does nothing
		f := app.Flags().Lookup("quiet")
		if f == nil {
			t.Fatal("--quiet not registered")
		}
		if f.Short != "" || !f.Ambient || !f.CommandLineOnly {
			t.Fatalf("--quiet: Short %q, Ambient %v, CommandLineOnly %v; want \"\", true, true", f.Short, f.Ambient, f.CommandLineOnly)
		}
		if !strings.Contains(f.Usage, "Disable all interactive prompts") {
			t.Fatalf("usage = %q", f.Usage)
		}
		n := 0
		for _, g := range app.Flags().Flags() {
			if g.Name == "quiet" {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("%d --quiet flags after two calls, want 1", n)
		}
		if err := app.Flags().Err(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("short name on request", func(t *testing.T) {
		app, _ := newQuietApp(t, clix.QuietOptions{Short: "q"})
		var quiet bool
		app.Root.Run = func(ctx *clix.Context) error { quiet = ctx.App.Quiet(); return nil }
		if err := app.Run(bg, []string{"-q"}); err != nil || !quiet {
			t.Fatalf("-q: err %v, Quiet %v", err, quiet)
		}
	})
}

// --quiet comes from the command line only: neither <PREFIX>_QUIET nor a
// "quiet" config key turns it on, and it does not carry over to the next
// Run.
func TestQuietOnlyFromCommandLine(t *testing.T) {
	app, _ := newQuietApp(t, clix.QuietOptions{})
	writeConfigFile(t, app, "quiet: true\n")
	t.Setenv("MYAPP_QUIET", "1")
	var quiet, interactive bool
	app.Root.Run = func(ctx *clix.Context) error {
		quiet, interactive = ctx.App.Quiet(), ctx.App.IsInteractive()
		return nil
	}
	for _, tc := range []struct {
		args               []string
		quiet, interactive bool
	}{
		{[]string{}, false, true},
		{[]string{"--quiet"}, true, false},
		{[]string{"--quiet=false"}, false, true},
		{[]string{}, false, true}, // not carried over
	} {
		if err := app.Run(bg, tc.args); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if quiet != tc.quiet || interactive != tc.interactive {
			t.Fatalf("%v: Quiet %v, IsInteractive %v; want %v, %v", tc.args, quiet, interactive, tc.quiet, tc.interactive)
		}
	}
	if app.Quiet() {
		t.Fatal("Quiet() true after a Run without --quiet")
	}
}

func TestQuietWithoutEnableQuiet(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	if app.Quiet() {
		t.Fatal("Quiet() without EnableQuiet")
	}
	app.Root.Run = noop
	err := app.Run(bg, []string{"--quiet"})
	if err == nil || !strings.Contains(err.Error(), "--quiet") {
		t.Fatalf("--quiet without EnableQuiet: err = %v, want an unknown flag error", err)
	}
	var nilApp *clix.App
	if nilApp.Quiet() {
		t.Fatal("nil App is quiet")
	}
}

// Under --quiet a missing required flag is an error, even with
// InteractiveAlways and although --quiet is Ambient.
func TestQuietRequiredFlagsError(t *testing.T) {
	app, _ := newQuietApp(t, clix.QuietOptions{})
	app.Prompter = failPrompter{t}
	var name string
	create := clix.NewCommand("create", clix.WithCommandRun(noop))
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
	app.Root.AddCommand(create)

	err := app.Run(bg, []string{"--quiet", "create"})
	var missing *clix.MissingFlagsError
	if !errors.As(err, &missing) || !reflect.DeepEqual(missing.Flags, []string{"name"}) {
		t.Fatalf("err = %v, want a *MissingFlagsError for --name", err)
	}
	if code := clix.ExitCode(err); code != 2 {
		t.Fatalf("exit code %d, want 2", code)
	}

	// Without --quiet the same command line prompts.
	prompted := false
	app.Prompter = promptFunc(func(context.Context, ...clix.PromptOption) (string, error) {
		prompted = true
		return "w1", nil
	})
	if err := app.Run(bg, []string{"create"}); err != nil || !prompted || name != "w1" {
		t.Fatalf("without --quiet: err %v, prompted %v, name %q", err, prompted, name)
	}
}

type promptFunc func(context.Context, ...clix.PromptOption) (string, error)

func (f promptFunc) Prompt(ctx context.Context, opts ...clix.PromptOption) (string, error) {
	return f(ctx, opts...)
}

// An app flag named --quiet is resolved like any name conflict: it wins
// over EnableQuiet called from an extension, and between the app's own
// definitions the first one registered is kept. When the app's flag is in
// effect, --quiet is an ordinary flag and Quiet reports false.
func TestAppQuietFlagConflicts(t *testing.T) {
	for _, tc := range []struct {
		name      string
		setup     func(app *clix.App, register func())
		ownWins   bool
		validates []string
	}{
		{"app flag first", func(app *clix.App, register func()) {
			register()
			app.EnableQuiet(clix.QuietOptions{})
		}, true, []string{"flag --quiet redefined"}},
		{"EnableQuiet first", func(app *clix.App, register func()) {
			app.EnableQuiet(clix.QuietOptions{})
			register()
		}, false, []string{"flag --quiet redefined"}},
		{"EnableQuiet in an extension", func(app *clix.App, register func()) {
			app.AddExtension(extensionFunc(func(app *clix.App) error {
				app.EnableQuiet(clix.QuietOptions{})
				return nil
			}))
			register()
		}, true, []string{"the app's --quiet is used"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _ := newTestApp(t, "myapp", clix.WithAppInteractive(clix.InteractiveAlways))
			var own bool
			tc.setup(app, func() {
				app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "quiet"}, Value: &own})
			})
			var quiet, interactive bool
			app.Root.Run = func(ctx *clix.Context) error {
				quiet, interactive = ctx.App.Quiet(), ctx.App.IsInteractive()
				return nil
			}
			if err := app.Run(bg, []string{"--quiet"}); err != nil {
				t.Fatal(err)
			}
			if tc.ownWins && (!own || quiet || !interactive) {
				t.Fatalf("app's --quiet %v, Quiet %v, IsInteractive %v; want true, false, true", own, quiet, interactive)
			}
			if !tc.ownWins && (own || !quiet || interactive) {
				t.Fatalf("app's --quiet %v, Quiet %v, IsInteractive %v; want false, true, false", own, quiet, interactive)
			}
			assertValidate(t, app, tc.validates...)
		})
	}
}

// runQuietPrompt runs a handler that prompts with req under --quiet and
// returns the answer, what the prompter wrote and the prompt's error.
func runQuietPrompt(t *testing.T, prompter func(in *failReader, out *bytes.Buffer) clix.Prompter, req clix.PromptRequest) (got, drawn string, err error) {
	t.Helper()
	app, _ := newQuietApp(t, clix.QuietOptions{})
	in := &failReader{t}
	var out bytes.Buffer
	app.Prompter = prompter(in, &out)
	app.Root.Run = func(ctx *clix.Context) error {
		got, err = ctx.App.Prompter.Prompt(ctx, req)
		return nil
	}
	if runErr := app.Run(bg, []string{"--quiet"}); runErr != nil {
		t.Fatal(runErr)
	}
	return got, out.String(), err
}

var quietOptions = []clix.SelectOption{
	{Label: "Small", Value: "s"},
	{Label: "Medium", Value: "m"},
	{Label: "Large"}, // no Value
}

var quietAnswerCases = []struct {
	name    string
	req     clix.PromptRequest
	want    string
	wantErr error // when not nil, the error must wrap it
}{
	{"confirm without default", clix.PromptRequest{Label: "Delete?", Confirm: true}, "y", nil},
	{"confirm default yes", clix.PromptRequest{Label: "Delete?", Confirm: true, Default: "yes"}, "y", nil},
	{"confirm default no", clix.PromptRequest{Label: "Delete?", Confirm: true, Default: "n"}, "n", nil},
	{"confirm default false", clix.PromptRequest{Label: "Delete?", Confirm: true, Default: " FALSE "}, "n", nil},
	{"text default", clix.PromptRequest{Label: "Name", Default: "w1"}, "w1", nil},
	{"text default kept as is", clix.PromptRequest{Label: "Name", Default: " w1 "}, " w1 ", nil},
	{"text without default", clix.PromptRequest{Label: "Name"}, "", clix.ErrPromptsDisabled},
	{"secret default", clix.PromptRequest{Label: "Token", Secret: true, Default: "s3cret"}, "s3cret", nil},
	{"secret without default", clix.PromptRequest{Label: "Token", Secret: true}, "", clix.ErrPromptsDisabled},
	{"select by value", clix.PromptRequest{Label: "Size", Options: quietOptions, Default: "m"}, "m", nil},
	{"select by label", clix.PromptRequest{Label: "Size", Options: quietOptions, Default: "Medium"}, "m", nil},
	{"select option without value", clix.PromptRequest{Label: "Size", Options: quietOptions, Default: "Large"}, "", nil},
	{"select without default", clix.PromptRequest{Label: "Size", Options: quietOptions}, "", clix.ErrPromptsDisabled},
	{"select unknown default", clix.PromptRequest{Label: "Size", Options: quietOptions, Default: "huge"}, "", clix.ErrInvalidDefault},
	{"select no options", clix.PromptRequest{Label: "Size", Options: []clix.SelectOption{}}, "", clix.ErrNoOptions},
	{"multi-select in option order", clix.PromptRequest{Label: "Sizes", Options: quietOptions, MultiSelect: true, Default: "Large,s"}, "s,Large", nil},
	{"multi-select by number", clix.PromptRequest{Label: "Sizes", Options: quietOptions, MultiSelect: true, Default: "2,1"}, "s,m", nil},
	{"multi-select without default", clix.PromptRequest{Label: "Sizes", Options: quietOptions, MultiSelect: true}, "", clix.ErrPromptsDisabled},
	{"multi-select empty default", clix.PromptRequest{Label: "Sizes", Options: quietOptions, MultiSelect: true, Default: " , "}, "", clix.ErrPromptsDisabled},
	{"default fails validation", clix.PromptRequest{Label: "Name", Default: "bad", Validate: func(v string) error {
		if v == "bad" {
			return errors.New("not allowed: bad")
		}
		return nil
	}}, "", nil},
	{"confirm answer validated", clix.PromptRequest{Label: "Delete?", Confirm: true, Validate: func(v string) error {
		if v == "y" {
			return errors.New("must decline")
		}
		return nil
	}}, "", nil},
}

func TestQuietAnswer(t *testing.T) {
	for _, tc := range quietAnswerCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &clix.PromptConfig{}
			tc.req.Apply(cfg)
			got, err := clix.QuietAnswer(cfg)
			validated := tc.req.Validate != nil
			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want one wrapping %v", err, tc.wantErr)
				}
				if errors.Is(tc.wantErr, clix.ErrPromptsDisabled) && (!errors.Is(err, clix.ErrQuiet) || !strings.Contains(err.Error(), "no default")) {
					t.Fatalf("err = %v, want it to wrap ErrQuiet and say there is no default", err)
				}
			case validated:
				if err == nil || !strings.Contains(err.Error(), "under --quiet") {
					t.Fatalf("err = %v, want a validation error", err)
				}
			default:
				if err != nil || got != tc.want {
					t.Fatalf("QuietAnswer = %q, %v; want %q", got, err, tc.want)
				}
			}
		})
	}
	if _, err := clix.QuietAnswer(nil); err == nil {
		t.Fatal("QuietAnswer(nil) did not fail")
	}
}

// A secret default that fails validation is not shown in the error.
func TestQuietAnswerRedactsSecret(t *testing.T) {
	cfg := &clix.PromptConfig{}
	clix.PromptRequest{Label: "Token", Secret: true, Default: "hunter2", Validate: func(v string) error {
		return errors.New("token " + v + " is too short")
	}}.Apply(cfg)
	_, err := clix.QuietAnswer(cfg)
	if err == nil || strings.Contains(err.Error(), "hunter2") || !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("err = %v, want the secret redacted", err)
	}
}

// The built-in prompters answer with QuietAnswer: they read nothing and
// draw nothing.
func TestBuiltinPromptersUnderQuiet(t *testing.T) {
	prompters := map[string]func(in *failReader, out *bytes.Buffer) clix.Prompter{
		"TextPrompter": func(in *failReader, out *bytes.Buffer) clix.Prompter { return clix.TextPrompter{In: in, Out: out} },
		"TerminalPrompter": func(in *failReader, out *bytes.Buffer) clix.Prompter {
			return prompt.TerminalPrompter{In: in, Out: out}
		},
	}
	for name, newPrompter := range prompters {
		for _, tc := range quietAnswerCases {
			if name == "TextPrompter" && len(tc.req.Options) > 0 {
				continue // select prompts need the prompt extension
			}
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				cfg := &clix.PromptConfig{}
				tc.req.Apply(cfg)
				want, wantErr := clix.QuietAnswer(cfg)
				got, drawn, err := runQuietPrompt(t, newPrompter, tc.req)
				if got != want || (err == nil) != (wantErr == nil) {
					t.Fatalf("Prompt = %q, %v; want %q, %v", got, err, want, wantErr)
				}
				if wantErr != nil && err.Error() != wantErr.Error() {
					t.Fatalf("err = %v, want %v", err, wantErr)
				}
				if drawn != "" {
					t.Fatalf("drew %q under --quiet", drawn)
				}
			})
		}
	}
}

// NewPromptConfig reports --quiet with an error that wraps ErrQuiet and
// ErrPromptsDisabled, after checking the options, and --quiet wins over
// <PREFIX>_DISABLE_PROMPTS and InteractiveNever.
func TestNewPromptConfigUnderQuiet(t *testing.T) {
	for _, disabled := range []string{"", "env", "never"} {
		t.Run("disabled="+disabled, func(t *testing.T) {
			app, _ := newQuietApp(t, clix.QuietOptions{})
			t.Setenv("MYAPP_DISABLE_PROMPTS", "")
			switch disabled {
			case "env":
				t.Setenv("MYAPP_DISABLE_PROMPTS", "1")
			case "never":
				app.Interactive = clix.InteractiveNever
			}
			var errs []error
			app.Root.Run = func(ctx *clix.Context) error {
				for _, req := range []clix.PromptRequest{
					{Label: "Name", Default: "x"},
					{Label: "Size", Options: []clix.SelectOption{}},
					{Label: "Size", Options: quietOptions, Default: "huge"},
				} {
					cfg, err := clix.NewPromptConfig(ctx, req)
					if cfg == nil || cfg.Label != req.Label {
						t.Errorf("NewPromptConfig returned config %+v", cfg)
					}
					errs = append(errs, err)
				}
				return nil
			}
			if err := app.Run(bg, []string{"--quiet"}); err != nil {
				t.Fatal(err)
			}
			if !errors.Is(errs[0], clix.ErrQuiet) || !errors.Is(errs[0], clix.ErrPromptsDisabled) {
				t.Fatalf("err = %v, want ErrQuiet and ErrPromptsDisabled", errs[0])
			}
			if want := `cannot prompt for "Name": prompts are disabled (--quiet is set)`; errs[0].Error() != want {
				t.Fatalf("err = %q, want %q", errs[0], want)
			}
			if !errors.Is(errs[1], clix.ErrNoOptions) || errors.Is(errs[1], clix.ErrQuiet) {
				t.Fatalf("empty options: err = %v, want ErrNoOptions", errs[1])
			}
			if !errors.Is(errs[2], clix.ErrInvalidDefault) || errors.Is(errs[2], clix.ErrQuiet) {
				t.Fatalf("bad default: err = %v, want ErrInvalidDefault", errs[2])
			}
		})
	}

	// Without --quiet, DISABLE_PROMPTS still fails closed.
	app, _ := newQuietApp(t, clix.QuietOptions{})
	t.Setenv("MYAPP_DISABLE_PROMPTS", "1")
	var err error
	app.Root.Run = func(ctx *clix.Context) error {
		_, err = ctx.App.Prompter.Prompt(ctx, clix.PromptRequest{Label: "Delete?", Confirm: true})
		return nil
	}
	if runErr := app.Run(bg, []string{}); runErr != nil {
		t.Fatal(runErr)
	}
	if !errors.Is(err, clix.ErrPromptsDisabled) || errors.Is(err, clix.ErrQuiet) {
		t.Fatalf("DISABLE_PROMPTS without --quiet: err = %v, want ErrPromptsDisabled only", err)
	}
}

// Under --quiet Context.Progress prints nothing, whatever ProgressMode is.
func TestQuietProgress(t *testing.T) {
	for _, mode := range []clix.ProgressMode{clix.ProgressAuto, clix.ProgressPlain} {
		app, out := newQuietApp(t, clix.QuietOptions{})
		app.ProgressMode = mode
		app.Root.Run = func(ctx *clix.Context) error {
			p := ctx.Progress("Deploying")
			p.Update(0.5, "uploading")
			p.Done("deployed")
			ctx.Progress("Left running") // Run ends it
			return nil
		}
		if err := app.Run(bg, []string{"--quiet"}); err != nil {
			t.Fatal(err)
		}
		if out.Len() != 0 {
			t.Fatalf("%v: progress printed %q under --quiet", mode, out.String())
		}
		if err := app.Run(bg, []string{}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Deploying...") {
			t.Fatalf("%v: without --quiet: %q", mode, out.String())
		}
	}
}

// --quiet shows in help, with -q when the app asks for it.
func TestQuietHelp(t *testing.T) {
	for short, want := range map[string]string{"": "  --quiet", "q": "-q, --quiet"} {
		app, out := newQuietApp(t, clix.QuietOptions{Short: short})
		app.Root.AddCommand(clix.NewCommand("create", clix.WithCommandRun(noop)))
		if err := app.Run(bg, []string{"--help"}); err != nil {
			t.Fatal(err)
		}
		help := out.String()
		if !strings.Contains(help, want) || !strings.Contains(help, "Disable all interactive prompts") {
			t.Fatalf("short %q: help lacks %q:\n%s", short, want, help)
		}
		if short == "" && strings.Contains(help, "-q,") {
			t.Fatalf("help shows -q without opting in:\n%s", help)
		}
	}
}
