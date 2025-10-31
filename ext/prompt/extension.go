package prompt

import "clix"

// Extension replaces the default TextPrompter with TerminalPrompter,
// enabling advanced prompt features: select, multi-select, confirm, and raw terminal mode.
//
// Usage:
//
//	import (
//		"clix"
//		"clix/ext/prompt"
//	)
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(prompt.Extension{})
//	// Now your app supports select, multi-select, and confirm prompts
//
//	// For advanced prompts, use type assertion to access TerminalPrompter methods:
//	if tp, ok := app.Prompter.(TerminalPrompter); ok {
//		// Use PromptWithOptions, PromptMultiSelect, or PromptConfirm
//		result, _ := tp.PromptWithOptions(ctx, clix.PromptRequest{
//			TextPromptRequest: clix.TextPromptRequest{Label: "Choose"},
//			Options: []clix.SelectOption{{Label: "A", Value: "a"}},
//		})
//	}
type Extension struct{}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	// Replace TextPrompter with TerminalPrompter
	if app.In != nil && app.Out != nil {
		app.Prompter = TerminalPrompter{
			In:  app.In,
			Out: app.Out,
		}
	}
	return nil
}

// AsTerminalPrompter safely casts a Prompter to TerminalPrompter.
// Returns nil if the prompter is not a TerminalPrompter (e.g., it's a TextPrompter).
// This allows you to access advanced prompt methods with compile-time safety.
//
// Example:
//
//	if tp := prompt.AsTerminalPrompter(app.Prompter); tp != nil {
//		result, err := tp.PromptWithOptions(ctx, clix.PromptRequest{
//			TextPromptRequest: clix.TextPromptRequest{Label: "Choose"},
//			Options: []clix.SelectOption{{Label: "A", Value: "a"}},
//		})
//	}
func AsTerminalPrompter(p clix.Prompter) *TerminalPrompter {
	tp, ok := p.(TerminalPrompter)
	if !ok {
		return nil
	}
	return &tp
}
