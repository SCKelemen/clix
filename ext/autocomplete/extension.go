package autocomplete

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SCKelemen/clix/v2"
)

// Extension adds shell completion to a clix app.
// It supports bash (3.2 and later), zsh, fish and PowerShell.
//
// The extension adds:
//
//	cli autocomplete [bash|zsh|fish|powershell] - Print the completion script for the shell
//	cli __complete ...                          - Hidden: print candidates (called by the scripts)
//
// The shell may also be given as --shell; pwsh is accepted for powershell.
//
// The scripts are thin wrappers that ask the program itself for the
// candidates (see NewCompleteCommand), so completion always matches the
// installed version and follows the command tree exactly: commands and
// aliases where they are valid (never hidden ones), the flags in scope at
// the cursor, and flag and argument values from completion hooks
// registered with clix.FlagSet.SetCompletion.
//
// Example:
//
//	import (
//		"github.com/SCKelemen/clix/v2"
//		"github.com/SCKelemen/clix/v2/ext/autocomplete"
//	)
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(autocomplete.Extension{})
//	// Now your app has: myapp autocomplete [shell]
//
//	// Users can load completion in the current shell:
//	//   source <(myapp autocomplete bash)          # bash
//	//   source <(myapp autocomplete zsh)           # zsh, after compinit
//	//   myapp autocomplete fish | source           # fish
//	// or install it:
//	//   myapp autocomplete bash > ~/.local/share/bash-completion/completions/myapp
//	//   myapp autocomplete zsh > "${fpath[1]}/_myapp"
//	//   myapp autocomplete fish > ~/.config/fish/completions/myapp.fish
//	//   myapp autocomplete powershell > myapp.ps1  # then dot-source it from $PROFILE
type Extension struct {
	// Extension has no configuration options.
	// Simply add it to your app to enable shell completion.
}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}

	// Only add if not already present
	if findChild(app.Root, "autocomplete") == nil {
		app.Root.AddCommand(NewAutocompleteCommand(app))
	}
	ensureCompleteCommand(app)
	return nil
}

func findChild(cmd *clix.Command, name string) *clix.Command {
	// Use ResolvePath for consistent behavior with core library
	if resolved := cmd.ResolvePath([]string{name}); resolved != nil {
		return resolved
	}
	return nil
}

// ensureCompleteCommand adds the hidden __complete command to the app's
// root unless the root already has one.
func ensureCompleteCommand(app *clix.App) {
	if app.Root != nil && findChild(app.Root, completeName) == nil {
		app.Root.AddCommand(NewCompleteCommand(app))
	}
}

// NewAutocompleteCommand provides shell completion scripts. The scripts
// call the hidden __complete command (see NewCompleteCommand), so
// NewAutocompleteCommand also adds that command to app.Root when it is not
// there yet.
func NewAutocompleteCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("autocomplete")
	cmd.Short = "Generate shell completion script"
	cmd.Usage = fmt.Sprintf("%s autocomplete [bash|zsh|fish|powershell]", app.Name)
	cmd.Long = installHelp(app.Name)
	cmd.IsExtensionCommand = true

	var shell string
	cmd.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:       "shell",
			Usage:      "Shell type (bash, zsh, fish, powershell)",
			Positional: true,
		},
		Value: &shell,
	})
	cmd.Flags.SetCompletion("shell", clix.CompleteValues(shells...))

	cmd.Run = func(ctx *clix.Context) error {
		if shell == "" {
			// Show help if no shell provided
			helper := clix.HelpRenderer{App: app, Command: cmd}
			return helper.Render(output(app))
		}
		text, err := script(app.Name, shell)
		if err != nil {
			return err
		}
		if app.Root != nil && findChild(app.Root, completeName) == nil {
			// The script would have nothing to call.
			_, _ = fmt.Fprintf(errOutput(app), "warning: %s has no %s command; add autocomplete.NewCompleteCommand(app) to the root command\n", app.Name, completeName)
		}
		// Only the script goes to standard output, so it can be redirected
		// to a file or sourced as it is.
		_, err = io.WriteString(output(app), text)
		return err
	}
	ensureCompleteCommand(app)
	return cmd
}

// installHelp returns the autocomplete command's long help: how to load
// and install each script.
func installHelp(name string) string {
	return strings.NewReplacer("@NAME@", name, "@ZFUNC@", "_"+identifier(name)).Replace(`Print a shell completion script on standard output.

Completion asks @NAME@ itself for candidates, so it always matches the
installed version: commands where they are valid, the flags in scope, and
values for flags and arguments.

bash (3.2 or later):
  source <(@NAME@ autocomplete bash)
  # or install it for bash-completion:
  @NAME@ autocomplete bash > ~/.local/share/bash-completion/completions/@NAME@

zsh:
  source <(@NAME@ autocomplete zsh)    # after compinit
  # or install it as @ZFUNC@ in a directory on $fpath, before compinit runs:
  @NAME@ autocomplete zsh > "${fpath[1]}/@ZFUNC@"

fish:
  @NAME@ autocomplete fish | source
  # or install it:
  @NAME@ autocomplete fish > ~/.config/fish/completions/@NAME@.fish

PowerShell:
  @NAME@ autocomplete powershell > @NAME@.ps1
  # then dot-source it from your profile ($PROFILE):
  . /path/to/@NAME@.ps1`)
}

// errOutput returns the app's error output.
func errOutput(app *clix.App) io.Writer {
	if app.Err != nil {
		return app.Err
	}
	return os.Stderr
}
