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
		usage = fmt.Sprintf("%s [flags]", cmd.Path())
	}
	fmt.Fprintf(w, "%s\n  %s\n\n", renderText(styles.SectionHeading, "USAGE"), renderText(styles.Usage, usage))

	if cmd.Long != "" {
		long := renderText(styles.CommandTitle, cmd.Long)
		fmt.Fprintf(w, "%s\n\n", long)
	}

	h.renderFlags(w, cmd)
	h.renderArguments(w, cmd)
	h.renderChildren(w, cmd)

	if cmd.Example != "" {
		example := strings.ReplaceAll(cmd.Example, "\n", "\n  ")
		fmt.Fprintf(w, "%s\n  %s\n", renderText(styles.SectionHeading, "EXAMPLES"), renderText(styles.Example, example))
	}

	return nil
}

func (h HelpRenderer) renderFlags(w io.Writer, cmd *Command) {
	flags := cmd.Flags.Flags()
	if len(flags) == 0 {
		return
	}

	fmt.Fprintln(w, renderText(h.App.Styles.SectionHeading, "FLAGS"))
	for _, flag := range flags {
		var names []string
		if flag.Short != "" {
			names = append(names, "-"+flag.Short)
		}
		names = append(names, "--"+flag.Name)
		renderedNames := renderText(h.App.Styles.FlagName, strings.Join(names, ", "))
		usage := renderText(h.App.Styles.FlagUsage, flag.Usage)
		fmt.Fprintf(w, "  %-20s %s\n", renderedNames, usage)
	}
	fmt.Fprintln(w)
}

func (h HelpRenderer) renderArguments(w io.Writer, cmd *Command) {
	if len(cmd.Arguments) == 0 {
		return
	}
	fmt.Fprintln(w, renderText(h.App.Styles.SectionHeading, "ARGUMENTS"))
	for _, arg := range cmd.Arguments {
		marker := "optional"
		if arg.Required {
			marker = "required"
		}
		name := renderText(h.App.Styles.ArgumentName, arg.Name)
		marker = renderText(h.App.Styles.ArgumentMarker, marker)
		fmt.Fprintf(w, "  %-20s %s\n", name, marker)
	}
	fmt.Fprintln(w)
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
			name := renderText(h.App.Styles.SubcommandName, group.Name)
			desc = renderText(h.App.Styles.SubcommandDesc, desc)
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
			name := renderText(h.App.Styles.SubcommandName, child.Name)
			desc = renderText(h.App.Styles.SubcommandDesc, desc)
			fmt.Fprintf(w, "  %-20s %s\n", name, desc)
		}
		fmt.Fprintln(w)
	}
}

