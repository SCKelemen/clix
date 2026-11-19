package help

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
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
// The words after "help" name the command, by name or alias, so
// "cli help group command", "cli help \"group command\"" and
// "cli help --command \"group command\"" are the same. Hidden commands are
// left out of help listings, suggestions and completion, but "cli help
// hidden-command" still shows their help. A word that names no command is
// an error such as `unknown command "wigdets" for "cli"; did you mean
// "widgets"?` (exit code 1 with clix.ExitCode).
//
// Help is written to App.Out. Like flag-based help, it is plain text when
// NO_COLOR is set or App.Out is not a terminal (see clix.HelpRenderer).
//
// An app command named "help" replaces the extension's, and adding the
// extension twice adds the command once.
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
// word. It reports the first word that does not name a child, with the
// visible children whose names are close to it.
func resolveCommand(root *clix.Command, path []string) (*clix.Command, error) {
	cmd := root
	for _, word := range path {
		next := cmd.ResolvePath([]string{word})
		if next == nil {
			msg := fmt.Sprintf("unknown command %q for %q", word, cmd.Path())
			if s := suggestions(cmd, word); len(s) > 0 {
				msg += "; did you mean " + quoteList(s) + "?"
			}
			return nil, errors.New(msg)
		}
		cmd = next
	}
	return cmd, nil
}

// suggestions returns the names of cmd's visible children that word may be
// a typo of: within two edits (ignoring case), or a prefix of the name.
func suggestions(cmd *clix.Command, word string) []string {
	word = strings.ToLower(word)
	type match struct {
		name string
		dist int
	}
	var matches []match
	for _, child := range cmd.VisibleChildren() {
		best := -1
		for _, name := range append([]string{child.Name}, child.Aliases...) {
			n := strings.ToLower(name)
			d := editDistance(word, n)
			if strings.HasPrefix(n, word) && d > 2 {
				d = 2
			}
			if d <= 2 && (best < 0 || d < best) {
				best = d
			}
		}
		if best >= 0 {
			matches = append(matches, match{child.Name, best})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].dist < matches[j].dist })
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.name
	}
	return names
}

// editDistance is the Levenshtein distance between a and b, in runes.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// quoteList formats names as "a", "a" or "b", or "a", "b" or "c".
func quoteList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = strconv.Quote(n)
	}
	if len(quoted) == 1 {
		return quoted[0]
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " or " + quoted[len(quoted)-1]
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
