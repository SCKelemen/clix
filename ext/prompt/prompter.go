package prompt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"clix"

	"golang.org/x/term"
)

// EnhancedTerminalPrompter implements Prompter with full support for text, select,
// multi-select, and confirm prompts, including raw terminal mode for interactive navigation.
type EnhancedTerminalPrompter struct {
	In  io.Reader
	Out io.Writer
}

// Prompt displays a prompt and reads the user's response.
// Supports text, select, multi-select, and confirm prompts with automatic fallback.
func (p EnhancedTerminalPrompter) Prompt(ctx context.Context, req clix.PromptRequest) (string, error) {
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
func (p EnhancedTerminalPrompter) promptText(ctx context.Context, req clix.PromptRequest) (string, error) {
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

// renderText renders text with optional styling.
func renderText(style clix.TextStyle, value string) string {
	if style == nil {
		return value
	}
	return style.Render(value)
}

// promptSelect handles select-style prompts with navigable options.
func (p EnhancedTerminalPrompter) promptSelect(ctx context.Context, req clix.PromptRequest) (string, error) {
	// Check if input is a terminal - if not, use line-based fallback
	inFile, isTerminal := p.In.(*os.File)
	if !isTerminal {
		return p.promptSelectLineBased(ctx, req)
	}

	// Check if it's actually a TTY
	if !term.IsTerminal(int(inFile.Fd())) {
		return p.promptSelectLineBased(ctx, req)
	}

	// Enable raw mode for arrow key navigation
	state, err := EnableRawMode(inFile)
	if err != nil {
		// Fall back to line-based if raw mode fails
		return p.promptSelectLineBased(ctx, req)
	}
	defer state.Restore()

	// Find default option index
	selectedIdx := 0
	if req.Default != "" {
		for i, opt := range req.Options {
			if opt.Value == req.Default || opt.Label == req.Default {
				selectedIdx = i
				break
			}
		}
	}

	// Hide cursor during selection
	HideCursor(p.Out)
	defer ShowCursor(p.Out)

	// Calculate number of lines we'll render
	linesToRender := 1 + len(req.Options) // label line + options

	// Initial render
	p.renderSelectPrompt(req, selectedIdx)

	for {
		// Read a single keypress
		key, err := ReadKey(p.In)
		if err != nil {
			return "", err
		}

		// Handle navigation
		switch key {
		case KeyUp:
			if selectedIdx > 0 {
				selectedIdx--
			} else {
				selectedIdx = len(req.Options) - 1 // Wrap to bottom
			}
			// Move cursor up to start of prompt, then redraw
			MoveCursorUp(p.Out, linesToRender)
			p.renderSelectPrompt(req, selectedIdx)
		case KeyDown:
			if selectedIdx < len(req.Options)-1 {
				selectedIdx++
			} else {
				selectedIdx = 0 // Wrap to top
			}
			// Move cursor up to start of prompt, then redraw
			MoveCursorUp(p.Out, linesToRender)
			p.renderSelectPrompt(req, selectedIdx)
		case KeyEnter:
			// Selection confirmed - show cursor and return
			ShowCursor(p.Out)
			fmt.Fprint(p.Out, "\n")
			if len(req.Options) > 0 {
				return req.Options[selectedIdx].Value, nil
			}
		case KeyCtrlC, KeyEscape:
			// Cancelled - show cursor and return
			ShowCursor(p.Out)
			fmt.Fprint(p.Out, "\n")
			return "", errors.New("cancelled")
		case KeyHome:
			selectedIdx = 0
			MoveCursorUp(p.Out, linesToRender)
			p.renderSelectPrompt(req, selectedIdx)
		case KeyEnd:
			selectedIdx = len(req.Options) - 1
			MoveCursorUp(p.Out, linesToRender)
			p.renderSelectPrompt(req, selectedIdx)
		default:
			// Try to match by number (1-9) for quick selection
			if key.IsPrintable() && key.Rune >= '1' && key.Rune <= '9' {
				idx := int(key.Rune - '1')
				if idx < len(req.Options) {
					selectedIdx = idx
					ShowCursor(p.Out)
					fmt.Fprint(p.Out, "\n")
					return req.Options[selectedIdx].Value, nil
				}
			}
			// For typing, we might want to switch to filtering mode
			// For now, just ignore non-navigation keys
		}
	}
}

// renderSelectPrompt renders the select prompt with the current selection.
func (p EnhancedTerminalPrompter) renderSelectPrompt(req clix.PromptRequest, selectedIdx int) {
	// Move to start of line and clear it
	fmt.Fprint(p.Out, "\r\033[K")
	prefix := renderText(req.Theme.PrefixStyle, req.Theme.Prefix)
	label := renderText(req.Theme.LabelStyle, req.Label)
	fmt.Fprintf(p.Out, "%s%s", prefix, label)

	if req.Theme.Hint != "" {
		hint := renderText(req.Theme.HintStyle, req.Theme.Hint)
		fmt.Fprintf(p.Out, " %s", hint)
	}
	// Clear rest of line and move to next
	fmt.Fprint(p.Out, "\033[K\n")

	// Display options
	for i, opt := range req.Options {
		// Move to start of line and clear it
		fmt.Fprint(p.Out, "\r\033[K")
		marker := " "
		if i == selectedIdx {
			marker = ">"
		}
		fmt.Fprintf(p.Out, "%s %s", marker, opt.Label)
		if opt.Description != "" {
			fmt.Fprintf(p.Out, " - %s", opt.Description)
		}
		// Clear rest of line and move to next
		fmt.Fprint(p.Out, "\033[K\n")
	}
}

// promptSelectLineBased is the fallback line-based implementation for non-terminal input.
func (p EnhancedTerminalPrompter) promptSelectLineBased(ctx context.Context, req clix.PromptRequest) (string, error) {
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

// promptConfirm handles yes/no confirmation prompts.
func (p EnhancedTerminalPrompter) promptConfirm(ctx context.Context, req clix.PromptRequest) (string, error) {
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
func (p EnhancedTerminalPrompter) promptMultiSelect(ctx context.Context, req clix.PromptRequest) (string, error) {
	// Check if input is a terminal - if not, use line-based fallback
	inFile, isTerminal := p.In.(*os.File)
	if !isTerminal {
		return p.promptMultiSelectLineBased(ctx, req)
	}

	// Check if it's actually a TTY
	if !term.IsTerminal(int(inFile.Fd())) {
		return p.promptMultiSelectLineBased(ctx, req)
	}

	// Enable raw mode for arrow key navigation
	state, err := EnableRawMode(inFile)
	if err != nil {
		// Fall back to line-based if raw mode fails
		return p.promptMultiSelectLineBased(ctx, req)
	}
	defer state.Restore()

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

	currentIdx := 0
	if len(req.Options) > 0 {
		// Find first selected option or default to 0
		for i := range req.Options {
			if selected[i] {
				currentIdx = i
				break
			}
		}
	}

	// Hide cursor during selection
	HideCursor(p.Out)
	defer ShowCursor(p.Out)

	// Calculate number of lines we'll render (label + options + continue line)
	linesToRender := 2 + len(req.Options) // label line + options + continue line

	// Track if we're on the continue button (-1 means continue button, >= 0 means option index)
	onContinueButton := false

	// Initial render
	p.renderMultiSelectPrompt(req, selected, currentIdx, onContinueButton)

	for {
		// Read a single keypress
		key, err := ReadKey(p.In)
		if err != nil {
			return "", err
		}

		// Handle navigation and selection
		switch key {
		case KeyUp:
			if onContinueButton {
				// Move from continue button to last option
				onContinueButton = false
				currentIdx = len(req.Options) - 1
			} else if currentIdx > 0 {
				currentIdx--
			} else {
				// Wrap to continue button
				onContinueButton = true
			}
			// Move cursor up to start of prompt, then redraw
			MoveCursorUp(p.Out, linesToRender)
			p.renderMultiSelectPrompt(req, selected, currentIdx, onContinueButton)
		case KeyDown:
			if onContinueButton {
				// Move from continue button to first option
				onContinueButton = false
				currentIdx = 0
			} else if currentIdx < len(req.Options)-1 {
				currentIdx++
			} else {
				// Wrap to continue button
				onContinueButton = true
			}
			// Move cursor up to start of prompt, then redraw
			MoveCursorUp(p.Out, linesToRender)
			p.renderMultiSelectPrompt(req, selected, currentIdx, onContinueButton)
		case KeySpace, KeyEnter:
			if onContinueButton {
				// On continue button - confirm if we have selections
				hasSelection := false
				for _, sel := range selected {
					if sel {
						hasSelection = true
						break
					}
				}
				if hasSelection {
					ShowCursor(p.Out)
					fmt.Fprint(p.Out, "\n")
					return p.formatSelectedValues(req.Options, selected), nil
				}
				// No selections - stay on continue button (can't continue without selections)
			} else {
				// Toggle current selection (Enter or Space both toggle)
				if len(req.Options) > 0 {
					selected[currentIdx] = !selected[currentIdx]
					// Move cursor up to start of prompt, then redraw
					MoveCursorUp(p.Out, linesToRender)
					p.renderMultiSelectPrompt(req, selected, currentIdx, onContinueButton)
				}
			}
		case KeyCtrlC, KeyEscape:
			// Cancelled - show cursor and return
			ShowCursor(p.Out)
			fmt.Fprint(p.Out, "\n")
			return "", errors.New("cancelled")
		case KeyHome:
			onContinueButton = false
			currentIdx = 0
			MoveCursorUp(p.Out, linesToRender)
			p.renderMultiSelectPrompt(req, selected, currentIdx, onContinueButton)
		case KeyEnd:
			onContinueButton = true
			currentIdx = len(req.Options) - 1
			MoveCursorUp(p.Out, linesToRender)
			p.renderMultiSelectPrompt(req, selected, currentIdx, onContinueButton)
		default:
			// Try number keys for quick toggle (1-9)
			if key.IsPrintable() && key.Rune >= '1' && key.Rune <= '9' {
				idx := int(key.Rune - '1')
				if idx < len(req.Options) {
					onContinueButton = false
					currentIdx = idx
					selected[idx] = !selected[idx]
					// Move cursor up to start of prompt, then redraw
					MoveCursorUp(p.Out, linesToRender)
					p.renderMultiSelectPrompt(req, selected, currentIdx, onContinueButton)
				}
			}
		}
	}
}

// renderMultiSelectPrompt renders the multi-select prompt with current selection state.
func (p EnhancedTerminalPrompter) renderMultiSelectPrompt(req clix.PromptRequest, selected map[int]bool, currentIdx int, onContinueButton bool) {
	// Move to start of line and clear it
	fmt.Fprint(p.Out, "\r\033[K")
	prefix := renderText(req.Theme.PrefixStyle, req.Theme.Prefix)
	label := renderText(req.Theme.LabelStyle, req.Label)
	fmt.Fprintf(p.Out, "%s%s", prefix, label)

	if req.Theme.Hint != "" {
		hint := renderText(req.Theme.HintStyle, req.Theme.Hint)
		fmt.Fprintf(p.Out, " %s", hint)
	}
	// Clear rest of line and move to next
	fmt.Fprint(p.Out, "\033[K\n")

	// Display options with checkboxes
	for i, opt := range req.Options {
		// Move to start of line and clear it
		fmt.Fprint(p.Out, "\r\033[K")
		marker := "[ ]"
		if selected[i] {
			marker = "[x]"
		}
		// Highlight current option
		indicator := " "
		if !onContinueButton && i == currentIdx {
			indicator = ">"
		}
		fmt.Fprintf(p.Out, "%s %s %d. %s", indicator, marker, i+1, opt.Label)
		if opt.Description != "" {
			fmt.Fprintf(p.Out, " - %s", opt.Description)
		}
		// Clear rest of line and move to next
		fmt.Fprint(p.Out, "\033[K\n")
	}

	// Display continue button
	continueText := req.ContinueText
	if continueText == "" {
		continueText = "Continue"
	}
	// Move to start of line and clear it
	fmt.Fprint(p.Out, "\r\033[K")
	indicator := " "
	if onContinueButton {
		indicator = ">"
	}
	fmt.Fprintf(p.Out, "%s %s", indicator, continueText)
	// Clear rest of line
	fmt.Fprint(p.Out, "\033[K\n")
}

// formatSelectedValues formats selected options into a comma-separated string.
func (p EnhancedTerminalPrompter) formatSelectedValues(options []clix.SelectOption, selected map[int]bool) string {
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

// promptMultiSelectLineBased is the fallback line-based implementation for non-terminal input.
func (p EnhancedTerminalPrompter) promptMultiSelectLineBased(ctx context.Context, req clix.PromptRequest) (string, error) {
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
