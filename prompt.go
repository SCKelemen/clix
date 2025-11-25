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
// Example:
//
//	// Basic text prompt
//	result, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
//		Label:   "Name",
//		Default: "unknown",
//		Validate: validation.NotEmpty,
//	})
//
//	// Select prompt (requires prompt extension)
//	result, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
//		Label: "Choose an option",
//		Options: []clix.SelectOption{
//			{Label: "Option A", Value: "a"},
//			{Label: "Option B", Value: "b"},
//		},
//	})
//
//	// Confirm prompt
//	result, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
//		Label:   "Continue?",
//		Confirm: true,
//	})
type PromptRequest struct {
	// Label is the prompt text shown to the user.
	Label string

	// Default is the default value shown in the prompt.
	// For text prompts, this appears as placeholder text.
	Default string

	// NoDefaultPlaceholder is custom placeholder text when Default is empty.
	NoDefaultPlaceholder string

	// Validate is an optional validation function called when the user submits input.
	// Return an error if the value is invalid.
	Validate func(string) error

	// Theme configures the styling for this prompt.
	// If not set, uses app.DefaultTheme.
	Theme PromptTheme

	// Options are the choices for select/multi-select prompts.
	// Requires the prompt extension for advanced prompt types.
	Options []SelectOption

	// MultiSelect enables multi-select mode (user can choose multiple options).
	// Requires the prompt extension.
	MultiSelect bool

	// Confirm enables confirm mode (yes/no prompt).
	// Returns "y" or "n" (or "yes"/"no").
	Confirm bool

	// ContinueText is the text shown for the continue button in select prompts.
	ContinueText string

	// CommandHandler allows custom command handling during prompts.
	// Users can type commands that are processed by this handler.
	CommandHandler PromptCommandHandler

	// KeyMap configures keyboard shortcuts for the prompt.
	KeyMap PromptKeyMap
}

// Apply implements PromptOption so PromptRequest can be used directly.
func (r PromptRequest) Apply(cfg *PromptConfig) {
	if r.Label != "" {
		cfg.Label = r.Label
	}
	if r.Default != "" {
		cfg.Default = r.Default
	}
	if r.NoDefaultPlaceholder != "" {
		cfg.NoDefaultPlaceholder = r.NoDefaultPlaceholder
	}
	if r.Validate != nil {
		cfg.Validate = r.Validate
	}
	if r.Theme.isConfigured() {
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
	if r.CommandHandler != nil {
		cfg.CommandHandler = r.CommandHandler
	}
	if r.KeyMap.isConfigured() {
		cfg.KeyMap = r.KeyMap
	}
}

// PromptConfig holds all prompt configuration internally.
// Exported so extension packages can implement PromptOption.
type PromptConfig struct {
	Label                string
	Default              string
	NoDefaultPlaceholder string
	Validate             func(string) error
	Theme                PromptTheme
	Options              []SelectOption
	MultiSelect          bool
	Confirm              bool
	ContinueText         string
	CommandHandler       PromptCommandHandler
	KeyMap               PromptKeyMap
}

// PromptCommandType identifies a special key command intercepted by interactive prompts.
type PromptCommandType int

const (
	// PromptCommandUnknown represents an unclassified key.
	PromptCommandUnknown PromptCommandType = iota
	// PromptCommandEscape indicates the escape key was pressed.
	PromptCommandEscape
	// PromptCommandTab indicates the tab key was pressed.
	PromptCommandTab
	// PromptCommandFunction indicates an F-key (F1-F12) was pressed.
	PromptCommandFunction
	// PromptCommandEnter indicates the enter key was pressed.
	PromptCommandEnter
)

// PromptCommand describes a high-level command initiated by the user.
// For function keys, FunctionKey contains the key index (1-12).
type PromptCommand struct {
	Type        PromptCommandType
	FunctionKey int
}

// PromptCommandAction instructs the prompter how to proceed after a command is handled.
type PromptCommandAction struct {
	// Handled indicates the command was consumed and default handling should be skipped.
	Handled bool
	// Exit requests the prompter to exit immediately.
	// If ExitErr is non-nil it will be returned from the prompt.
	Exit bool
	// ExitErr is returned from the prompt when Exit is true.
	ExitErr error
}

// PromptKeyState describes the prompt state when evaluating key bindings.
type PromptKeyState struct {
	Command    PromptCommand
	Input      string
	Default    string
	Suggestion string
}

// PromptCommandContext provides the handler context when a key binding is invoked.
type PromptCommandContext struct {
	PromptKeyState
	SetInput func(string)
}

// PromptCommandHandler processes special key commands during an interactive prompt.
// Returning an action with Exit=true stops the prompt immediately.
type PromptCommandHandler func(PromptCommandContext) PromptCommandAction

// PromptKeyBinding maps a command to display metadata and optional handling.
type PromptKeyBinding struct {
	Command     PromptCommand
	Description string
	Handler     PromptCommandHandler
	Active      func(PromptKeyState) bool
}

// PromptKeyMap holds the configured key bindings for a prompt.
type PromptKeyMap struct {
	Bindings []PromptKeyBinding
}

func (m PromptKeyMap) isConfigured() bool {
	return len(m.Bindings) > 0
}

// BindingFor returns the configured binding for the given command, if any.
func (m PromptKeyMap) BindingFor(cmd PromptCommand) (PromptKeyBinding, bool) {
	for _, binding := range m.Bindings {
		if binding.Command.Type != cmd.Type {
			continue
		}
		if binding.Command.Type == PromptCommandFunction && binding.Command.FunctionKey != cmd.FunctionKey {
			continue
		}
		return binding, true
	}
	return PromptKeyBinding{}, false
}

// TextPromptOption implements PromptOption for basic text prompts.
// These options work with all prompters.
type TextPromptOption func(*PromptConfig)

func (o TextPromptOption) Apply(cfg *PromptConfig) {
	o(cfg)
}

// WithLabel sets the prompt label (functional option).
//
// Example:
//
//	result, err := prompter.Prompt(ctx,
//		clix.WithLabel("Enter your name"),
//	)
func WithLabel(label string) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.Label = label
	})
}

// WithDefault sets the default value (functional option).
// The default is shown in the prompt and used if the user presses Enter without input.
//
// Example:
//
//	result, err := prompter.Prompt(ctx,
//		clix.WithLabel("Color"),
//		clix.WithDefault("blue"),
//	)
func WithDefault(def string) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.Default = def
	})
}

// WithCommandHandler registers a handler for special key commands during prompts.
// Command handlers can intercept escape, tab, function keys, and enter to provide
// custom behavior (e.g., autocomplete, help, cancellation).
//
// Example:
//
//	result, err := prompter.Prompt(ctx,
//		clix.WithLabel("Enter value"),
//		clix.WithCommandHandler(func(ctx clix.PromptCommandContext) clix.PromptCommandAction {
//			if ctx.Command.Type == clix.PromptCommandEscape {
//				return clix.PromptCommandAction{Exit: true, ExitErr: errors.New("cancelled")}
//			}
//			return clix.PromptCommandAction{Handled: false}
//		}),
//	)
func WithCommandHandler(handler PromptCommandHandler) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.CommandHandler = handler
	})
}

// WithKeyMap configures the key bindings shown and invoked by the prompt.
// Key maps allow you to define keyboard shortcuts with descriptions and handlers.
//
// Example:
//
//	keyMap := clix.PromptKeyMap{
//		Bindings: []clix.PromptKeyBinding{
//			{
//				Command:     clix.PromptCommand{Type: clix.PromptCommandEscape},
//				Description: "Cancel",
//				Handler: func(ctx clix.PromptCommandContext) clix.PromptCommandAction {
//					return clix.PromptCommandAction{Exit: true}
//				},
//			},
//		},
//	}
//	result, err := prompter.Prompt(ctx,
//		clix.WithLabel("Enter value"),
//		clix.WithKeyMap(keyMap),
//	)
func WithKeyMap(m PromptKeyMap) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		if m.isConfigured() {
			cfg.KeyMap = m
		}
	})
}

// WithNoDefaultPlaceholder sets the placeholder text shown when no default exists.
// This is typically used by higher-level workflows (like surveys) to prompt the
// user that pressing enter will keep their existing value.
//
// Example:
//
//	result, err := prompter.Prompt(ctx,
//		clix.WithLabel("Enter value"),
//		clix.WithNoDefaultPlaceholder("(press Enter to keep current value)"),
//	)
func WithNoDefaultPlaceholder(text string) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.NoDefaultPlaceholder = text
	})
}

// WithValidate sets the validation function (functional option).
// The validation function is called when the user submits input.
// Return an error if the value is invalid; the prompt will re-prompt until valid.
//
// Example:
//
//	result, err := prompter.Prompt(ctx,
//		clix.WithLabel("Enter email"),
//		clix.WithValidate(func(value string) error {
//			if !strings.Contains(value, "@") {
//				return errors.New("invalid email address")
//			}
//			return nil
//		}),
//	)
func WithValidate(validate func(string) error) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.Validate = validate
	})
}

// WithTheme sets the prompt theme (functional option).
// Themes control the visual appearance of prompts (prefix, hint, error indicators, styling).
// If not set, uses app.DefaultTheme.
//
// Example:
//
//	theme := clix.PromptTheme{
//		Prefix: "> ",
//		Error:  "✗ ",
//	}
//	result, err := prompter.Prompt(ctx,
//		clix.WithLabel("Name"),
//		clix.WithTheme(theme),
//	)
func WithTheme(theme PromptTheme) PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.Theme = theme
	})
}

// WithConfirm enables yes/no confirmation prompt mode (functional option).
// Works with TextPrompter - it's just a text prompt with y/n validation.
// Returns "y" or "n" (or "yes"/"no").
//
// Example:
//
//	result, err := prompter.Prompt(ctx,
//		clix.WithLabel("Continue?"),
//		clix.WithConfirm(),
//	)
//	if result == "y" {
//		// User confirmed
//	}
func WithConfirm() PromptOption {
	return TextPromptOption(func(cfg *PromptConfig) {
		cfg.Confirm = true
	})
}

// SelectOption represents a choice in a select or multi-select prompt.
//
// Example:
//
//	result, err := prompter.Prompt(ctx, clix.PromptRequest{
//		Label: "Choose an option",
//		Options: []clix.SelectOption{
//			{Label: "Option A", Value: "a", Description: "First option"},
//			{Label: "Option B", Value: "b", Description: "Second option"},
//		},
//	})
type SelectOption struct {
	// Label is the text displayed to the user for this option.
	Label string

	// Value is the value returned when this option is selected.
	Value string

	// Description is optional additional text shown below the label.
	Description string
}

// PromptTheme configures the visual appearance of prompts.
// Themes control the prefix, hint, error indicators, and styling for all prompt elements.
//
// Example:
//
//	theme := clix.PromptTheme{
//		Prefix: "> ",
//		Hint:   "(press Enter to confirm)",
//		Error:  "✗ ",
//		LabelStyle: lipgloss.NewStyle().Bold(true),
//		ErrorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
//	}
//	result, err := prompter.Prompt(ctx, clix.PromptRequest{
//		Label: "Name",
//		Theme: theme,
//	})
type PromptTheme struct {
	// Prefix is the text shown before the prompt label (e.g., "? ", "> ").
	Prefix string

	// Hint is the text shown as a hint below the prompt (e.g., "(optional)").
	Hint string

	// Error is the text shown before error messages (e.g., "! ", "✗ ").
	Error string

	// PrefixStyle styles the prefix text.
	PrefixStyle TextStyle

	// LabelStyle styles the prompt label text.
	LabelStyle TextStyle

	// HintStyle styles the hint text.
	HintStyle TextStyle

	// DefaultStyle styles the default value text.
	DefaultStyle TextStyle

	// PlaceholderStyle styles placeholder/default text (e.g., bracketed defaults).
	PlaceholderStyle TextStyle

	// SuggestionStyle styles inline suggestion/ghost text.
	SuggestionStyle TextStyle

	// ErrorStyle styles error messages.
	ErrorStyle TextStyle

	// ButtonActiveStyle styles active button hints.
	ButtonActiveStyle TextStyle

	// ButtonInactiveStyle styles inactive/grayed-out button hints.
	ButtonInactiveStyle TextStyle

	// ButtonHoverStyle styles hovered button hints.
	ButtonHoverStyle TextStyle

	// Buttons groups button hint styles together for easier configuration.
	Buttons PromptButtonStyles
}

// PromptButtonStyles groups button hint styles together for easier configuration.
// These styles are used for keyboard shortcut hints in prompts.
type PromptButtonStyles struct {
	// Active styles active button hints.
	Active TextStyle

	// Inactive styles inactive/grayed-out button hints.
	Inactive TextStyle

	// Hover styles hovered button hints.
	Hover TextStyle
}

func (t PromptTheme) isConfigured() bool {
	return t.Prefix != "" ||
		t.Hint != "" ||
		t.Error != "" ||
		t.PrefixStyle != nil ||
		t.LabelStyle != nil ||
		t.HintStyle != nil ||
		t.DefaultStyle != nil ||
		t.PlaceholderStyle != nil ||
		t.SuggestionStyle != nil ||
		t.ErrorStyle != nil ||
		t.ButtonActiveStyle != nil ||
		t.ButtonInactiveStyle != nil ||
		t.ButtonHoverStyle != nil ||
		t.Buttons.Active != nil ||
		t.Buttons.Inactive != nil ||
		t.Buttons.Hover != nil
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
// TextPrompter is the default prompt implementation.
// It supports basic text input and confirm prompts.
// For advanced prompts (select, multi-select), use the prompt extension
// which provides TerminalPrompter.
//
// Example:
//
//	app.Prompter = clix.TextPrompter{
//		In:  os.Stdin,
//		Out: os.Stdout,
//	}
type TextPrompter struct {
	// In is the reader for user input (typically os.Stdin).
	In io.Reader

	// Out is the writer for prompt output (typically os.Stdout).
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

// Builder-style methods for PromptRequest (fluent API)

// SetLabel sets the prompt label and returns the request for method chaining.
func (r *PromptRequest) SetLabel(label string) *PromptRequest {
	r.Label = label
	return r
}

// SetDefault sets the prompt default value and returns the request for method chaining.
func (r *PromptRequest) SetDefault(defaultValue string) *PromptRequest {
	r.Default = defaultValue
	return r
}

// SetNoDefaultPlaceholder sets the no-default placeholder and returns the request for method chaining.
func (r *PromptRequest) SetNoDefaultPlaceholder(placeholder string) *PromptRequest {
	r.NoDefaultPlaceholder = placeholder
	return r
}

// SetValidate sets the validation function and returns the request for method chaining.
func (r *PromptRequest) SetValidate(validate func(string) error) *PromptRequest {
	r.Validate = validate
	return r
}

// SetTheme sets the prompt theme and returns the request for method chaining.
func (r *PromptRequest) SetTheme(theme PromptTheme) *PromptRequest {
	r.Theme = theme
	return r
}

// SetOptions sets the prompt options and returns the request for method chaining.
func (r *PromptRequest) SetOptions(options ...SelectOption) *PromptRequest {
	r.Options = options
	return r
}

// SetMultiSelect enables multi-select mode and returns the request for method chaining.
func (r *PromptRequest) SetMultiSelect(multiSelect bool) *PromptRequest {
	r.MultiSelect = multiSelect
	return r
}

// SetConfirm enables confirm mode and returns the request for method chaining.
func (r *PromptRequest) SetConfirm(confirm bool) *PromptRequest {
	r.Confirm = confirm
	return r
}

// SetContinueText sets the continue text and returns the request for method chaining.
func (r *PromptRequest) SetContinueText(text string) *PromptRequest {
	r.ContinueText = text
	return r
}

// SetCommandHandler sets the command handler and returns the request for method chaining.
func (r *PromptRequest) SetCommandHandler(handler PromptCommandHandler) *PromptRequest {
	r.CommandHandler = handler
	return r
}

// SetKeyMap sets the key map and returns the request for method chaining.
func (r *PromptRequest) SetKeyMap(keyMap PromptKeyMap) *PromptRequest {
	r.KeyMap = keyMap
	return r
}
