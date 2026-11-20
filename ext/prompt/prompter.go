package prompt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/SCKelemen/clix/v2"
)

func buttonActiveStyle(theme clix.PromptTheme) clix.TextStyle {
	if theme.Buttons.Active != nil {
		return theme.Buttons.Active
	}
	return theme.ButtonActiveStyle
}

func buttonInactiveStyle(theme clix.PromptTheme) clix.TextStyle {
	if theme.Buttons.Inactive != nil {
		return theme.Buttons.Inactive
	}
	return theme.ButtonInactiveStyle
}

func placeholderStyle(theme clix.PromptTheme) clix.TextStyle {
	if theme.PlaceholderStyle != nil {
		return theme.PlaceholderStyle
	}
	return theme.DefaultStyle
}

func suggestionStyle(theme clix.PromptTheme) clix.TextStyle {
	if theme.SuggestionStyle != nil {
		return theme.SuggestionStyle
	}
	if theme.PlaceholderStyle != nil {
		return theme.PlaceholderStyle
	}
	return theme.DefaultStyle
}

func placeholderText(cfg *clix.PromptConfig) string {
	if cfg.Default != "" {
		return cfg.Default
	}
	return cfg.NoDefaultPlaceholder
}

func suggestionText(cfg *clix.PromptConfig, currentInput string) string {
	if cfg.Default != "" {
		if currentInput == "" {
			return cfg.Default
		}
		if strings.HasPrefix(cfg.Default, currentInput) {
			return cfg.Default[len(currentInput):]
		}
		return ""
	}

	// No default - don't show any suggestion text
	// (Users can just press Enter, which is a common CLI pattern)
	return ""
}

func dispatchCommand(cfg *clix.PromptConfig, state clix.PromptKeyState, setInput func(string)) clix.PromptCommandAction {
	if setInput == nil {
		setInput = func(string) {}
	}
	ctx := clix.PromptCommandContext{
		PromptKeyState: state,
		SetInput:       setInput,
	}

	if binding, ok := cfg.KeyMap.BindingFor(state.Command); ok {
		if binding.Active != nil && !binding.Active(state) {
			return clix.PromptCommandAction{Handled: true}
		}
		if binding.Handler != nil {
			action := binding.Handler(ctx)
			if action.Exit || action.Handled {
				return action
			}
		}
	}

	if cfg.CommandHandler != nil {
		return cfg.CommandHandler(ctx)
	}

	return clix.PromptCommandAction{}
}

func functionKeyNumber(key Key) int {
	switch key {
	case KeyF1:
		return 1
	case KeyF2:
		return 2
	case KeyF3:
		return 3
	case KeyF4:
		return 4
	case KeyF5:
		return 5
	case KeyF6:
		return 6
	case KeyF7:
		return 7
	case KeyF8:
		return 8
	case KeyF9:
		return 9
	case KeyF10:
		return 10
	case KeyF11:
		return 11
	case KeyF12:
		return 12
	default:
		return 0
	}
}

func commandLabel(cmd clix.PromptCommand) string {
	switch cmd.Type {
	case clix.PromptCommandEscape:
		return "ESC"
	case clix.PromptCommandTab:
		return "Tab"
	case clix.PromptCommandEnter:
		return "Enter"
	case clix.PromptCommandFunction:
		if cmd.FunctionKey > 0 {
			return fmt.Sprintf("F%d", cmd.FunctionKey)
		}
	}
	return ""
}

func renderHintLine(cfg *clix.PromptConfig, baseState clix.PromptKeyState) string {
	if len(cfg.KeyMap.Bindings) == 0 {
		return ""
	}

	var hints []string
	for _, binding := range cfg.KeyMap.Bindings {
		label := commandLabel(binding.Command)
		if label == "" {
			continue
		}

		state := baseState
		state.Command = binding.Command

		active := true
		if binding.Active != nil {
			active = binding.Active(state)
		}

		hint := fmt.Sprintf("[ %s ] %s", label, binding.Description)
		style := buttonActiveStyle(cfg.Theme)
		if !active {
			style = buttonInactiveStyle(cfg.Theme)
		}
		if style != nil {
			hint = renderText(style, hint)
		}
		hints = append(hints, hint)
	}

	if len(hints) == 0 {
		return ""
	}

	hintText := strings.Join(hints, "    ")
	if cfg.Theme.HintStyle != nil {
		hintText = renderText(cfg.Theme.HintStyle, hintText)
	}

	return hintText
}

// TerminalPrompter implements Prompter with full support for text, secret,
// select, multi-select, and confirm prompts.
//
// When In and Out are both terminals (and TERM is not "dumb"), prompts run
// in raw mode: keys are read one at a time, the prompt is redrawn in place,
// arrow keys and the other keys listed in docs/prompting-and-terminals.md
// work, bracketed paste inserts text without acting on it, long option
// lists scroll, and key bindings (WithKeyMap, WithCommandHandler) are
// honored for Esc, Tab, Enter and F1-F12. Ctrl-C always cancels with
// clix.ErrCancelled and cannot be bound. On Windows, raw mode needs a
// console with virtual terminal support (Windows 10 1511 or later, Windows
// Terminal); legacy consoles use line mode.
//
// Otherwise prompts run in line mode: one line is read per answer, with the
// same answer rules, and key bindings are not used. Either way prompts
// return ctx.Err() as soon as ctx is done.
type TerminalPrompter struct {
	In  io.Reader
	Out io.Writer
}

// Prompt displays a prompt and reads the user's response.
// Supports all prompt types: text, secret, select, multi-select, and confirm.
func (p TerminalPrompter) Prompt(ctx context.Context, opts ...clix.PromptOption) (string, error) {
	if p.In == nil || p.Out == nil {
		return "", errors.New("prompter missing IO")
	}
	cfg, err := clix.NewPromptConfig(ctx, opts...)
	if err != nil {
		return "", err
	}
	return p.prompt(ctx, cfg)
}

func (p TerminalPrompter) prompt(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	if p.In == nil || p.Out == nil {
		return "", errors.New("prompter missing IO")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	multi := cfg.MultiSelect && !cfg.Confirm
	choice := len(cfg.Options) > 0 && !cfg.Confirm

	if s, ok := p.startRaw(); ok {
		defer s.close()
		switch {
		case cfg.Confirm:
			return s.text(ctx, cfg, true)
		case choice:
			return s.choose(ctx, cfg, multi)
		default:
			return s.text(ctx, cfg, false)
		}
	}

	l := p.linePrompt()
	switch {
	case cfg.Confirm:
		return l.confirm(ctx, cfg)
	case choice && multi:
		return l.chooseMany(ctx, cfg)
	case choice:
		return l.choose(ctx, cfg)
	default:
		return l.text(ctx, cfg)
	}
}

// renderText renders text with optional styling.
func renderText(style clix.TextStyle, value string) string {
	if style == nil {
		return value
	}
	return style.Render(value)
}

// singleDefault returns the index of the option Default names (by value,
// then label), or -1. NewPromptConfig has already rejected a Default that
// names no option.
func singleDefault(cfg *clix.PromptConfig) int {
	if cfg.Default == "" {
		return -1
	}
	for i, o := range cfg.Options {
		if o.Value == cfg.Default {
			return i
		}
	}
	for i, o := range cfg.Options {
		if o.Label == cfg.Default {
			return i
		}
	}
	return -1
}

// multiDefault returns the options a multi-select Default preselects: a
// comma-separated list of values or labels, or (when none of the parts is a
// value or label) of 1-based option numbers.
func multiDefault(cfg *clix.PromptConfig) []int {
	idx, _ := clix.ResolveMultiSelectDefault(cfg.Options, cfg.Default)
	return idx
}

// formatSelected joins the values of the selected options with commas, in
// option order; an option without a Value contributes its Label.
func formatSelected(options []clix.SelectOption, selected map[int]bool) string {
	var values []string
	for i, opt := range options {
		if !selected[i] {
			continue
		}
		if opt.Value != "" {
			values = append(values, opt.Value)
		} else {
			values = append(values, opt.Label)
		}
	}
	return strings.Join(values, ",")
}
