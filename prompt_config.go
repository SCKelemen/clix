package clix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SCKelemen/clix/v2/internal/answer"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

// ErrPromptsDisabled is returned by the built-in prompters when a command
// prompts although its App does not allow prompting: App.Interactive is
// InteractiveNever, or the <EnvPrefix>_DISABLE_PROMPTS environment variable
// is set to a true value. Test for it with errors.Is.
var ErrPromptsDisabled = errors.New("prompts are disabled")

// ErrNoOptions is returned when a select or multi-select prompt is given
// an empty option list (for example, one built from a query that found
// nothing). Test for it with errors.Is.
var ErrNoOptions = errors.New("prompt has no options to choose from")

// ErrInvalidDefault is returned when a select or multi-select prompt's
// Default names no option. Test for it with errors.Is.
var ErrInvalidDefault = errors.New("prompt default is not one of the options")

// appContextKey is the context key under which a *Context exposes its App.
type appContextKey struct{}

// Value implements context.Context. It returns the running App for the key
// prompters use to find it, so a prompt made from command code, or with a
// context derived from the command's (for example with a timeout), picks
// up App.DefaultTheme and App.Interactive. Other keys are looked up in the
// embedded context.
func (c *Context) Value(key any) any {
	if _, ok := key.(appContextKey); ok && c.App != nil {
		return c.App
	}
	if c.Context == nil {
		return nil
	}
	return c.Context.Value(key)
}

// appFromContext returns the App running the command that ctx belongs to,
// or nil.
func appFromContext(ctx context.Context) *App {
	if ctx == nil {
		return nil
	}
	app, _ := ctx.Value(appContextKey{}).(*App)
	return app
}

// promptsDisabled reports why app does not allow prompting, or "" when it
// does. InteractiveAuto keeps prompting from a non-terminal (a script can
// pipe answers in); only an explicit opt-out disables prompts.
func (a *App) promptsDisabled() string {
	if a == nil {
		return ""
	}
	if name := a.DisablePromptsEnvVar(); envTrue(os.Getenv(name)) {
		return name + " is set"
	}
	if a.Interactive == InteractiveNever {
		return "App.Interactive is InteractiveNever"
	}
	return ""
}

// NewPromptConfig builds the configuration a Prompter should use for one
// prompt, so custom prompters behave like the built-in ones:
//
//   - The theme starts from DefaultPromptTheme, overlaid with the running
//     App's DefaultTheme when ctx is a command's *Context or derives from
//     one; each option's theme is then merged field by field (see
//     PromptTheme.Merge), so a partial theme keeps the defaults it does not
//     set.
//   - The options are applied in order.
//
// It returns an error wrapping ErrPromptsDisabled when the App in ctx does
// not allow prompting, one wrapping ErrNoOptions when a select or
// multi-select prompt has an empty option list, and one wrapping
// ErrInvalidDefault when such a prompt's Default names no option. The
// configuration is returned in each case.
func NewPromptConfig(ctx context.Context, opts ...PromptOption) (*PromptConfig, error) {
	theme := DefaultPromptTheme
	app := appFromContext(ctx)
	if app != nil {
		theme = theme.Merge(app.DefaultTheme)
	}
	cfg := &PromptConfig{Theme: theme}
	for _, opt := range opts {
		if opt != nil {
			opt.Apply(cfg)
		}
	}
	if reason := app.promptsDisabled(); reason != "" {
		return cfg, fmt.Errorf("cannot prompt for %q: %w (%s)", cfg.Label, ErrPromptsDisabled, reason)
	}
	if cfg.Confirm {
		return cfg, nil
	}
	if (cfg.Options != nil || cfg.MultiSelect) && len(cfg.Options) == 0 {
		return cfg, fmt.Errorf("cannot prompt for %q: %w", cfg.Label, ErrNoOptions)
	}
	if len(cfg.Options) > 0 && cfg.Default != "" {
		values, labels := optionNames(cfg.Options)
		var ok bool
		if cfg.MultiSelect {
			_, ok = answer.MultiDefault(values, labels, cfg.Default)
		} else {
			_, ok = answer.SingleDefault(values, labels, cfg.Default)
		}
		if !ok {
			return cfg, fmt.Errorf("cannot prompt for %q: %w: %q", cfg.Label, ErrInvalidDefault, cfg.Default)
		}
	}
	return cfg, nil
}

func optionNames(options []SelectOption) (values, labels []string) {
	values = make([]string, len(options))
	labels = make([]string, len(options))
	for i, o := range options {
		values[i], labels[i] = o.Value, o.Label
	}
	return values, labels
}

// ResolveMultiSelectDefault returns the options a multi-select prompt with
// the given Default preselects, as indices into options. Default is a
// comma-separated list of option values or labels; when one of its parts
// is neither, all parts are read as 1-based option numbers instead (the
// older form, kept for compatibility). ok is false when Default is
// neither. Custom prompters can use it to read Default the same way.
func ResolveMultiSelectDefault(options []SelectOption, def string) (indices []int, ok bool) {
	values, labels := optionNames(options)
	return answer.MultiDefault(values, labels, def)
}

// Merge returns t with every field that o sets replaced by o's value:
// strings that are not empty and styles that are not nil. Fields o leaves
// unset keep t's value, so PromptTheme{LabelStyle: bold} changes only the
// label style.
func (t PromptTheme) Merge(o PromptTheme) PromptTheme {
	str := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	style := func(dst *TextStyle, v TextStyle) {
		if v != nil {
			*dst = v
		}
	}
	str(&t.Prefix, o.Prefix)
	str(&t.Hint, o.Hint)
	str(&t.Error, o.Error)
	style(&t.PrefixStyle, o.PrefixStyle)
	style(&t.LabelStyle, o.LabelStyle)
	style(&t.HintStyle, o.HintStyle)
	style(&t.DefaultStyle, o.DefaultStyle)
	style(&t.PlaceholderStyle, o.PlaceholderStyle)
	style(&t.SuggestionStyle, o.SuggestionStyle)
	style(&t.ErrorStyle, o.ErrorStyle)
	style(&t.ButtonActiveStyle, o.ButtonActiveStyle)
	style(&t.ButtonInactiveStyle, o.ButtonInactiveStyle)
	style(&t.ButtonHoverStyle, o.ButtonHoverStyle)
	style(&t.Buttons.Active, o.Buttons.Active)
	style(&t.Buttons.Inactive, o.Buttons.Inactive)
	style(&t.Buttons.Hover, o.Buttons.Hover)
	return t
}

// ReadLine reads one line of prompt input from r and returns it without
// its line ending ("\n" or "\r\n"). It never reads past the end of the
// line, so prompts and other code can share r. It returns ctx.Err() as soon
// as ctx is done; a line that arrives later goes to the next ReadLine on
// r. A last line without a newline is returned with a nil error; io.EOF is
// returned only when no input is left.
//
// Custom prompters can use it to honor cancellation the way the built-in
// prompters do.
func ReadLine(ctx context.Context, r io.Reader) (string, error) {
	line, err := readLine(ctx, r)
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

// promptStyler renders prompt text for one output: styles apply as usual,
// but their ANSI escapes are removed when the output should not get color
// (NO_COLOR, a pipe or file, TERM=dumb; see tty.ColorEnabled).
type promptStyler struct{ color bool }

func newPromptStyler(w io.Writer) promptStyler {
	return promptStyler{color: tty.ColorEnabled(w)}
}

func (s promptStyler) render(style TextStyle, value string) string {
	if style == nil {
		return value
	}
	out := style.Render(value)
	if !s.color {
		out = tty.StripANSI(out)
	}
	return out
}
