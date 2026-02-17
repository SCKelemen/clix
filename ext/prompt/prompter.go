package prompt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/SCKelemen/clix/v2"

	"golang.org/x/term"
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

func buttonHoverStyle(theme clix.PromptTheme) clix.TextStyle {
	if theme.Buttons.Hover != nil {
		return theme.Buttons.Hover
	}
	return theme.ButtonHoverStyle
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

// TerminalPrompter implements Prompter with full support for text, select,
// multi-select, and confirm prompts, including raw terminal mode for interactive navigation.
type TerminalPrompter struct {
	In  io.Reader
	Out io.Writer
}

// Prompt displays a prompt and reads the user's response.
// Supports all prompt types: text, select, multi-select, and confirm.
func (p TerminalPrompter) Prompt(ctx context.Context, opts ...clix.PromptOption) (string, error) {
	if p.In == nil || p.Out == nil {
		return "", errors.New("prompter missing IO")
	}

	cfg := &clix.PromptConfig{Theme: clix.DefaultPromptTheme}
	for _, opt := range opts {
		opt.Apply(cfg)
	}

	return p.prompt(ctx, cfg)
}

func (p TerminalPrompter) prompt(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	if p.In == nil || p.Out == nil {
		return "", errors.New("prompter missing IO")
	}

	// Handle confirmation prompt
	if cfg.Confirm {
		return p.promptConfirm(ctx, cfg)
	}

	// Handle multi-select prompt
	if len(cfg.Options) > 0 && cfg.MultiSelect {
		return p.promptMultiSelect(ctx, cfg)
	}

	// Handle select prompt (options list)
	if len(cfg.Options) > 0 {
		return p.promptSelect(ctx, cfg)
	}

	// Regular text prompt
	return p.promptText(ctx, cfg)
}

// promptText handles regular text input prompts.
func (p TerminalPrompter) promptText(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	// Check if input is a terminal - if not, use line-based fallback
	inFile, isTerminal := p.In.(*os.File)
	if !isTerminal {
		return p.promptTextLineBased(ctx, cfg)
	}

	// Check if it's actually a TTY
	if !term.IsTerminal(int(inFile.Fd())) {
		return p.promptTextLineBased(ctx, cfg)
	}

	// Use interactive mode with raw terminal
	return p.promptTextInteractive(ctx, cfg, inFile)
}

// promptTextLineBased handles text input with line-based reading (fallback for non-terminals).
func (p TerminalPrompter) promptTextLineBased(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	reader := bufio.NewReader(p.In)

	for {
		prefix := renderText(cfg.Theme.PrefixStyle, cfg.Theme.Prefix)
		label := renderText(cfg.Theme.LabelStyle, cfg.Label)
		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		if placeholder := placeholderText(cfg); placeholder != "" {
			def := renderText(placeholderStyle(cfg.Theme), placeholder)
			fmt.Fprintf(p.Out, " [%s]", def)
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

// promptTextInteractive handles text input with raw terminal mode for advanced features.
func (p TerminalPrompter) promptTextInteractive(ctx context.Context, cfg *clix.PromptConfig, inFile *os.File) (string, error) {
	// Enable raw mode for individual keystroke handling
	state, err := EnableRawMode(inFile)
	if err != nil {
		// Fall back to line-based if raw mode fails
		return p.promptTextLineBased(ctx, cfg)
	}
	defer state.Restore()

	currentInput := ""

	for {
		// Clear both lines (input and hint)
		fmt.Fprint(p.Out, "\r\033[K") // Clear input line
		fmt.Fprint(p.Out, "\n")
		fmt.Fprint(p.Out, "\r\033[K") // Clear hint line
		MoveCursorUp(p.Out, 1)        // Move back to input line

		prefix := renderText(cfg.Theme.PrefixStyle, cfg.Theme.Prefix)
		label := renderText(cfg.Theme.LabelStyle, cfg.Label)

		// Calculate visual text lengths (runes, not bytes) for cursor positioning
		// This handles multi-byte characters correctly (e.g., emoji in prefix)
		prefixTextLen := utf8.RuneCountInString(cfg.Theme.Prefix)
		labelTextLen := utf8.RuneCountInString(cfg.Label)

		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		// Show input value or placeholder with inline default
		fmt.Fprint(p.Out, ": ")
		if currentInput != "" {
			fmt.Fprint(p.Out, currentInput)
		}

		suggestion := suggestionText(cfg, currentInput)
		if suggestion != "" {
			def := renderText(suggestionStyle(cfg.Theme), suggestion)
			fmt.Fprint(p.Out, def)
		}

		// Move to next line for key hints
		fmt.Fprint(p.Out, "\n")
		fmt.Fprint(p.Out, "\r\033[K")

		hintText := renderHintLine(cfg, clix.PromptKeyState{
			Input:      currentInput,
			Default:    cfg.Default,
			Suggestion: suggestion,
		})
		fmt.Fprint(p.Out, hintText)

		// Move cursor back up to input line and position at end
		MoveCursorUp(p.Out, 1)

		// Position cursor at end of input (after currentInput, not after suggestion)
		fmt.Fprint(p.Out, "\r")
		// Calculate position: prefix + label + ": " + input length (using visual rune counts)
		// Note: We position after the input text, not after the suggestion (suggestion is just visual)
		// Use utf8.RuneCountInString for multi-byte character support
		inputLen := utf8.RuneCountInString(currentInput)
		totalPos := prefixTextLen + labelTextLen + 2 + inputLen // +2 for ": "

		// Move cursor to position (we're already at start, so just move right)
		// Note: ANSI escape codes in styled text don't count as visual columns,
		// and \033[%dC moves by visual character positions (runes), so this should be accurate
		if totalPos > 0 {
			// Use ANSI to move right - this moves by visual character count
			fmt.Fprintf(p.Out, "\033[%dC", totalPos)
		}

		// Read a single keypress
		key, err := ReadKey(p.In)
		if err != nil {
			return "", err
		}

		ShowCursor(p.Out) // Show cursor for input

		switch key {
		case KeyEnter:
			state := clix.PromptKeyState{
				Command:    clix.PromptCommand{Type: clix.PromptCommandEnter},
				Input:      currentInput,
				Default:    cfg.Default,
				Suggestion: suggestion,
			}
			if action := dispatchCommand(cfg, state, func(v string) { currentInput = v }); action.Exit || action.Handled {
				if action.Exit {
					HideCursor(p.Out)
					fmt.Fprint(p.Out, "\n")
					fmt.Fprint(p.Out, "\r\033[K")
					ShowCursor(p.Out)
					return "", action.ExitErr
				}
				continue
			}
			// Finished input - clear hint line and return
			HideCursor(p.Out)
			fmt.Fprint(p.Out, "\n")
			fmt.Fprint(p.Out, "\r\033[K") // Clear hint line
			ShowCursor(p.Out)

			value := currentInput
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
					currentInput = ""
					continue
				}
			}

			return value, nil
		case KeyBackspace:
			if len(currentInput) > 0 {
				currentInput = currentInput[:len(currentInput)-1]
			}
		case KeyTab:
			state := clix.PromptKeyState{
				Command:    clix.PromptCommand{Type: clix.PromptCommandTab},
				Input:      currentInput,
				Default:    cfg.Default,
				Suggestion: suggestion,
			}
			action := dispatchCommand(cfg, state, func(v string) { currentInput = v })
			if action.Exit {
				HideCursor(p.Out)
				fmt.Fprint(p.Out, "\n")
				fmt.Fprint(p.Out, "\r\033[K")
				ShowCursor(p.Out)
				return "", action.ExitErr
			}
			if action.Handled {
				continue
			}
			// Tab completion to default
			if cfg.Default != "" {
				currentInput = cfg.Default
			}
		case KeyCtrlC:
			fmt.Fprint(p.Out, "\n")
			fmt.Fprint(p.Out, "\r\033[K")
			return "", errors.New("cancelled")
		case KeyEscape:
			state := clix.PromptKeyState{
				Command:    clix.PromptCommand{Type: clix.PromptCommandEscape},
				Input:      currentInput,
				Default:    cfg.Default,
				Suggestion: suggestion,
			}
			action := dispatchCommand(cfg, state, func(v string) { currentInput = v })
			if action.Exit {
				HideCursor(p.Out)
				fmt.Fprint(p.Out, "\n")
				fmt.Fprint(p.Out, "\r\033[K")
				ShowCursor(p.Out)
				return "", action.ExitErr
			}
			if action.Handled {
				continue
			}
			// Default: clear input
			currentInput = ""
		case KeyF1, KeyF2, KeyF3, KeyF4, KeyF5, KeyF6, KeyF7, KeyF8, KeyF9, KeyF10, KeyF11, KeyF12:
			state := clix.PromptKeyState{
				Command: clix.PromptCommand{
					Type:        clix.PromptCommandFunction,
					FunctionKey: functionKeyNumber(key),
				},
				Input:      currentInput,
				Default:    cfg.Default,
				Suggestion: suggestion,
			}
			action := dispatchCommand(cfg, state, func(v string) { currentInput = v })
			if action.Exit {
				HideCursor(p.Out)
				fmt.Fprint(p.Out, "\n")
				fmt.Fprint(p.Out, "\r\033[K")
				ShowCursor(p.Out)
				return "", action.ExitErr
			}
			if action.Handled {
				continue
			}
		default:
			// Regular printable character
			if key.IsPrintable() && key.Rune != 0 {
				currentInput += string(key.Rune)
			}
		}
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
func (p TerminalPrompter) promptSelect(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	// Check if input is a terminal - if not, use line-based fallback
	inFile, isTerminal := p.In.(*os.File)
	if !isTerminal {
		return p.promptSelectLineBased(ctx, cfg)
	}

	// Check if it's actually a TTY
	if !term.IsTerminal(int(inFile.Fd())) {
		return p.promptSelectLineBased(ctx, cfg)
	}

	// Enable raw mode for arrow key navigation
	state, err := EnableRawMode(inFile)
	if err != nil {
		// Fall back to line-based if raw mode fails
		return p.promptSelectLineBased(ctx, cfg)
	}
	defer state.Restore()

	// Find default option index
	selectedIdx := 0
	if cfg.Default != "" {
		for i, opt := range cfg.Options {
			if opt.Value == cfg.Default || opt.Label == cfg.Default {
				selectedIdx = i
				break
			}
		}
	}

	// Hide cursor during selection
	HideCursor(p.Out)
	defer ShowCursor(p.Out)

	// Calculate number of lines we'll render (label + options + hint line)
	hintLines := 0
	if cfg.Theme.Hint != "" {
		hintLines = 1
	}
	linesToRender := 1 + len(cfg.Options) + hintLines // label line + options + hint

	// Initial render
	p.renderSelectPrompt(cfg, selectedIdx)

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
				selectedIdx = len(cfg.Options) - 1 // Wrap to bottom
			}
			// Move cursor up to start of prompt, then redraw
			MoveCursorUp(p.Out, linesToRender)
			p.renderSelectPrompt(cfg, selectedIdx)
		case KeyDown:
			if selectedIdx < len(cfg.Options)-1 {
				selectedIdx++
			} else {
				selectedIdx = 0 // Wrap to top
			}
			// Move cursor up to start of prompt, then redraw
			MoveCursorUp(p.Out, linesToRender)
			p.renderSelectPrompt(cfg, selectedIdx)
		case KeyEnter:
			// Selection confirmed - clear all prompt lines and show selection
			MoveCursorUp(p.Out, linesToRender)
			// Clear all lines we rendered
			for i := 0; i < linesToRender; i++ {
				fmt.Fprint(p.Out, "\r\033[K")
				if i < linesToRender-1 {
					MoveCursorDown(p.Out, 1)
				}
			}
			// Move back to the top line and clear it, then show the selected value
			MoveCursorUp(p.Out, linesToRender-1)
			fmt.Fprint(p.Out, "\r\033[K")
			if len(cfg.Options) > 0 {
				// Show what was selected (optional - can be removed if not desired)
				prefix := renderText(cfg.Theme.PrefixStyle, cfg.Theme.Prefix)
				label := renderText(cfg.Theme.LabelStyle, cfg.Label)
				fmt.Fprintf(p.Out, "%s%s: %s\n", prefix, label, cfg.Options[selectedIdx].Label)
				// Ensure cursor is at column 0 for next prompt
				fmt.Fprint(p.Out, "\r")
			}
			ShowCursor(p.Out)
			if len(cfg.Options) > 0 {
				return cfg.Options[selectedIdx].Value, nil
			}
		case KeyCtrlC:
			// Cancelled - clear all prompt lines
			MoveCursorUp(p.Out, linesToRender)
			// Clear all lines we rendered
			for i := 0; i < linesToRender; i++ {
				fmt.Fprint(p.Out, "\r\033[K")
				if i < linesToRender-1 {
					MoveCursorDown(p.Out, 1)
				}
			}
			// Move back to the top line
			MoveCursorUp(p.Out, linesToRender-1)
			fmt.Fprint(p.Out, "\r\033[K")
			ShowCursor(p.Out)
			return "", errors.New("cancelled")
		case KeyEscape:
			state := clix.PromptKeyState{Command: clix.PromptCommand{Type: clix.PromptCommandEscape}, Default: cfg.Default}
			action := dispatchCommand(cfg, state, nil)
			if action.Exit {
				MoveCursorUp(p.Out, linesToRender)
				for i := 0; i < linesToRender; i++ {
					fmt.Fprint(p.Out, "\r\033[K")
					if i < linesToRender-1 {
						MoveCursorDown(p.Out, 1)
					}
				}
				MoveCursorUp(p.Out, linesToRender-1)
				fmt.Fprint(p.Out, "\r\033[K")
				ShowCursor(p.Out)
				return "", action.ExitErr
			}
			if action.Handled {
				MoveCursorUp(p.Out, linesToRender)
				p.renderSelectPrompt(cfg, selectedIdx)
				continue
			}
			// Default: treat as cancel
			MoveCursorUp(p.Out, linesToRender)
			for i := 0; i < linesToRender; i++ {
				fmt.Fprint(p.Out, "\r\033[K")
				if i < linesToRender-1 {
					MoveCursorDown(p.Out, 1)
				}
			}
			MoveCursorUp(p.Out, linesToRender-1)
			fmt.Fprint(p.Out, "\r\033[K")
			ShowCursor(p.Out)
			return "", errors.New("cancelled")
		case KeyF1, KeyF2, KeyF3, KeyF4, KeyF5, KeyF6, KeyF7, KeyF8, KeyF9, KeyF10, KeyF11, KeyF12:
			state := clix.PromptKeyState{
				Command: clix.PromptCommand{Type: clix.PromptCommandFunction, FunctionKey: functionKeyNumber(key)},
				Default: cfg.Default,
			}
			action := dispatchCommand(cfg, state, nil)
			if action.Exit {
				MoveCursorUp(p.Out, linesToRender)
				for i := 0; i < linesToRender; i++ {
					fmt.Fprint(p.Out, "\r\033[K")
					if i < linesToRender-1 {
						MoveCursorDown(p.Out, 1)
					}
				}
				MoveCursorUp(p.Out, linesToRender-1)
				fmt.Fprint(p.Out, "\r\033[K")
				ShowCursor(p.Out)
				return "", action.ExitErr
			}
			if action.Handled {
				MoveCursorUp(p.Out, linesToRender)
				p.renderSelectPrompt(cfg, selectedIdx)
				continue
			}
			// Default: treat as cancel
			MoveCursorUp(p.Out, linesToRender)
			for i := 0; i < linesToRender; i++ {
				fmt.Fprint(p.Out, "\r\033[K")
				if i < linesToRender-1 {
					MoveCursorDown(p.Out, 1)
				}
			}
			MoveCursorUp(p.Out, linesToRender-1)
			fmt.Fprint(p.Out, "\r\033[K")
			ShowCursor(p.Out)
			return "", errors.New("cancelled")
		case KeyHome:
			selectedIdx = 0
			MoveCursorUp(p.Out, linesToRender)
			p.renderSelectPrompt(cfg, selectedIdx)
		case KeyEnd:
			selectedIdx = len(cfg.Options) - 1
			MoveCursorUp(p.Out, linesToRender)
			p.renderSelectPrompt(cfg, selectedIdx)
		default:
			// Try to match by number (1-9) for quick selection
			if key.IsPrintable() && key.Rune >= '1' && key.Rune <= '9' {
				idx := int(key.Rune - '1')
				if idx < len(cfg.Options) {
					selectedIdx = idx
					// Clear all prompt lines and show selection
					MoveCursorUp(p.Out, linesToRender)
					// Clear all lines we rendered
					for i := 0; i < linesToRender; i++ {
						fmt.Fprint(p.Out, "\r\033[K")
						if i < linesToRender-1 {
							MoveCursorDown(p.Out, 1)
						}
					}
					// Clear all lines and show the selected value on a fresh line
					MoveCursorUp(p.Out, linesToRender)
					for i := 0; i < linesToRender; i++ {
						fmt.Fprint(p.Out, "\r\033[K")
						if i < linesToRender-1 {
							fmt.Fprint(p.Out, "\n")
						}
					}
					// Show what was selected on a clean line (starting from beginning of line)
					fmt.Fprint(p.Out, "\r\033[K") // Ensure we're at start of line
					prefix := renderText(cfg.Theme.PrefixStyle, cfg.Theme.Prefix)
					label := renderText(cfg.Theme.LabelStyle, cfg.Label)
					fmt.Fprintf(p.Out, "%s%s: %s\n", prefix, label, cfg.Options[selectedIdx].Label)
					// Ensure cursor is at column 0 for next prompt
					fmt.Fprint(p.Out, "\r")
					ShowCursor(p.Out)
					return cfg.Options[selectedIdx].Value, nil
				}
			}
			// For typing, we might want to switch to filtering mode
			// For now, just ignore non-navigation keys
		}
	}
}

// renderSelectPrompt renders the select prompt with the current selection.
func (p TerminalPrompter) renderSelectPrompt(cfg *clix.PromptConfig, selectedIdx int) {
	// Move to start of line and clear it
	fmt.Fprint(p.Out, "\r\033[K")
	prefix := renderText(cfg.Theme.PrefixStyle, cfg.Theme.Prefix)
	label := renderText(cfg.Theme.LabelStyle, cfg.Label)
	fmt.Fprintf(p.Out, "%s%s", prefix, label)

	if cfg.Theme.Hint != "" {
		hint := renderText(cfg.Theme.HintStyle, cfg.Theme.Hint)
		fmt.Fprintf(p.Out, " %s", hint)
	}
	// Clear rest of line and move to next
	fmt.Fprint(p.Out, "\033[K\n")

	// Display options
	for i, opt := range cfg.Options {
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

	// Show hint at bottom in low contrast
	if cfg.Theme.Hint != "" {
		fmt.Fprint(p.Out, "\r\033[K")
		hint := renderText(cfg.Theme.HintStyle, cfg.Theme.Hint)
		fmt.Fprint(p.Out, hint)
		fmt.Fprint(p.Out, "\n")
	}
}

// promptSelectLineBased is the fallback line-based implementation for non-terminal input.
func (p TerminalPrompter) promptSelectLineBased(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	reader := bufio.NewReader(p.In)

	// Find default option index
	defaultIdx := -1
	if cfg.Default != "" {
		for i, opt := range cfg.Options {
			if opt.Value == cfg.Default || opt.Label == cfg.Default {
				defaultIdx = i
				break
			}
		}
	}

	for {
		prefix := renderText(cfg.Theme.PrefixStyle, cfg.Theme.Prefix)
		label := renderText(cfg.Theme.LabelStyle, cfg.Label)
		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		if cfg.Theme.Hint != "" {
			hint := renderText(cfg.Theme.HintStyle, cfg.Theme.Hint)
			fmt.Fprintf(p.Out, " %s", hint)
		}
		fmt.Fprint(p.Out, "\n")

		// Display options
		selectedIdx := defaultIdx
		if selectedIdx < 0 {
			selectedIdx = 0
		}

		for i, opt := range cfg.Options {
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
				return cfg.Options[defaultIdx].Value, nil
			}
			if len(cfg.Options) > 0 {
				return cfg.Options[0].Value, nil
			}
		}

		// Try to match by number (1-based index)
		if idx := parseIndex(input, len(cfg.Options)); idx >= 0 {
			return cfg.Options[idx].Value, nil
		}

		// Try to match by value or label
		for _, opt := range cfg.Options {
			if strings.EqualFold(opt.Value, input) || strings.EqualFold(opt.Label, input) {
				return opt.Value, nil
			}
			// Partial match on label (for filtering)
			if strings.HasPrefix(strings.ToLower(opt.Label), strings.ToLower(input)) {
				return opt.Value, nil
			}
		}

		// No match - validate if validator provided
		if cfg.Validate != nil {
			if err := cfg.Validate(input); err != nil {
				errPrefix := renderText(cfg.Theme.ErrorStyle, cfg.Theme.Error)
				errMsg := err.Error()
				if errMsg != "" {
					errMsg = renderText(cfg.Theme.ErrorStyle, errMsg)
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
func (p TerminalPrompter) promptConfirm(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	reader := bufio.NewReader(p.In)

	// Determine default (Y/n or y/N)
	defaultYes := true
	defaultText := "Y"
	if cfg.Default == "n" || cfg.Default == "N" || strings.ToLower(cfg.Default) == "no" {
		defaultYes = false
		defaultText = "N"
	}

	for {
		// Ensure cursor is at column 0 (in case previous prompt left it elsewhere)
		fmt.Fprint(p.Out, "\r")
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

// promptMultiSelect handles multi-select prompts where users can choose multiple options.
func (p TerminalPrompter) promptMultiSelect(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	// Check if input is a terminal - if not, use line-based fallback
	inFile, isTerminal := p.In.(*os.File)
	if !isTerminal {
		return p.promptMultiSelectLineBased(ctx, cfg)
	}

	// Check if it's actually a TTY
	if !term.IsTerminal(int(inFile.Fd())) {
		return p.promptMultiSelectLineBased(ctx, cfg)
	}

	// Enable raw mode for arrow key navigation
	state, err := EnableRawMode(inFile)
	if err != nil {
		// Fall back to line-based if raw mode fails
		return p.promptMultiSelectLineBased(ctx, cfg)
	}
	defer state.Restore()

	// Parse default selections
	selected := make(map[int]bool)
	if cfg.Default != "" {
		// Try parsing as indices first (e.g., "1,2,3")
		indices := parseIndices(cfg.Default, len(cfg.Options))
		if len(indices) > 0 {
			for _, idx := range indices {
				selected[idx] = true
			}
		} else {
			// Try parsing as comma-separated values (e.g., "a,b,c")
			values := strings.Split(cfg.Default, ",")
			for _, val := range values {
				val = strings.TrimSpace(val)
				for i, opt := range cfg.Options {
					if opt.Value == val || opt.Label == val {
						selected[i] = true
						break
					}
				}
			}
		}
	}

	currentIdx := 0
	if len(cfg.Options) > 0 {
		// Find first selected option or default to 0
		for i := range cfg.Options {
			if selected[i] {
				currentIdx = i
				break
			}
		}
	}

	// Hide cursor during selection
	HideCursor(p.Out)
	defer ShowCursor(p.Out)

	// Calculate number of lines we'll render (label + options + continue line + hint line)
	hintLines := 0
	if cfg.Theme.Hint != "" {
		hintLines = 1
	}
	linesToRender := 2 + len(cfg.Options) + hintLines // label line + options + continue line + hint

	// Track if we're on the continue button (-1 means continue button, >= 0 means option index)
	onContinueButton := false

	// Initial render
	p.renderMultiSelectPrompt(cfg, selected, currentIdx, onContinueButton)

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
				currentIdx = len(cfg.Options) - 1
			} else if currentIdx > 0 {
				currentIdx--
			} else {
				// Wrap to continue button
				onContinueButton = true
			}
			// Move cursor up to start of prompt, then redraw
			MoveCursorUp(p.Out, linesToRender)
			p.renderMultiSelectPrompt(cfg, selected, currentIdx, onContinueButton)
		case KeyDown:
			if onContinueButton {
				// Move from continue button to first option
				onContinueButton = false
				currentIdx = 0
			} else if currentIdx < len(cfg.Options)-1 {
				currentIdx++
			} else {
				// Wrap to continue button
				onContinueButton = true
			}
			// Move cursor up to start of prompt, then redraw
			MoveCursorUp(p.Out, linesToRender)
			p.renderMultiSelectPrompt(cfg, selected, currentIdx, onContinueButton)
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
					return p.formatSelectedValues(cfg.Options, selected), nil
				}
				// No selections - stay on continue button (can't continue without selections)
			} else {
				// Toggle current selection (Enter or Space both toggle)
				if len(cfg.Options) > 0 {
					selected[currentIdx] = !selected[currentIdx]
					// Move cursor up to start of prompt, then redraw
					MoveCursorUp(p.Out, linesToRender)
					p.renderMultiSelectPrompt(cfg, selected, currentIdx, onContinueButton)
				}
			}
		case KeyCtrlC:
			// Cancelled - clear all prompt lines
			MoveCursorUp(p.Out, linesToRender)
			for i := 0; i < linesToRender; i++ {
				fmt.Fprint(p.Out, "\r\033[K")
				if i < linesToRender-1 {
					MoveCursorDown(p.Out, 1)
				}
			}
			MoveCursorUp(p.Out, linesToRender-1)
			fmt.Fprint(p.Out, "\r\033[K")
			ShowCursor(p.Out)
			return "", errors.New("cancelled")
		case KeyEscape:
			state := clix.PromptKeyState{Command: clix.PromptCommand{Type: clix.PromptCommandEscape}, Default: cfg.Default}
			action := dispatchCommand(cfg, state, nil)
			if action.Exit {
				MoveCursorUp(p.Out, linesToRender)
				for i := 0; i < linesToRender; i++ {
					fmt.Fprint(p.Out, "\r\033[K")
					if i < linesToRender-1 {
						MoveCursorDown(p.Out, 1)
					}
				}
				MoveCursorUp(p.Out, linesToRender-1)
				fmt.Fprint(p.Out, "\r\033[K")
				ShowCursor(p.Out)
				return "", action.ExitErr
			}
			if action.Handled {
				MoveCursorUp(p.Out, linesToRender)
				p.renderMultiSelectPrompt(cfg, selected, currentIdx, onContinueButton)
				continue
			}
			// Default: treat as cancel
			MoveCursorUp(p.Out, linesToRender)
			for i := 0; i < linesToRender; i++ {
				fmt.Fprint(p.Out, "\r\033[K")
				if i < linesToRender-1 {
					MoveCursorDown(p.Out, 1)
				}
			}
			MoveCursorUp(p.Out, linesToRender-1)
			fmt.Fprint(p.Out, "\r\033[K")
			ShowCursor(p.Out)
			return "", errors.New("cancelled")
		case KeyF1, KeyF2, KeyF3, KeyF4, KeyF5, KeyF6, KeyF7, KeyF8, KeyF9, KeyF10, KeyF11, KeyF12:
			state := clix.PromptKeyState{
				Command: clix.PromptCommand{Type: clix.PromptCommandFunction, FunctionKey: functionKeyNumber(key)},
				Default: cfg.Default,
			}
			action := dispatchCommand(cfg, state, nil)
			if action.Exit {
				MoveCursorUp(p.Out, linesToRender)
				for i := 0; i < linesToRender; i++ {
					fmt.Fprint(p.Out, "\r\033[K")
					if i < linesToRender-1 {
						MoveCursorDown(p.Out, 1)
					}
				}
				MoveCursorUp(p.Out, linesToRender-1)
				fmt.Fprint(p.Out, "\r\033[K")
				ShowCursor(p.Out)
				return "", action.ExitErr
			}
			if action.Handled {
				MoveCursorUp(p.Out, linesToRender)
				p.renderMultiSelectPrompt(cfg, selected, currentIdx, onContinueButton)
				continue
			}
			// Default: treat as cancel
			MoveCursorUp(p.Out, linesToRender)
			for i := 0; i < linesToRender; i++ {
				fmt.Fprint(p.Out, "\r\033[K")
				if i < linesToRender-1 {
					MoveCursorDown(p.Out, 1)
				}
			}
			MoveCursorUp(p.Out, linesToRender-1)
			fmt.Fprint(p.Out, "\r\033[K")
			ShowCursor(p.Out)
			return "", errors.New("cancelled")
		case KeyHome:
			onContinueButton = false
			currentIdx = 0
			MoveCursorUp(p.Out, linesToRender)
			p.renderMultiSelectPrompt(cfg, selected, currentIdx, onContinueButton)
		case KeyEnd:
			onContinueButton = true
			currentIdx = len(cfg.Options) - 1
			MoveCursorUp(p.Out, linesToRender)
			p.renderMultiSelectPrompt(cfg, selected, currentIdx, onContinueButton)
		default:
			// Try number keys for quick toggle (1-9)
			if key.IsPrintable() && key.Rune >= '1' && key.Rune <= '9' {
				idx := int(key.Rune - '1')
				if idx < len(cfg.Options) {
					onContinueButton = false
					currentIdx = idx
					selected[idx] = !selected[idx]
					// Move cursor up to start of prompt, then redraw
					MoveCursorUp(p.Out, linesToRender)
					p.renderMultiSelectPrompt(cfg, selected, currentIdx, onContinueButton)
				}
			}
		}
	}
}

// renderMultiSelectPrompt renders the multi-select prompt with current selection state.
func (p TerminalPrompter) renderMultiSelectPrompt(cfg *clix.PromptConfig, selected map[int]bool, currentIdx int, onContinueButton bool) {
	// Move to start of line and clear it
	fmt.Fprint(p.Out, "\r\033[K")
	prefix := renderText(cfg.Theme.PrefixStyle, cfg.Theme.Prefix)
	label := renderText(cfg.Theme.LabelStyle, cfg.Label)
	fmt.Fprintf(p.Out, "%s%s", prefix, label)
	// Clear rest of line and move to next (hint moved to bottom)
	fmt.Fprint(p.Out, "\033[K\n")

	// Display options with checkboxes
	for i, opt := range cfg.Options {
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
	continueText := cfg.ContinueText
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

	// Show hint at bottom in low contrast
	if cfg.Theme.Hint != "" {
		fmt.Fprint(p.Out, "\r\033[K")
		hint := renderText(cfg.Theme.HintStyle, cfg.Theme.Hint)
		fmt.Fprint(p.Out, hint)
		fmt.Fprint(p.Out, "\n")
	}
}

// formatSelectedValues formats selected options into a comma-delimited string of values.
func (p TerminalPrompter) formatSelectedValues(options []clix.SelectOption, selected map[int]bool) string {
	var values []string
	for i, opt := range options {
		if selected[i] {
			if opt.Value != "" {
				values = append(values, opt.Value)
				continue
			}
			values = append(values, opt.Label)
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
func (p TerminalPrompter) promptMultiSelectLineBased(ctx context.Context, cfg *clix.PromptConfig) (string, error) {
	reader := bufio.NewReader(p.In)

	// Parse default selections
	selected := make(map[int]bool)
	if cfg.Default != "" {
		// Try parsing as indices first (e.g., "1,2,3")
		indices := parseIndices(cfg.Default, len(cfg.Options))
		if len(indices) > 0 {
			for _, idx := range indices {
				selected[idx] = true
			}
		} else {
			// Try parsing as comma-separated values (e.g., "a,b,c")
			values := strings.Split(cfg.Default, ",")
			for _, val := range values {
				val = strings.TrimSpace(val)
				for i, opt := range cfg.Options {
					if opt.Value == val || opt.Label == val {
						selected[i] = true
						break
					}
				}
			}
		}
	}

	for {
		prefix := renderText(cfg.Theme.PrefixStyle, cfg.Theme.Prefix)
		label := renderText(cfg.Theme.LabelStyle, cfg.Label)
		fmt.Fprintf(p.Out, "%s%s", prefix, label)

		if cfg.Theme.Hint != "" {
			hint := renderText(cfg.Theme.HintStyle, cfg.Theme.Hint)
			fmt.Fprintf(p.Out, " %s", hint)
		}
		fmt.Fprint(p.Out, "\n")

		// Display options with checkboxes
		for i, opt := range cfg.Options {
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
				fmt.Fprintf(p.Out, "%sPlease select at least one option\n", renderText(cfg.Theme.ErrorStyle, cfg.Theme.Error))
				continue
			}
			return p.formatSelectedValues(cfg.Options, selected), nil
		}

		// Empty input with selections - return selected values
		if input == "" {
			if len(selected) > 0 {
				return p.formatSelectedValues(cfg.Options, selected), nil
			}
			fmt.Fprintf(p.Out, "%sPlease select at least one option\n", renderText(cfg.Theme.ErrorStyle, cfg.Theme.Error))
			continue
		}

		// Parse input as indices (supports "1,2,3" or "1 2 3" or "1, 2, 3")
		indices := parseIndices(input, len(cfg.Options))

		// Toggle selections
		for _, idx := range indices {
			if idx >= 0 && idx < len(cfg.Options) {
				selected[idx] = !selected[idx]
			}
		}

		// If no valid indices, try to match by label/value
		if len(indices) == 0 {
			found := false
			for _, opt := range cfg.Options {
				if strings.EqualFold(opt.Value, input) || strings.EqualFold(opt.Label, input) {
					for i, o := range cfg.Options {
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
				fmt.Fprintf(p.Out, "%sInvalid selection. Enter option numbers (e.g., 1,2,3)\n", renderText(cfg.Theme.ErrorStyle, cfg.Theme.Error))
				continue
			}
		}

		// After toggling, continue loop to show updated state
	}
}
