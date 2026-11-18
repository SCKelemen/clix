package prompt

import "github.com/SCKelemen/clix/v2"

// Extension replaces the default TextPrompter with TerminalPrompter,
// enabling advanced prompt features: select, multi-select, confirm, and raw terminal mode.
//
// Only the core default is replaced: when App.Prompter is nil or a
// clix.TextPrompter, it becomes a TerminalPrompter reading from App.In and
// writing to App.Err (App.Out if Err is nil), so prompts never mix with
// command output. A Prompter you set yourself is left in place.
//
// Without this extension, advanced prompt types (select, multi-select) return errors
// directing users to add the extension. With this extension, all prompt types are supported.
//
// Example:
//
//	import (
//		"github.com/SCKelemen/clix/v2"
//		"github.com/SCKelemen/clix/v2/ext/prompt"
//	)
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(prompt.Extension{})
//	// Now your app supports select, multi-select, and confirm prompts
//
//	// Use advanced prompts via the standard PromptRequest API:
//	result, err := app.Prompter.Prompt(ctx, clix.PromptRequest{
//		Label: "Choose an option",
//		Options: []clix.SelectOption{
//			{Label: "Option A", Value: "a"},
//			{Label: "Option B", Value: "b"},
//		},
//	})
//
//	// Or use the helper function for type-safe access:
//	if tp := prompt.AsTerminalPrompter(app.Prompter); tp != nil {
//		// Access TerminalPrompter-specific methods if needed
//	}
type Extension struct {
	// Extension has no configuration options.
	// Simply add it to your app to enable advanced prompt features.
}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	switch app.Prompter.(type) {
	case nil, clix.TextPrompter, *clix.TextPrompter:
		// The core default (or nothing): upgrade it.
	default:
		// A Prompter chosen by the application; keep it.
		return nil
	}

	out := app.Err
	if out == nil {
		out = app.Out
	}
	if app.In != nil && out != nil {
		app.Prompter = TerminalPrompter{
			In:  app.In,
			Out: out,
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
