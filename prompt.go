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
type PromptRequest struct {
	Label    string
	Default  string
	Validate func(string) error
	Theme    PromptTheme

	// Options for select-style prompts (navigable list)
	// If provided, displays as a selectable list
	Options []SelectOption

	// MultiSelect enables multi-selection mode when Options are provided
	// When true, allows selecting multiple options and returns comma-separated values
	MultiSelect bool

	// Confirm is for yes/no confirmation prompts
	// If true, prompts with Y/n default (true) or y/N default (false if Default is "n")
	Confirm bool
}

// SelectOption represents a single option in a select prompt.
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

// TerminalPrompter implements Prompter using standard input/output streams.
type TerminalPrompter struct {
	In  io.Reader
	Out io.Writer
}

// Prompt displays a prompt and reads the user's response.
func (p TerminalPrompter) Prompt(ctx context.Context, req PromptRequest) (string, error) {
	if p.In == nil || p.Out == nil {
		return "", errors.New("prompter missing IO")
	}

	// Handle confirmation prompt
	if req.Confirm {
		return p.promptConfirm(ctx, req)
	}

	// Handle multi-select prompt
	if len(req.Options) > 0 && req.MultiSelect {
		return p.promptMultiSelect(ctx, req)
	}

	// Handle select prompt (options list)
	if len(req.Options) > 0 {
		return p.promptSelect(ctx, req)
	}

	// Regular text prompt
	return p.promptText(ctx, req)
}

// promptText handles regular text input prompts.
func (p TerminalPrompter) promptText(ctx context.Context, req PromptRequest) (string, error) {
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

// promptSelect handles select-style prompts with navigable options.
func (p TerminalPrompter) promptSelect(ctx context.Context, req PromptRequest) (string, error) {
	reader := bufio.NewReader(p.In)

	// Find default option index
	defaultIdx := -1
	if req.Default != "" {
		for i, opt := range req.Options {
			if opt.Value == req.Default || opt.Label == req.Default {
				defaultIdx = i
				break
			}
		}
	}

	for {
		prefix := renderText(req.Theme.PrefixStyle, req.Theme.Prefix)
		label := renderText(req.Theme.LabelStyle, req.Label)
		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		if req.Theme.Hint != "" {
			hint := renderText(req.Theme.HintStyle, req.Theme.Hint)
			fmt.Fprintf(p.Out, " %s", hint)
		}
		fmt.Fprint(p.Out, "\n")

		// Display options
		selectedIdx := defaultIdx
		if selectedIdx < 0 {
			selectedIdx = 0
		}

		for i, opt := range req.Options {
			marker := " "
			if i == selectedIdx {
				marker = ">"
			}
			fmt.Fprintf(p.Out, "%s %s", marker, opt.Label)
			if opt.Description != "" {
				fmt.Fprintf(p.Out, " - %s", opt.Description)
			}
			fmt.Fprint(p.Out, "\n")
		}

		fmt.Fprint(p.Out, "> ")

		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		input := strings.TrimSpace(line)

		// Empty input uses default or first option
		if input == "" {
			if defaultIdx >= 0 {
				return req.Options[defaultIdx].Value, nil
			}
			if len(req.Options) > 0 {
				return req.Options[0].Value, nil
			}
		}

		// Try to match by number (1-based index)
		if idx := parseIndex(input, len(req.Options)); idx >= 0 {
			return req.Options[idx].Value, nil
		}

		// Try to match by value or label
		for _, opt := range req.Options {
			if strings.EqualFold(opt.Value, input) || strings.EqualFold(opt.Label, input) {
				return opt.Value, nil
			}
			// Partial match on label (for filtering)
			if strings.HasPrefix(strings.ToLower(opt.Label), strings.ToLower(input)) {
				return opt.Value, nil
			}
		}

		// No match - validate if validator provided
		if req.Validate != nil {
			if err := req.Validate(input); err != nil {
				errPrefix := renderText(req.Theme.ErrorStyle, req.Theme.Error)
				errMsg := err.Error()
				if errMsg != "" {
					errMsg = renderText(req.Theme.ErrorStyle, errMsg)
				}
				fmt.Fprintf(p.Out, "%s%s\n", errPrefix, errMsg)
				continue
			}
		}

		// If validation passes, return the input
		return input, nil
	}
}

// promptConfirm handles yes/no confirmation prompts.
func (p TerminalPrompter) promptConfirm(ctx context.Context, req PromptRequest) (string, error) {
	reader := bufio.NewReader(p.In)

	// Determine default (Y/n or y/N)
	defaultYes := true
	defaultText := "Y"
	if req.Default == "n" || req.Default == "N" || strings.ToLower(req.Default) == "no" {
		defaultYes = false
		defaultText = "N"
	}

	for {
		prefix := renderText(req.Theme.PrefixStyle, req.Theme.Prefix)
		label := renderText(req.Theme.LabelStyle, req.Label)
		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		// Show default in prompt
		if defaultYes {
			fmt.Fprintf(p.Out, " (%s/n)", defaultText)
		} else {
			fmt.Fprintf(p.Out, " (y/%s)", defaultText)
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

		// Normalize response
		value = strings.ToLower(value)
		if value == "y" || value == "yes" {
			return "y", nil
		}
		if value == "n" || value == "no" {
			return "n", nil
		}

		// Invalid input
		errPrefix := renderText(req.Theme.ErrorStyle, req.Theme.Error)
		errMsg := "please enter 'y' or 'n'"
		if req.Theme.ErrorStyle != nil {
			errMsg = renderText(req.Theme.ErrorStyle, errMsg)
		}
		fmt.Fprintf(p.Out, "%s%s\n", errPrefix, errMsg)
	}
}

// promptMultiSelect handles multi-select prompts where users can choose multiple options.
func (p TerminalPrompter) promptMultiSelect(ctx context.Context, req PromptRequest) (string, error) {
	reader := bufio.NewReader(p.In)

	// Parse default selections
	selected := make(map[int]bool)
	if req.Default != "" {
		// Try parsing as indices first (e.g., "1,2,3")
		indices := parseIndices(req.Default, len(req.Options))
		if len(indices) > 0 {
			for _, idx := range indices {
				selected[idx] = true
			}
		} else {
			// Try parsing as comma-separated values (e.g., "a,b,c")
			values := strings.Split(req.Default, ",")
			for _, val := range values {
				val = strings.TrimSpace(val)
				for i, opt := range req.Options {
					if opt.Value == val || opt.Label == val {
						selected[i] = true
						break
					}
				}
			}
		}
	}

	for {
		prefix := renderText(req.Theme.PrefixStyle, req.Theme.Prefix)
		label := renderText(req.Theme.LabelStyle, req.Label)
		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		if req.Theme.Hint != "" {
			hint := renderText(req.Theme.HintStyle, req.Theme.Hint)
			fmt.Fprintf(p.Out, " %s", hint)
		}
		fmt.Fprint(p.Out, "\n")

		// Display options with checkboxes
		for i, opt := range req.Options {
			marker := "[ ]"
			if selected[i] {
				marker = "[x]"
			}
			fmt.Fprintf(p.Out, "%s %d. %s", marker, i+1, opt.Label)
			if opt.Description != "" {
				fmt.Fprintf(p.Out, " - %s", opt.Description)
			}
			fmt.Fprint(p.Out, "\n")
		}

		fmt.Fprint(p.Out, "> ")

		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		input := strings.TrimSpace(line)

		// If "done" or "finish" typed, return selections
		if strings.EqualFold(input, "done") || strings.EqualFold(input, "finish") || strings.EqualFold(input, "q") {
			if len(selected) == 0 {
				fmt.Fprintf(p.Out, "%sPlease select at least one option\n", renderText(req.Theme.ErrorStyle, req.Theme.Error))
				continue
			}
			return p.formatSelectedValues(req.Options, selected), nil
		}

		// Empty input with selections - return selected values
		if input == "" {
			if len(selected) > 0 {
				return p.formatSelectedValues(req.Options, selected), nil
			}
			fmt.Fprintf(p.Out, "%sPlease select at least one option\n", renderText(req.Theme.ErrorStyle, req.Theme.Error))
			continue
		}

		// Parse input as indices (supports "1,2,3" or "1 2 3" or "1, 2, 3")
		indices := parseIndices(input, len(req.Options))

		// Toggle selections
		for _, idx := range indices {
			if idx >= 0 && idx < len(req.Options) {
				selected[idx] = !selected[idx]
			}
		}

		// If no valid indices, try to match by label/value
		if len(indices) == 0 {
			found := false
			for _, opt := range req.Options {
				if strings.EqualFold(opt.Value, input) || strings.EqualFold(opt.Label, input) {
					for i, o := range req.Options {
						if o.Value == opt.Value || o.Label == opt.Label {
							selected[i] = !selected[i]
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			if !found {
				fmt.Fprintf(p.Out, "%sInvalid selection. Enter option numbers (e.g., 1,2,3)\n", renderText(req.Theme.ErrorStyle, req.Theme.Error))
				continue
			}
		}

		// After toggling, continue loop to show updated state
	}
}

// formatSelectedValues formats selected options into a comma-separated string.
func (p TerminalPrompter) formatSelectedValues(options []SelectOption, selected map[int]bool) string {
	var values []string
	for i, opt := range options {
		if selected[i] {
			values = append(values, opt.Value)
		}
	}
	return strings.Join(values, ",")
}

// parseIndices parses a string containing indices (supports comma, space, or comma-space separated).
func parseIndices(input string, max int) []int {
	// Replace commas with spaces, then split by spaces
	input = strings.ReplaceAll(input, ",", " ")
	parts := strings.Fields(input)

	var indices []int
	for _, part := range parts {
		var idx int
		if _, err := fmt.Sscanf(part, "%d", &idx); err != nil {
			continue
		}
		// Convert from 1-based to 0-based, validate range
		idx--
		if idx >= 0 && idx < max {
			indices = append(indices, idx)
		}
	}
	return indices
}

// parseIndex attempts to parse input as a 1-based index.
func parseIndex(input string, max int) int {
	// Try to parse as integer
	var idx int
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil {
		return -1
	}
	// Convert from 1-based to 0-based, validate range
	idx--
	if idx >= 0 && idx < max {
		return idx
	}
	return -1
}
