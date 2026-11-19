package clix

import (
	"fmt"
	"io"
	"strings"
)

// HelpRenderer prints help text for commands.
type HelpRenderer struct {
	App     *App
	Command *Command
}

// Render writes the help to the provided writer.
func (h HelpRenderer) Render(w io.Writer) error {
	cmd := h.Command
	if cmd == nil {
		return fmt.Errorf("no command provided")
	}

	styles := h.App.Styles

	if cmd == h.App.Root {
		title := strings.ToUpper(h.App.Name)
		fmt.Fprintf(w, "%s\n", renderText(styles.AppTitle, title))
		if h.App.Description != "" {
			desc := renderText(styles.AppDescription, h.App.Description)
			fmt.Fprintf(w, "%s\n\n", desc)
		} else {
			fmt.Fprintln(w)
		}
	} else {
		short := cmd.Short
		if short == "" {
			short = cmd.Long
		}
		fmt.Fprintf(w, "%s\n\n", renderText(styles.CommandTitle, short))
	}

	usage := cmd.Usage
	if usage == "" {
		usage = h.buildUsageLine(cmd)
	}
	fmt.Fprintf(w, "%s\n  %s\n\n", renderText(styles.SectionHeading, "USAGE"), renderText(styles.Usage, usage))

	if cmd.Long != "" {
		long := renderText(styles.CommandTitle, cmd.Long)
		fmt.Fprintf(w, "%s\n\n", long)
	}

	h.renderArguments(w, cmd)
	h.renderFlags(w, cmd)
	h.renderGlobalFlags(w, cmd)
	h.renderChildren(w, cmd)

	if cmd.Example != "" {
		example := strings.ReplaceAll(cmd.Example, "\n", "\n  ")
		fmt.Fprintf(w, "%s\n  %s\n", renderText(styles.SectionHeading, "EXAMPLES"), renderText(styles.Example, example))
	}

	return nil
}

func (h HelpRenderer) renderArguments(w io.Writer, cmd *Command) {
	positionals := cmd.Flags.PositionalFlags()
	if len(positionals) == 0 {
		return
	}

	nameStyle, usageStyle := h.flagStylesFor(cmd == h.App.Root)

	fmt.Fprintln(w, renderText(h.App.Styles.SectionHeading, "ARGUMENTS"))
	for _, flag := range positionals {
		label := "<" + flag.Name + ">"
		if flag.IsVariadic() {
			label += "..."
		}
		renderedName := renderText(nameStyle, label)
		usage := flag.Usage
		if flag.Required {
			usage += " (required)"
		}
		usage = renderText(usageStyle, usage)
		fmt.Fprintf(w, "  %-20s %s\n", renderedName, usage)
	}
	fmt.Fprintln(w)
}

func (h HelpRenderer) renderFlags(w io.Writer, cmd *Command) {
	flags := cmd.Flags.Flags()
	if len(flags) == 0 {
		return
	}

	nameStyle, usageStyle := h.flagStylesFor(cmd == h.App.Root)

	fmt.Fprintln(w, renderText(h.App.Styles.SectionHeading, "FLAGS"))
	for _, flag := range flags {
		h.renderFlag(w, flag, flag.Short, nameStyle, usageStyle)
	}
	fmt.Fprintln(w)
}

// renderGlobalFlags lists the root (app-level) flags, which the parser also
// accepts after a subcommand's name. A root flag that the command shadows
// with its own flag of the same long name is left out, and a root flag's
// short name is left out when the command's own flags claim it, because the
// parser resolves both against the command first.
func (h HelpRenderer) renderGlobalFlags(w io.Writer, cmd *Command) {
	root := h.App.Root
	if root == nil || cmd == root || root.Flags == nil {
		return
	}

	type entry struct {
		flag  *Flag
		short string
	}
	var entries []entry
	for _, flag := range root.Flags.Flags() {
		if flag.Positional || cmd.Flags.lookup(flag.Name) != nil {
			continue
		}
		short := flag.Short
		if short != "" && lookupArg("-"+short, cmd.Flags) != nil {
			short = ""
		}
		entries = append(entries, entry{flag: flag, short: short})
	}
	if len(entries) == 0 {
		return
	}

	nameStyle, usageStyle := h.flagStylesFor(true)

	fmt.Fprintln(w, renderText(h.App.Styles.SectionHeading, "GLOBAL FLAGS"))
	for _, e := range entries {
		h.renderFlag(w, e.flag, e.short, nameStyle, usageStyle)
	}
	fmt.Fprintln(w)
}

// renderFlag prints a flag as "-s, --name" followed by its usage.
func (h HelpRenderer) renderFlag(w io.Writer, flag *Flag, short string, nameStyle, usageStyle TextStyle) {
	var names []string
	if short != "" {
		names = append(names, "-"+short)
	}
	names = append(names, "--"+flag.Name)
	renderedNames := renderText(nameStyle, strings.Join(names, ", "))
	usage := flag.Usage
	if flag.Required {
		usage += " (required)"
	}
	usage = renderText(usageStyle, usage)
	fmt.Fprintf(w, "  %-20s %s\n", renderedNames, usage)
}

func (h HelpRenderer) buildUsageLine(cmd *Command) string {
	var b strings.Builder
	b.WriteString(cmd.Path())
	b.WriteString(" [flags]")
	for _, f := range cmd.Flags.PositionalFlags() {
		dots := ""
		if f.IsVariadic() {
			dots = "..."
		}
		if f.Required {
			fmt.Fprintf(&b, " <%s>%s", f.Name, dots)
		} else {
			fmt.Fprintf(&b, " [%s%s]", f.Name, dots)
		}
	}
	return b.String()
}

func (h HelpRenderer) flagStylesFor(isGlobal bool) (name TextStyle, usage TextStyle) {
	if isGlobal {
		if h.App.Styles.AppFlagName != nil {
			name = h.App.Styles.AppFlagName
		} else {
			name = h.App.Styles.FlagName
		}
		if h.App.Styles.AppFlagUsage != nil {
			usage = h.App.Styles.AppFlagUsage
		} else {
			usage = h.App.Styles.FlagUsage
		}
		return
	}

	if h.App.Styles.CommandFlagName != nil {
		name = h.App.Styles.CommandFlagName
	} else {
		name = h.App.Styles.FlagName
	}

	if h.App.Styles.CommandFlagUsage != nil {
		usage = h.App.Styles.CommandFlagUsage
	} else {
		usage = h.App.Styles.FlagUsage
	}

	return
}

// renderChildren renders both groups and commands, showing them in separate sections.
func (h HelpRenderer) renderChildren(w io.Writer, cmd *Command) {
	visible := cmd.VisibleChildren()
	if len(visible) == 0 {
		return
	}

	// Separate into groups and commands
	var groups, commands []*Command
	for _, child := range visible {
		if child.IsGroup() {
			groups = append(groups, child)
		} else {
			// Include all non-groups (both leaf commands and commands without Run handlers)
			commands = append(commands, child)
		}
	}

	// Render groups first
	if len(groups) > 0 {
		fmt.Fprintln(w, renderText(h.App.Styles.SectionHeading, "GROUPS"))
		for _, group := range groups {
			desc := group.Short
			if desc == "" {
				desc = group.Long
			}
			name := renderText(h.App.Styles.ChildName, group.Name)
			desc = renderText(h.App.Styles.ChildDesc, desc)
			fmt.Fprintf(w, "  %-20s %s\n", name, desc)
		}
		fmt.Fprintln(w)
	}

	// Render commands
	if len(commands) > 0 {
		fmt.Fprintln(w, renderText(h.App.Styles.SectionHeading, "COMMANDS"))
		for _, child := range commands {
			desc := child.Short
			if desc == "" {
				desc = child.Long
			}
			name := renderText(h.App.Styles.ChildName, child.Name)
			desc = renderText(h.App.Styles.ChildDesc, desc)
			fmt.Fprintf(w, "  %-20s %s\n", name, desc)
		}
		fmt.Fprintln(w)
	}
}
