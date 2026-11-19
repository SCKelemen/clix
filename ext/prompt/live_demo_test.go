package prompt_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
	"github.com/SCKelemen/clix/v2/ext/survey"
)

// The demo app is what the live end-to-end tests drive in a real
// terminal: a clix app with the prompt extension whose commands ask text,
// select, multi-select, confirm and secret prompts and a survey, and write
// the answers to a results file. Its environment:
//
//	CLIX_DEMO_OUT       results file (JSON, demoResult)
//	CLIX_DEMO_PROGRESS  a line is appended here before each prompt starts
//
// The test binary runs as the demo app when CLIX_LIVE_HELPER=demo (see
// live_main_test.go); its arguments are the app's.

// demoResult is what one run of the demo app answered.
type demoResult struct {
	Command string       `json:"command"`
	Term    string       `json:"term"`
	NoColor string       `json:"noColor"`
	Answers []demoAnswer `json:"answers"`
	Error   string       `json:"error,omitempty"`
	Exit    int          `json:"exit"`
}

type demoAnswer struct {
	Label     string `json:"label"`
	Value     string `json:"value"`
	Error     string `json:"error,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

// demoOptions are the choices of the demo's select prompts.
var demoOptions = []clix.SelectOption{
	{Label: "production-eu", Value: "production-eu"},
	{Label: "production", Value: "production"},
	{Label: "staging", Value: "staging"},
	{Label: "development", Value: "development"},
}

// demoLongOptions is a list taller than a small pane, for the resize test.
func demoLongOptions() []clix.SelectOption {
	opts := make([]clix.SelectOption, 40)
	for i := range opts {
		opts[i] = clix.SelectOption{Label: fmt.Sprintf("item %02d", i+1), Value: fmt.Sprintf("v%02d", i+1)}
	}
	return opts
}

// progressPrompter records each prompt in the progress file just before
// asking it, and each answer in the result, so the driver knows which
// prompt is on screen.
type progressPrompter struct {
	inner  clix.Prompter
	path   string
	result *demoResult
}

func (p progressPrompter) Prompt(ctx context.Context, opts ...clix.PromptOption) (string, error) {
	var cfg clix.PromptConfig
	for _, o := range opts {
		o.Apply(&cfg)
	}
	if p.path != "" {
		if f, err := os.OpenFile(p.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600); err == nil {
			_, _ = fmt.Fprintln(f, cfg.Label)
			_ = f.Close()
		}
	}
	v, err := p.inner.Prompt(ctx, opts...)
	a := demoAnswer{Label: cfg.Label, Value: v}
	if err != nil {
		a.Error = err.Error()
		a.Cancelled = errors.Is(err, clix.ErrCancelled)
	}
	p.result.Answers = append(p.result.Answers, a)
	return v, err
}

// red is a theme style the NO_COLOR test looks for.
var red = clix.StyleFunc(func(s ...string) string { return "\x1b[31m" + strings.Join(s, "") + "\x1b[0m" })

// newDemoApp builds the demo app. Every command asks through a
// progressPrompter that records into result.
func newDemoApp(result *demoResult) *clix.App {
	app := clix.NewApp("demo")
	app.AddExtension(prompt.Extension{})
	cmd := func(name string, run func(ctx *clix.Context, p progressPrompter) error) {
		app.Root.AddCommand(clix.NewCommand(name, clix.WithCommandRun(func(ctx *clix.Context) error {
			result.Command = name
			return run(ctx, progressPrompter{inner: ctx.App.Prompter, path: os.Getenv("CLIX_DEMO_PROGRESS"), result: result})
		})))
	}
	cmd("text", func(ctx *clix.Context, p progressPrompter) error {
		_, err := p.Prompt(ctx, clix.WithLabel("Name"))
		return err
	})
	cmd("select", func(ctx *clix.Context, p progressPrompter) error {
		_, err := p.Prompt(ctx, clix.WithLabel("Env"), prompt.Select(demoOptions))
		return err
	})
	cmd("long-select", func(ctx *clix.Context, p progressPrompter) error {
		_, err := p.Prompt(ctx, clix.WithLabel("Item"), prompt.Select(demoLongOptions()))
		return err
	})
	cmd("multiselect", func(ctx *clix.Context, p progressPrompter) error {
		_, err := p.Prompt(ctx, clix.WithLabel("Envs"), prompt.MultiSelect(demoOptions))
		return err
	})
	cmd("confirm", func(ctx *clix.Context, p progressPrompter) error {
		_, err := p.Prompt(ctx, clix.WithLabel("Proceed?"), clix.WithConfirm())
		return err
	})
	cmd("secret", func(ctx *clix.Context, p progressPrompter) error {
		_, err := p.Prompt(ctx, clix.WithLabel("Token"), clix.WithSecret())
		return err
	})
	cmd("color", func(ctx *clix.Context, p progressPrompter) error {
		_, err := p.Prompt(ctx, clix.WithLabel("Name"), clix.WithTheme(clix.PromptTheme{LabelStyle: red}))
		return err
	})
	// cancel asks a select (cancelled with Esc), then a text prompt
	// (cancelled with Ctrl-C), and fails with the last error.
	cmd("cancel", func(ctx *clix.Context, p progressPrompter) error {
		_, err1 := p.Prompt(ctx, clix.WithLabel("Env"), prompt.Select(demoOptions))
		_, err2 := p.Prompt(ctx, clix.WithLabel("Name"))
		if err2 == nil {
			return err1
		}
		return err2
	})
	cmd("survey", func(ctx *clix.Context, p progressPrompter) error {
		// Each answer's handler asks the next question.
		s := survey.New(ctx, p)
		s.Ask(clix.PromptRequest{Label: "Project"}, func(_ string, s *survey.Survey) {
			s.Ask(clix.PromptRequest{Label: "Region", Options: demoOptions}, func(_ string, s *survey.Survey) {
				s.Ask(clix.PromptRequest{Label: "Deploy now?", Confirm: true}, nil)
			})
		})
		return s.Run()
	})
	return app
}

// runDemoHelper runs the demo app with the test binary's arguments and
// writes the results file.
func runDemoHelper() int {
	result := &demoResult{Term: os.Getenv("TERM"), NoColor: os.Getenv("NO_COLOR")}
	err := newDemoApp(result).Run(context.Background(), os.Args[1:])
	result.Exit = clix.ExitCode(err)
	if err != nil {
		result.Error = err.Error()
	}
	if path := os.Getenv("CLIX_DEMO_OUT"); path != "" {
		b, _ := json.MarshalIndent(result, "", "  ")
		_ = os.WriteFile(path, b, 0o600)
	}
	return result.Exit
}
