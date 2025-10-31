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
