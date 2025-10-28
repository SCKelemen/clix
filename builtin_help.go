package clix

import "fmt"

// NewHelpCommand constructs the built-in help command.
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
