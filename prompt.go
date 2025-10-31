package clix

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Prompter encapsulates interactive prompting.
// Accepts either functional options or a PromptRequest struct:
//
//	// Struct-based (primary API, consistent with rest of codebase)
//	prompter.Prompt(ctx, PromptRequest{Label: "Name", Default: "unknown"})
//
//	// Functional options (convenience layer)
//	prompter.Prompt(ctx, WithLabel("Name"), WithDefault("unknown"))
type Prompter interface {
	Prompt(ctx context.Context, opts ...PromptOption) (string, error)
}

// PromptOption configures a prompt using the functional options pattern.
// Options can be used to build prompts:
//
//	// Basic text prompt
//	prompter.Prompt(ctx, WithLabel("Name"), WithDefault("unknown"))
//
//	// Advanced prompts (require prompt extension):
//	// prompter.Prompt(ctx, WithLabel("Choose"), Select([]SelectOption{...}))
type PromptOption interface {
	// Apply configures the prompt config.
	// Exported so extension packages can implement PromptOption.
	Apply(*PromptConfig)
}

// PromptRequest is the struct-based API for prompts, consistent with the rest of the codebase.
// This is the primary API - functional options are a convenience layer.
//
//	// Basic text prompt
//	prompter.Prompt(ctx, PromptRequest{
//		Label: "Name",
//		Default: "unknown",
//	})
//
//	// Advanced prompts (require prompt extension):
//	// prompter.Prompt(ctx, PromptRequest{
//	//     Label: "Choose",
//	//     Options: []SelectOption{{Label: "A", Value: "a"}},
//	// })
type PromptRequest struct {
	Label        string
	Default      string
	Validate     func(string) error
	Theme        PromptTheme
	Options      []SelectOption
	MultiSelect  bool
	Confirm      bool
	ContinueText string
}

// Apply implements PromptOption so PromptRequest can be used directly.
func (r PromptRequest) Apply(cfg *PromptConfig) {
	if r.Label != "" {
		cfg.Label = r.Label
	}
	if r.Default != "" {
		cfg.Default = r.Default
	}
	if r.Validate != nil {
		cfg.Validate = r.Validate
	}
	if r.Theme.Prefix != "" || r.Theme.PrefixStyle != nil {
		cfg.Theme = r.Theme
	}
	if len(r.Options) > 0 {
		cfg.Options = r.Options
	}
	if r.MultiSelect {
		cfg.MultiSelect = true
	}
	if r.Confirm {
		cfg.Confirm = true
	}
	if r.ContinueText != "" {
		cfg.ContinueText = r.ContinueText
	}
}

// PromptConfig holds all prompt configuration internally.
// Exported so extension packages can implement PromptOption.
type PromptConfig struct {
	Label        string
	Default      string
	Validate     func(string) error
	Theme        PromptTheme
	Options      []SelectOption
	MultiSelect  bool
	Confirm      bool
	ContinueText string
	// OnEscape is called when the Escape key is pressed.
	// If it returns an error, that error is returned instead of "cancelled".
	// Extensions (like survey) can use this to handle custom behavior for the Escape key.
	// Note: This refers to the keyboard Escape key press, not ANSI escape code sequences.
	OnEscape func() error
	// OnFunctionKey is called when any F key (F1-F12) is pressed.
	// The Key parameter indicates which F key was pressed (KeyF1 through KeyF12).
	// If it returns an error, that error is returned instead of "cancelled".
	// Extensions can use this to bind specific F keys to custom actions.
	// Note: Key type is from clix/ext/prompt package - use prompt.KeyF1, etc.
	OnFunctionKey func(key interface{}) error
}

// TextPromptOption implements PromptOption for basic text prompts.
// These options work with all prompters.
type TextPromptOption func(*PromptConfig)

func (o TextPromptOption) Apply(cfg *PromptConfig) {
	o(cfg)
}

// WithLabel sets the prompt label (functional option).
func WithLabel(label string) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.Label = label
	})
}

// WithDefault sets the default value (functional option).
func WithDefault(def string) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.Default = def
	})
}

// WithValidate sets the validation function (functional option).
func WithValidate(validate func(string) error) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.Validate = validate
	})
}

// WithTheme sets the prompt theme (functional option).
func WithTheme(theme PromptTheme) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.Theme = theme
	})
}

// WithConfirm enables yes/no confirmation prompt mode (functional option).
// Works with TextPrompter - it's just a text prompt with y/n validation.
func WithConfirm() PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.Confirm = true
	})
}

// SelectOption represents a single option in a select prompt.
// Used by prompt extension.
type SelectOption struct {
	Label       string // Display label
	Value       string // Return value when selected
	Description string // Optional description shown below label
}

// PromptTheme defines how prompts are styled.
type PromptTheme struct {
	Prefix string
	Hint   string
	Error  string

	PrefixStyle         TextStyle
	LabelStyle          TextStyle
	HintStyle           TextStyle
	DefaultStyle        TextStyle
	PlaceholderStyle    TextStyle // Style for placeholder/default text
	ErrorStyle          TextStyle
	ButtonActiveStyle   TextStyle // Style for active button hints
	ButtonInactiveStyle TextStyle // Style for inactive/grayed-out button hints
	ButtonHoverStyle    TextStyle // Style for hovered button hints
}

// DefaultPromptTheme provides a sensible default for terminal prompts.
var DefaultPromptTheme = PromptTheme{
	Prefix: "? ",
	Hint:   "",
	Error:  "! ",
}

// TextPrompter implements Prompter for basic text input only.
// This is the default prompter in core - it only handles text prompts.
// Advanced prompt options (Select, MultiSelect, Confirm) are rejected at runtime
// with clear error messages directing users to the prompt extension.
type TextPrompter struct {
	In  io.Reader
	Out io.Writer
}

// Prompt displays a text prompt and reads the user's response.
// Accepts both struct-based PromptRequest and functional options for flexibility.
// Advanced prompt options (Select, MultiSelect) are rejected - use the prompt extension for those.
func (p TextPrompter) Prompt(ctx context.Context, opts ...PromptOption) (string, error) {
	if p.In == nil || p.Out == nil {
		return "", errors.New("prompter missing IO")
	}

	cfg := &PromptConfig{Theme: DefaultPromptTheme}

	// Check if first argument is a PromptRequest struct (backward compatibility check)
	// In practice, we'll support both: functional options and direct PromptRequest
	// For now, we only support functional options, but the interface allows for struct support later

	for _, opt := range opts {
		opt.Apply(cfg)
	}

	// Handle confirm prompt (works with TextPrompter)
	if cfg.Confirm {
		return p.promptConfirm(ctx, cfg)
	}

	// Reject advanced prompt types
	if len(cfg.Options) > 0 {
		if cfg.MultiSelect {
			return "", errors.New("multi-select prompts require the prompt extension (clix/ext/prompt)")
		}
		return "", errors.New("select prompts require the prompt extension (clix/ext/prompt)")
	}

	return p.promptText(ctx, cfg)
}

// promptText handles regular text input prompts.
func (p TextPrompter) promptText(ctx context.Context, cfg *PromptConfig) (string, error) {
	reader := bufio.NewReader(p.In)

	for {
		prefix := renderText(cfg.Theme.PrefixStyle, cfg.Theme.Prefix)
		label := renderText(cfg.Theme.LabelStyle, cfg.Label)
		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		if cfg.Default != "" {
			def := renderText(cfg.Theme.DefaultStyle, cfg.Default)
			fmt.Fprintf(p.Out, " [%s]", def)
		}

		if cfg.Theme.Hint != "" {
			hint := renderText(cfg.Theme.HintStyle, cfg.Theme.Hint)
			fmt.Fprintf(p.Out, " %s", hint)
		}

		fmt.Fprint(p.Out, ": ")

		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		value := strings.TrimSpace(line)
		if value == "" {
			value = cfg.Default
		}

		if cfg.Validate != nil {
			if err := cfg.Validate(value); err != nil {
				errPrefix := renderText(cfg.Theme.ErrorStyle, cfg.Theme.Error)
				errMsg := err.Error()
				if errMsg != "" {
					errMsg = renderText(cfg.Theme.ErrorStyle, errMsg)
				}
				fmt.Fprintf(p.Out, "%s%s\n", errPrefix, errMsg)
				continue
			}
		}

		return value, nil
	}
}

// promptConfirm handles yes/no confirmation prompts.
// This works with TextPrompter since it's just a text prompt with validation.
func (p TextPrompter) promptConfirm(ctx context.Context, cfg *PromptConfig) (string, error) {
	reader := bufio.NewReader(p.In)

	// Determine default (Y/n or y/N)
	defaultYes := true
	defaultText := "Y"
	if cfg.Default == "n" || cfg.Default == "N" || strings.ToLower(cfg.Default) == "no" {
		defaultYes = false
		defaultText = "N"
	}

	for {
		prefix := renderText(cfg.Theme.PrefixStyle, cfg.Theme.Prefix)
		label := renderText(cfg.Theme.LabelStyle, cfg.Label)
		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		// Show default in prompt
		if defaultYes {
			fmt.Fprintf(p.Out, " (%s/n)", defaultText)
		} else {
			fmt.Fprintf(p.Out, " (y/%s)", defaultText)
		}

		// Show hint if provided (may include "back" instruction from survey)
		if cfg.Theme.Hint != "" {
			hint := renderText(cfg.Theme.HintStyle, cfg.Theme.Hint)
			fmt.Fprintf(p.Out, " %s", hint)
		}

		fmt.Fprint(p.Out, ": ")

		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		value := strings.TrimSpace(line)
		if value == "" {
			// Return default
			if defaultYes {
				return "y", nil
			}
			return "n", nil
		}

		// Normalize response (but preserve case for special commands like "back")
		lowerValue := strings.ToLower(value)
		if lowerValue == "y" || lowerValue == "yes" {
			return "y", nil
		}
		if lowerValue == "n" || lowerValue == "no" {
			return "n", nil
		}

		// Allow "back" to pass through for undo functionality (survey will handle it)
		if lowerValue == "back" {
			return value, nil // Return original case
		}

		// Invalid input
		errPrefix := renderText(cfg.Theme.ErrorStyle, cfg.Theme.Error)
		errMsg := "please enter 'y' or 'n'"
		if cfg.Theme.ErrorStyle != nil {
			errMsg = renderText(cfg.Theme.ErrorStyle, errMsg)
		}
		fmt.Fprintf(p.Out, "%s%s\n", errPrefix, errMsg)
	}
}
