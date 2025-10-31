package clix

import "fmt"

// NewHelpCommand constructs the built-in help command.
//
// Deprecated: Use clix/ext/help.Extension instead for optional help command.
// Flag-based help (-h, --help) remains in core and works without this.
// This function is kept for backward compatibility but will be removed in a future version.
func NewHelpCommand(app *App) *Command {
	cmd := NewCommand("help")
	cmd.Short = "Show help for commands"
	cmd.Usage = fmt.Sprintf("%s help [command]", app.Name)
	cmd.Run = func(ctx *Context) error {
		target := app.Root
		if len(ctx.Args) > 0 {
			if resolved := app.Root.ResolvePath(ctx.Args); resolved != nil {
				target = resolved
			} else {
				return fmt.Errorf("unknown command: %s", ctx.Args)
			}
		}
		helper := HelpRenderer{App: app, Command: target}
		return helper.Render(app.Out)
	}
	return cmd
}
