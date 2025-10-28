package clix

import (
	"fmt"
	"sort"
	"strings"
)

// NewAutocompleteCommand provides shell completion scripts.
func NewAutocompleteCommand(app *App) *Command {
	cmd := NewCommand("autocomplete")
	cmd.Short = "Generate shell completion script"
	cmd.Usage = fmt.Sprintf("%s autocomplete [bash|zsh|fish]", app.Name)
	cmd.Arguments = []*Argument{{Name: "shell", Prompt: "Shell", Required: true}}
	cmd.Run = func(ctx *Context) error {
		shell := strings.ToLower(ctx.Args[0])
		script, err := generateCompletionScript(app, shell)
		if err != nil {
			return err
		}
		fmt.Fprintln(app.Out, script)
		return nil
	}
	return cmd
}

func generateCompletionScript(app *App, shell string) (string, error) {
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

func collectCompletionEntries(cmd *Command) []completionEntry {
	entries := make(map[string]string)
	var walk func(*Command)
	walk = func(c *Command) {
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
		for _, sub := range c.Subcommands {
			walk(sub)
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
