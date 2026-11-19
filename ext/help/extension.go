package help

import (
	"fmt"
	"strings"

	"github.com/SCKelemen/clix/v2"
)

// Extension adds the help command to a clix app.
// This provides command-based help similar to man pages:
//
//   - cli help              - Show help for the root command
//   - cli help [command...] - Show help for a specific command, such as
//     "cli help group command"
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
//	// Now your app has: myapp help [command...]
//
//	// Users can now access help via:
//	//   myapp help
//	//   myapp help subcommand
//	//   myapp help subcommand nested     (or "subcommand nested", or --command "subcommand nested")
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
//
// The command takes the path of the command to describe as its remaining
// arguments, so "help widgets get", "help \"widgets get\"" and
// "help --command \"widgets get\"" all show the help for "widgets get".
// Shell completion offers the command names at each level of the path.
func NewHelpCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("help")
	cmd.Short = "Show help for commands"
	cmd.Usage = fmt.Sprintf("%s help [command...]", app.Name)
	cmd.IsExtensionCommand = true

	var command []string
	cmd.Flags.ArgsVar(clix.ArgsVarOptions{
		Name:  "command",
		Usage: "Command to show help for, such as: group command",
		Value: &command,
	})
	cmd.Flags.SetCompletion("command", completeCommandPath(app))

	cmd.Run = func(ctx *clix.Context) error {
		target, err := resolveCommand(app.Root, strings.Fields(strings.Join(command, " ")))
		if err != nil {
			return err
		}
		helper := clix.HelpRenderer{App: app, Command: target}
		return helper.Render(app.Out)
	}
	return cmd
}

// resolveCommand walks path from root, one command name (or alias) per
// word. It reports the first word that does not name a child.
func resolveCommand(root *clix.Command, path []string) (*clix.Command, error) {
	cmd := root
	for _, word := range path {
		next := cmd.ResolvePath([]string{word})
		if next == nil {
			return nil, fmt.Errorf("unknown command %q for %q", word, cmd.Path())
		}
		cmd = next
	}
	return cmd, nil
}

// completeCommandPath completes the help command's arguments: the names of
// the visible children of the command named by the words already typed.
func completeCommandPath(app *clix.App) clix.CompletionFunc {
	return func(req clix.CompletionRequest) []clix.Completion {
		root := app.Root
		if req.App != nil && req.App.Root != nil {
			root = req.App.Root
		}
		if root == nil {
			return nil
		}
		path := strings.Fields(strings.Join(req.Args, " "))

		// A quoted path ("help 'widgets g<TAB>") completes its last word.
		var prefix string
		if i := strings.LastIndexAny(req.ToComplete, " \t"); i >= 0 {
			prefix = req.ToComplete[:i+1]
			path = append(path, strings.Fields(prefix)...)
		}

		parent, err := resolveCommand(root, path)
		if err != nil {
			return nil
		}
		var out []clix.Completion
		for _, child := range parent.VisibleChildren() {
			desc := child.Short
			if desc == "" {
				desc, _, _ = strings.Cut(child.Long, "\n")
			}
			out = append(out, clix.Completion{Value: prefix + child.Name, Description: desc})
		}
		return out
	}
}
