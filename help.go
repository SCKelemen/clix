package clix

import (
	"fmt"
	"io"
	"strings"

	"github.com/SCKelemen/clix/v2/internal/tty"
)

// helpNameWidth is the minimum width, in terminal columns, of the name
// column in the ARGUMENTS, FLAGS, GLOBAL FLAGS, GROUPS and COMMANDS
// sections. A longer name pushes its description one space to the right.
const helpNameWidth = 20

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
	p := helpPrinter{HelpRenderer: h, w: w}
	p.render(cmd)
	return nil
}

// helpPrinter renders one help screen to a writer.
type helpPrinter struct {
	HelpRenderer
	w io.Writer
}

// style renders value with s.
func (p helpPrinter) style(s TextStyle, value string) string {
	return renderText(s, value)
}

// heading prints a section heading.
func (p helpPrinter) heading(title string) {
	fmt.Fprintln(p.w, p.style(p.App.Styles.SectionHeading, title))
}

// row prints one entry of a two-column section. The name is padded by its
// visible width, so escape sequences added by styles do not count toward
// the column width (as they do with fmt's %-20s).
func (p helpPrinter) row(name, desc string) {
	fmt.Fprintf(p.w, "  %s %s\n", tty.PadRight(name, helpNameWidth), desc)
}

func (p helpPrinter) render(cmd *Command) {
	w := p.w
	styles := p.App.Styles
	isRoot := cmd == p.App.Root

	if isRoot {
		title := strings.ToUpper(p.App.Name)
		fmt.Fprintf(w, "%s\n", p.style(styles.AppTitle, title))
		if p.App.Description != "" {
			fmt.Fprintf(w, "%s\n\n", p.style(styles.AppDescription, p.App.Description))
		} else {
			fmt.Fprintln(w)
		}
	} else {
		title := cmd.Short
		if title == "" {
			title = cmd.Long
		}
		fmt.Fprintf(w, "%s\n\n", p.style(styles.CommandTitle, title))
	}

	usage := cmd.Usage
	if usage == "" {
		usage = p.buildUsageLine(cmd)
	}
	p.heading("USAGE")
	fmt.Fprintf(w, "  %s\n\n", p.style(styles.Usage, usage))

	if !isRoot && len(cmd.Aliases) > 0 {
		p.heading("ALIASES")
		fmt.Fprintf(w, "  %s\n\n", p.style(styles.ChildName, commandNames(cmd)))
	}

	// When Short is empty, Long was already printed as the title.
	if cmd.Long != "" && (isRoot || cmd.Short != "") {
		fmt.Fprintf(w, "%s\n\n", p.style(styles.CommandTitle, cmd.Long))
	}

	p.renderArguments(cmd)
	p.renderFlags(cmd)
	p.renderGlobalFlags(cmd)
	p.renderChildren(cmd)

	if cmd.Example != "" {
		example := strings.ReplaceAll(cmd.Example, "\n", "\n  ")
		p.heading("EXAMPLES")
		fmt.Fprintf(w, "  %s\n", p.style(styles.Example, example))
	}
}

func (p helpPrinter) renderArguments(cmd *Command) {
	positionals := cmd.Flags.PositionalFlags()
	if len(positionals) == 0 {
		return
	}

	nameStyle, usageStyle := p.flagStylesFor(cmd == p.App.Root)

	p.heading("ARGUMENTS")
	for _, flag := range positionals {
		label := "<" + flag.Name + ">"
		if flag.IsVariadic() {
			label += "..."
		}
		usage := flag.Usage
		if flag.Required {
			usage += " (required)"
		}
		p.row(p.style(nameStyle, label), p.style(usageStyle, usage))
	}
	fmt.Fprintln(p.w)
}

func (p helpPrinter) renderFlags(cmd *Command) {
	flags := cmd.Flags.Flags()
	if len(flags) == 0 {
		return
	}

	nameStyle, usageStyle := p.flagStylesFor(cmd == p.App.Root)

	p.heading("FLAGS")
	for _, flag := range flags {
		p.flagRow(flag, flag.Short, nameStyle, usageStyle)
	}
	fmt.Fprintln(p.w)
}

// renderGlobalFlags lists the root (app-level) flags, which the parser also
// accepts after a subcommand's name. A root flag that the command shadows
// with its own flag of the same long name is left out, and a root flag's
// short name is left out when the command's own flags claim it, because the
// parser resolves both against the command first.
func (p helpPrinter) renderGlobalFlags(cmd *Command) {
	root := p.App.Root
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

	nameStyle, usageStyle := p.flagStylesFor(true)

	p.heading("GLOBAL FLAGS")
	for _, e := range entries {
		p.flagRow(e.flag, e.short, nameStyle, usageStyle)
	}
	fmt.Fprintln(p.w)
}

// flagRow prints a flag as "-s, --name" followed by its usage.
func (p helpPrinter) flagRow(flag *Flag, short string, nameStyle, usageStyle TextStyle) {
	var names []string
	if short != "" {
		names = append(names, "-"+short)
	}
	names = append(names, "--"+flag.Name)
	usage := flag.Usage
	if flag.Required {
		usage += " (required)"
	}
	p.row(p.style(nameStyle, strings.Join(names, ", ")), p.style(usageStyle, usage))
}

// renderChildren renders both groups and commands, showing them in separate sections.
func (p helpPrinter) renderChildren(cmd *Command) {
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

	p.renderChildSection("GROUPS", groups)
	p.renderChildSection("COMMANDS", commands)
}

// renderChildSection lists child commands by name and aliases, with their
// descriptions.
func (p helpPrinter) renderChildSection(title string, children []*Command) {
	if len(children) == 0 {
		return
	}
	p.heading(title)
	for _, child := range children {
		desc := child.Short
		if desc == "" {
			desc = child.Long
		}
		p.row(p.style(p.App.Styles.ChildName, commandNames(child)), p.style(p.App.Styles.ChildDesc, desc))
	}
	fmt.Fprintln(p.w)
}

// commandNames returns the command's name followed by its aliases, as in
// "get, show".
func commandNames(cmd *Command) string {
	if len(cmd.Aliases) == 0 {
		return cmd.Name
	}
	return cmd.Name + ", " + strings.Join(cmd.Aliases, ", ")
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
