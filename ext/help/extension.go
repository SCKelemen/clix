package help

import (
	"fmt"
	"strings"

	"github.com/SCKelemen/clix/v2"
)

// Extension adds the help command to a clix app.
// This provides command-based help similar to man pages:
//
//   - cli help                       - Show help for the root command
//   - cli help --command [command]   - Show help for a specific command
//
// Note: Flag-based help (-h, --help) is handled by the core library
// and does not require this extension. This extension only adds the
// "help" command itself.
//
// Example:
//
//	import (
//		"github.com/SCKelemen/clix/v2"
//		"github.com/SCKelemen/clix/v2/ext/help"
//	)
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(help.Extension{})
//	// Now your app has: myapp help --command [command]
//
//	// Users can now access help via:
//	//   myapp help
//	//   myapp help --command subcommand
//	//   myapp help --command "subcommand nested"
type Extension struct {
	// Extension has no configuration options.
	// Simply add it to your app to enable the help command.
}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}

	// Only add if not already present
	if findChild(app.Root, "help") == nil {
		app.Root.AddCommand(NewHelpCommand(app))
	}

	return nil
}

func findChild(cmd *clix.Command, name string) *clix.Command {
	// Use ResolvePath for consistent behavior with core library
	if resolved := cmd.ResolvePath([]string{name}); resolved != nil {
		return resolved
	}
	return nil
}

// NewHelpCommand constructs the help command.
func NewHelpCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("help")
	cmd.Short = "Show help for commands"
	cmd.Usage = fmt.Sprintf("%s help [--command <name>]", app.Name)
	cmd.IsExtensionCommand = true

	var command string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "command",
			Usage: "Command to show help for (space-separated for nested commands)",
		},
		Value: &command,
	})

	cmd.Run = func(ctx *clix.Context) error {
		target := app.Root
		if command != "" {
			parts := strings.Fields(command)
			if resolved := app.Root.ResolvePath(parts); resolved != nil {
				target = resolved
			} else {
				return fmt.Errorf("unknown command: %s", command)
			}
		}
		helper := clix.HelpRenderer{App: app, Command: target}
		return helper.Render(app.Out)
	}
	return cmd
}
