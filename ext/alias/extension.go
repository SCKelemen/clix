// Package alias adds gh-style command aliases to a clix app: shortcuts
// that users define for longer command lines.
//
//	myapp alias set co 'pr checkout'
//	myapp co 123           # runs: myapp pr checkout 123
//	myapp alias set il 'issue list --label=$1'
//	myapp il bug --web     # runs: myapp issue list --label=bug --web
//	myapp alias list
//	myapp alias delete co
//
// Aliases are stored in the config file under "aliases.<name>" and
// expanded by clix.App.Run (see clix.App.ExpandAlias): $1, $2, ... are
// replaced by the arguments after the alias, and the arguments after the
// highest one used are appended, as gh does.
//
// Security model:
//
//   - No shell aliases. gh's "alias set --shell" and expansions starting
//     with "!" run a shell; this extension refuses both, and Run refuses a
//     "!" expansion written into the config file by hand. An alias only
//     ever becomes the arguments of one of the app's own commands.
//   - The expansion is split into words with shell-like quoting ('...',
//     "...", backslashes) but nothing is evaluated: $VAR, globs, ~,
//     backquotes and $(...) are literal text.
//   - Arguments replace $1, $2, ... as whole words, or as the part of the
//     word where the placeholder is (--label=$1). They are never split
//     again, so an argument cannot inject flags or words.
//   - Aliases never shadow commands: setting one named like a root command
//     or command alias (hidden ones and the extensions' included) is an
//     error, and a command added later wins over an existing alias.
//   - An expansion must start with a root command, so aliases are expanded
//     once and cannot name other aliases.
//   - Only the first argument is looked up, and only when the root takes no
//     positional arguments of its own.
package alias

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/SCKelemen/clix/v2"
)

// Extension adds the alias commands and turns on alias expansion
// (clix.App.ExpandAliases):
//
//   - cli alias set <name> <expansion> [--clobber]
//   - cli alias list
//   - cli alias delete <name> | --all
//
// Root help lists the aliases under ALIAS COMMANDS, and shell completion
// (ext/autocomplete) offers them with the commands. An app command named
// "alias" replaces the extension's; expansion stays on.
type Extension struct{}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}
	app.ExpandAliases = true
	if app.Root.ResolvePath([]string{"alias"}) == nil {
		app.Root.AddCommand(NewAliasCommand(app))
	}
	return nil
}

// NewAliasCommand builds the alias command group.
func NewAliasCommand(app *clix.App) *clix.Command {
	cmd := clix.NewGroup("alias", "Create command shortcuts",
		setCommand(app),
		listCommand(app),
		deleteCommand(app),
	)
	cmd.Usage = fmt.Sprintf("%s alias [command]", app.Name)
	cmd.Long = `Aliases are shortcuts for longer command lines, stored in the config file.
$1, $2, ... in an expansion are replaced by the arguments after the alias,
and the remaining arguments are appended. An alias never runs a shell and
never replaces a command.`
	cmd.IsExtensionCommand = true
	clix.AllowConfigErrors(cmd)
	return cmd
}

// requireLoadable refuses to work on a config file that cannot be loaded:
// saving would replace it.
func requireLoadable(app *clix.App) error {
	if err := app.ConfigLoadError(); err != nil {
		return fmt.Errorf("%w (fix or reset the config file first)", err)
	}
	return nil
}

func setCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("set")
	cmd.Short = "Create a shortcut for a command line"
	cmd.Usage = fmt.Sprintf("%s alias set <name> <expansion> [--clobber]", app.Name)
	cmd.Example = fmt.Sprintf(`%[1]s alias set co 'pr checkout'
%[1]s alias set il 'issue list --label=$1'`, app.Name)
	var name, expansion string
	var clobber bool
	positional(cmd, "name", "Name of the alias", &name)
	positional(cmd, "expansion", "Command line it stands for, quoted as one argument", &expansion)
	cmd.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{Name: "clobber", Usage: "Replace an existing alias of that name", CommandLineOnly: true},
		Value:       &clobber,
	})
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		if err := requireLoadable(app); err != nil {
			return err
		}
		if old, ok := app.Aliases()[name]; ok && !clobber {
			return fmt.Errorf("alias %q already exists (for %q); use --clobber to replace it", name, old)
		}
		if err := app.SetAlias(name, expansion); err != nil {
			return err
		}
		if err := app.SaveConfig(); err != nil {
			return err
		}
		_, err := fmt.Fprintf(app.Out, "Added alias %s: %s\n", name, expansion)
		return err
	}
	return cmd
}

func listCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("list")
	cmd.Short = "List the aliases"
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		if err := requireLoadable(app); err != nil {
			return err
		}
		aliases := app.Aliases()
		names := app.AliasNames()
		for _, n := range names {
			if c := app.Root.ResolvePath([]string{n}); c != nil {
				_, _ = fmt.Fprintf(errOut(app), "warning: alias %s never runs: the command %q has its name\n", n, c.Name)
			}
		}
		if f, ok := app.Flags().String("format"); ok {
			switch strings.ToLower(f) {
			case clix.FormatJSON, clix.FormatYAML:
				return clix.FormatData(app.Out, aliases, strings.ToLower(f))
			}
		}
		if len(names) == 0 {
			_, _ = fmt.Fprintln(errOut(app), "no aliases configured")
			return nil
		}
		w := tabwriter.NewWriter(app.Out, 0, 0, 2, ' ', 0)
		for _, n := range names {
			_, _ = fmt.Fprintf(w, "%s:\t%s\n", n, aliases[n])
		}
		return w.Flush()
	}
	return cmd
}

func deleteCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("delete")
	cmd.Short = "Delete an alias"
	cmd.Usage = fmt.Sprintf("%s alias delete <name> | --all", app.Name)
	var name string
	var all bool
	positional(cmd, "name", "Alias to delete", &name)
	cmd.Flags.Lookup("name").Required = false
	cmd.Flags.BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{Name: "all", Usage: "Delete every alias", CommandLineOnly: true},
		Value:       &all,
	})
	cmd.Flags.SetCompletion("name", completeAliases(app))
	clix.AllowConfigErrors(cmd)
	cmd.Run = func(ctx *clix.Context) error {
		if err := requireLoadable(app); err != nil {
			return err
		}
		var names []string
		switch {
		case all && name != "":
			return errors.New("give an alias name or --all, not both")
		case all:
			names = app.AliasNames()
		case name == "":
			return &clix.MissingFlagsError{Flags: []string{"name"}}
		default:
			if _, ok := app.Aliases()[name]; !ok {
				return fmt.Errorf("no such alias %q", name)
			}
			names = []string{name}
		}
		for _, n := range names {
			app.DeleteAlias(n)
		}
		if len(names) > 0 {
			if err := app.SaveConfig(); err != nil {
				return err
			}
		}
		for _, n := range names {
			_, _ = fmt.Fprintf(app.Out, "Deleted alias %s\n", n)
		}
		return nil
	}
	return cmd
}

// positional registers a required, positional, command-line-only string
// flag.
func positional(cmd *clix.Command, name, usage string, value *string) {
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       name,
			Usage:      usage,
			Required:   true,
			Positional: true,
			// Only the command line: <PREFIX>_NAME or a "name" config key
			// must not become the argument.
			CommandLineOnly: true,
		},
		Value: value,
	})
}

// completeAliases completes alias names.
func completeAliases(app *clix.App) clix.CompletionFunc {
	return func(clix.CompletionRequest) []clix.Completion {
		aliases := app.Aliases()
		var out []clix.Completion
		for _, n := range app.AliasNames() {
			out = append(out, clix.Completion{Value: n, Description: aliases[n]})
		}
		return out
	}
}

func errOut(app *clix.App) io.Writer {
	if app.Err != nil {
		return app.Err
	}
	return os.Stderr
}
