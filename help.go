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

	if cmd == h.App.Root {
		fmt.Fprintf(w, "%s\n", strings.ToUpper(h.App.Name))
		if h.App.Description != "" {
			fmt.Fprintf(w, "%s\n\n", h.App.Description)
		} else {
			fmt.Fprintln(w)
		}
	} else {
		fmt.Fprintf(w, "%s\n\n", cmd.Short)
	}

	usage := cmd.Usage
	if usage == "" {
		usage = fmt.Sprintf("%s [flags]", cmd.Path())
	}
	fmt.Fprintf(w, "USAGE\n  %s\n\n", usage)

	if cmd.Long != "" {
		fmt.Fprintf(w, "%s\n\n", cmd.Long)
	}

	h.renderFlags(w, cmd)
	h.renderArguments(w, cmd)
	h.renderSubcommands(w, cmd)

	if cmd.Example != "" {
		fmt.Fprintf(w, "EXAMPLES\n  %s\n", strings.ReplaceAll(cmd.Example, "\n", "\n  "))
	}

	return nil
}

func (h HelpRenderer) renderFlags(w io.Writer, cmd *Command) {
	flags := cmd.Flags.Flags()
	if len(flags) == 0 {
		return
	}

	fmt.Fprintln(w, "FLAGS")
	for _, flag := range flags {
		var names []string
		if flag.Short != "" {
			names = append(names, "-"+flag.Short)
		}
		names = append(names, "--"+flag.Name)
		fmt.Fprintf(w, "  %-20s %s\n", strings.Join(names, ", "), flag.Usage)
	}
	fmt.Fprintln(w)
}

func (h HelpRenderer) renderArguments(w io.Writer, cmd *Command) {
	if len(cmd.Arguments) == 0 {
		return
	}
	fmt.Fprintln(w, "ARGUMENTS")
	for _, arg := range cmd.Arguments {
		marker := "optional"
		if arg.Required {
			marker = "required"
		}
		fmt.Fprintf(w, "  %-20s %s\n", arg.Name, marker)
	}
	fmt.Fprintln(w)
}

func (h HelpRenderer) renderSubcommands(w io.Writer, cmd *Command) {
	subs := cmd.VisibleSubcommands()
	if len(subs) == 0 {
		return
	}
	fmt.Fprintln(w, "SUBCOMMANDS")
	for _, sub := range subs {
		desc := sub.Short
		if desc == "" {
			desc = sub.Long
		}
		fmt.Fprintf(w, "  %-20s %s\n", sub.Name, desc)
	}
	fmt.Fprintln(w)
}
