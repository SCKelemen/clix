package autocomplete

import (
	"fmt"
	"sort"
	"strings"

	"clix"
)

// Extension adds the autocomplete command to a clix app.
// This provides shell completion script generation:
//
//   - cli autocomplete [bash|zsh|fish] - Generate completion script for the specified shell
//
// Usage:
//
//	import (
//		"clix"
//		"clix/ext/autocomplete"
//	)
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(autocomplete.Extension{})
//	// Now your app has: myapp autocomplete [shell]
type Extension struct{}

// Extend implements clix.Extension.
func (Extension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}

	// Only add if not already present
	if findChild(app.Root, "autocomplete") == nil {
		app.Root.AddCommand(NewAutocompleteCommand(app))
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

// NewAutocompleteCommand provides shell completion scripts.
func NewAutocompleteCommand(app *clix.App) *clix.Command {
	cmd := clix.NewCommand("autocomplete")
	cmd.Short = "Generate shell completion script"
	cmd.Usage = fmt.Sprintf("%s autocomplete [bash|zsh|fish]", app.Name)
	cmd.Arguments = []*clix.Argument{{Name: "shell", Prompt: "Shell", Required: false}}
	cmd.Run = func(ctx *clix.Context) error {
		shellArg := ctx.Arg(0)
		if shellArg == "" {
			// Show help if no shell provided
			helper := clix.HelpRenderer{App: app, Command: cmd}
			return helper.Render(app.Out)
		}
		shell := strings.ToLower(shellArg)
		script, err := generateCompletionScript(app, shell)
		if err != nil {
			return err
		}
		fmt.Fprintln(app.Out, script)
		return nil
	}
	return cmd
}

func generateCompletionScript(app *clix.App, shell string) (string, error) {
	commands := collectCompletionEntries(app.Root)
	switch shell {
	case "bash":
		return bashCompletion(app.Name, commands), nil
	case "zsh":
		return zshCompletion(app.Name, commands), nil
	case "fish":
		return fishCompletion(app.Name, commands), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
}

type completionEntry struct {
	Token string
	Help  string
}

func collectCompletionEntries(cmd *clix.Command) []completionEntry {
	entries := make(map[string]string)
	var walk func(*clix.Command)
	walk = func(c *clix.Command) {
		entries[c.Name] = c.Short
		for _, alias := range c.Aliases {
			entries[alias] = c.Short
		}
		for _, flag := range c.Flags.Flags() {
			entries["--"+flag.Name] = flag.Usage
			if flag.Short != "" {
				entries["-"+flag.Short] = flag.Usage
			}
		}
		for _, child := range c.Children {
			walk(child)
		}
	}
	walk(cmd)

	keys := make([]string, 0, len(entries))
	for token := range entries {
		keys = append(keys, token)
	}
	sort.Strings(keys)

	result := make([]completionEntry, 0, len(keys))
	for _, token := range keys {
		result = append(result, completionEntry{Token: token, Help: entries[token]})
	}

	return result
}

func bashCompletion(name string, entries []completionEntry) string {
	var tokens []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Token, "-") {
			tokens = append(tokens, entry.Token)
		}
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Token, "-") {
			tokens = append(tokens, entry.Token)
		}
	}
	options := strings.Join(tokens, " ")
	return fmt.Sprintf(`_%s_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    COMPREPLY=( $(compgen -W "%s" -- "$cur") )
}
complete -F _%s_completions %s`, name, options, name, name)
}

func zshCompletion(name string, entries []completionEntry) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("#compdef %s\n", name))
	builder.WriteString(fmt.Sprintf("_arguments '*: :->cmds'\ncase $state in\n  cmds)\n    _values \"Commands\" "))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Token, "-") {
			continue
		}
		desc := strings.ReplaceAll(entry.Help, "\"", "'")
		builder.WriteString(fmt.Sprintf("'%s[%s]' ", entry.Token, desc))
	}
	builder.WriteString(";;\n  *)\n    _values 'Flags' ")
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Token, "-") {
			continue
		}
		desc := strings.ReplaceAll(entry.Help, "\"", "'")
		builder.WriteString(fmt.Sprintf("'%s[%s]' ", entry.Token, desc))
	}
	builder.WriteString(";;\nesac")
	return builder.String()
}

func fishCompletion(name string, entries []completionEntry) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("complete -c %s -f\n", name))
	for _, entry := range entries {
		desc := strings.ReplaceAll(entry.Help, "\"", "'")
		if strings.HasPrefix(entry.Token, "--") {
			builder.WriteString(fmt.Sprintf("complete -c %s -f -n '__fish_use_subcommand' -l %s", name, strings.TrimPrefix(entry.Token, "--")))
			if desc != "" {
				builder.WriteString(fmt.Sprintf(" -d '%s'", desc))
			}
			builder.WriteString("\n")
			continue
		}
		if strings.HasPrefix(entry.Token, "-") {
			builder.WriteString(fmt.Sprintf("complete -c %s -f -n '__fish_use_subcommand' -s %s", name, strings.TrimPrefix(entry.Token, "-")))
			if desc != "" {
				builder.WriteString(fmt.Sprintf(" -d '%s'", desc))
			}
			builder.WriteString("\n")
			continue
		}
		builder.WriteString(fmt.Sprintf("complete -c %s -f -n '__fish_use_subcommand' -a '%s'", name, entry.Token))
		if desc != "" {
			builder.WriteString(fmt.Sprintf(" -d '%s'", desc))
		}
		builder.WriteString("\n")
	}
	return builder.String()
}
