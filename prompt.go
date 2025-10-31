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
type Prompter interface {
	Prompt(ctx context.Context, req PromptRequest) (string, error)
}

// PromptRequest carries the information necessary to display a prompt.
// Advanced fields (Options, MultiSelect, Confirm) are handled by the prompt extension.
type PromptRequest struct {
	Label    string
	Default  string
	Validate func(string) error
	Theme    PromptTheme

	// Options for select-style prompts (navigable list)
	// Handled by prompt extension
	Options []SelectOption

	// MultiSelect enables multi-selection mode when Options are provided
	// Handled by prompt extension
	MultiSelect bool

	// Confirm is for yes/no confirmation prompts
	// Handled by prompt extension
	Confirm bool

	// ContinueText is the text shown for the continue/next/done action in multi-select prompts
	// Handled by prompt extension
	ContinueText string
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

	PrefixStyle  TextStyle
	LabelStyle   TextStyle
	HintStyle    TextStyle
	DefaultStyle TextStyle
	ErrorStyle   TextStyle
}

// DefaultPromptTheme provides a sensible default for terminal prompts.
var DefaultPromptTheme = PromptTheme{
	Prefix: "? ",
	Hint:   "",
	Error:  "! ",
}

// SimpleTextPrompter implements Prompter for basic text input only.
// This is the default prompter in core - it only handles text prompts.
// For advanced prompts (select, multi-select, confirm), use the prompt extension.
type SimpleTextPrompter struct {
	In  io.Reader
	Out io.Writer
}

// Prompt displays a text prompt and reads the user's response.
// Advanced prompt types (Options, MultiSelect, Confirm) are ignored by SimpleTextPrompter.
// Use the prompt extension for advanced prompt types.
func (p SimpleTextPrompter) Prompt(ctx context.Context, req PromptRequest) (string, error) {
	if p.In == nil || p.Out == nil {
		return "", errors.New("prompter missing IO")
	}

	// SimpleTextPrompter only handles text prompts
	// Advanced prompt types should use the prompt extension
	if req.Confirm {
		return "", errors.New("confirm prompts require the prompt extension (clix/ext/prompt)")
	}
	if len(req.Options) > 0 {
		if req.MultiSelect {
			return "", errors.New("multi-select prompts require the prompt extension (clix/ext/prompt)")
		}
		return "", errors.New("select prompts require the prompt extension (clix/ext/prompt)")
	}

	return p.promptText(ctx, req)
}

// promptText handles regular text input prompts.
func (p SimpleTextPrompter) promptText(ctx context.Context, req PromptRequest) (string, error) {
	reader := bufio.NewReader(p.In)

	for {
		prefix := renderText(req.Theme.PrefixStyle, req.Theme.Prefix)
		label := renderText(req.Theme.LabelStyle, req.Label)
		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		if req.Default != "" {
			def := renderText(req.Theme.DefaultStyle, req.Default)
			fmt.Fprintf(p.Out, " [%s]", def)
		}

		if req.Theme.Hint != "" {
			hint := renderText(req.Theme.HintStyle, req.Theme.Hint)
			fmt.Fprintf(p.Out, " %s", hint)
		}

		fmt.Fprint(p.Out, ": ")

		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		value := strings.TrimSpace(line)
		if value == "" {
			value = req.Default
		}

		if req.Validate != nil {
			if err := req.Validate(value); err != nil {
				errPrefix := renderText(req.Theme.ErrorStyle, req.Theme.Error)
				errMsg := err.Error()
				if errMsg != "" {
					errMsg = renderText(req.Theme.ErrorStyle, errMsg)
				}
				fmt.Fprintf(p.Out, "%s%s\n", errPrefix, errMsg)
				continue
			}
		}

		return value, nil
	}
}
