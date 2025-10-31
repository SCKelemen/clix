package prompt

import "clix"

// Extension replaces the default SimpleTextPrompter with EnhancedTerminalPrompter,
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
	// Replace SimpleTextPrompter with EnhancedTerminalPrompter
	if app.In != nil && app.Out != nil {
		app.Prompter = EnhancedTerminalPrompter{
			In:  app.In,
			Out: app.Out,
		}
	}
	return nil
}
