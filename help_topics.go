package clix

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/SCKelemen/clix/v2/internal/tty"
)

// HelpTopic is a page of help that is not about one command, such as gh's
// "help environment" or gcloud's "topic" pages. Users read it with
// "myapp help <name>" (the help extension, ext/help), and root help lists
// the topics under ADDITIONAL HELP TOPICS.
type HelpTopic struct {
	// Name is how the topic is asked for: "myapp help <name>".
	Name string

	// Short is a one-line summary, shown in the list and as the title.
	Short string

	// Body is the text of the topic.
	Body string

	owner
	builtin func(*App) string // the body of a built-in topic, written when it is shown
}

// builtinRank orders the built-in topics after every topic an app or
// extension adds, so those replace them.
const builtinRank = runtimeRank + 1

// builtinTopics are the topics clix ships, about what it owns: the
// environment variables it reads and the exit statuses it returns.
var builtinTopics = []HelpTopic{
	{Name: "environment", Short: "Environment variables that change how the program behaves", builtin: environmentTopic, owner: owner{rank: builtinRank}},
	{Name: "exit-codes", Short: "Exit codes and what they mean", builtin: exitCodesTopic, owner: owner{rank: builtinRank}},
}

// AddHelpTopic adds the help topic name, with a one-line summary and its
// text, which "myapp help <name>" shows when the help extension is added
// (see ext/help). Root help lists the topics under ADDITIONAL HELP TOPICS
// when the root has a help command.
//
// clix ships two topics, "environment" (the environment variables that
// clix reads for this app) and "exit-codes" (the exit statuses ExitCode
// returns). A topic of the same name replaces them, and an empty body
// removes a topic, so AddHelpTopic("exit-codes", "", "") drops the
// built-in one. As with flags and commands, the app's topics win over an
// extension's, whenever they are added, and between extensions the one
// added first wins; adding a topic again replaces the same owner's.
//
// Names are matched like command names (ASCII case is ignored unless
// App.CaseSensitive is set), and a command of the root with that name or
// alias wins over a topic, which App.Validate reports.
func (a *App) AddHelpTopic(name, short, body string) {
	t := HelpTopic{Name: name, Short: short, Body: body}
	if a.extending {
		t.claim(a.extRank, a.extName)
	}
	for i, old := range a.topics {
		if old.rank == t.rank && a.namesMatch(old.Name, name) {
			a.topics[i] = t
			return
		}
	}
	a.topics = append(a.topics, t)
}

// topicWinners returns the topic in effect for each name, built-in ones
// included, removed ones left out, sorted by name.
func (a *App) topicWinners() []HelpTopic {
	var winners []HelpTopic
	all := append(append([]HelpTopic(nil), a.topics...), builtinTopics...)
	for _, t := range all {
		replaced := false
		for i, w := range winners {
			if a.namesMatch(w.Name, t.Name) {
				replaced = true
				if t.rank < w.rank {
					winners[i] = t
				}
				break
			}
		}
		if !replaced {
			winners = append(winners, t)
		}
	}
	var out []HelpTopic
	for _, t := range winners {
		if t.Body == "" && t.builtin == nil {
			continue // removed
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// HelpTopics returns the help topics, sorted by name: the app's,
// extensions' and the built-in ones that were not replaced or removed,
// except those a root command shadows. Built-in topics have their Body
// written for the app as it is now.
func (a *App) HelpTopics() []HelpTopic {
	var out []HelpTopic
	for _, t := range a.topicWinners() {
		if a.Root != nil && a.Root.findChildMatch(t.Name, a.CaseSensitive) != nil {
			continue
		}
		out = append(out, a.finishTopic(t))
	}
	return out
}

// LookupHelpTopic returns the topic name (see HelpTopics), matching names
// like command names.
func (a *App) LookupHelpTopic(name string) (HelpTopic, bool) {
	for _, t := range a.HelpTopics() {
		if a.namesMatch(t.Name, name) {
			return t, true
		}
	}
	return HelpTopic{}, false
}

// finishTopic writes a built-in topic's body.
func (a *App) finishTopic(t HelpTopic) HelpTopic {
	if t.builtin != nil {
		t.Body = t.builtin(a)
	}
	return t
}

// RenderTopic writes the help topic t: its summary as the title, then its
// body, styled and with color as for command help (see Render).
func (h HelpRenderer) RenderTopic(w io.Writer, t HelpTopic) error {
	if h.App == nil {
		return fmt.Errorf("no app provided")
	}
	p := helpPrinter{HelpRenderer: h, w: w, color: tty.ColorEnabled(w)}
	title := t.Short
	if title == "" {
		title = t.Name
	}
	_, _ = fmt.Fprintf(w, "%s\n\n", p.style(h.App.Styles.CommandTitle, title))
	_, err := fmt.Fprintf(w, "%s\n", strings.TrimRight(t.Body, "\n"))
	return err
}

// renderTopics lists the help topics under ADDITIONAL HELP TOPICS, as gh
// does, when the root has a help command to show them.
func (p helpPrinter) renderTopics() {
	if p.App.Root == nil || p.App.Root.findChildMatch("help", p.App.CaseSensitive) == nil {
		return
	}
	topics := p.App.HelpTopics()
	if len(topics) == 0 {
		return
	}
	p.heading("ADDITIONAL HELP TOPICS")
	for _, t := range topics {
		p.row(p.style(p.App.Styles.ChildName, t.Name), p.style(p.App.Styles.ChildDesc, t.Short))
	}
	_, _ = fmt.Fprintln(p.w)
}

// validateTopics reports topics a root command shadows and names that
// cannot be typed as one word.
func (a *App) validateTopics(errs *[]error) {
	for _, t := range a.topicWinners() {
		if t.builtin != nil {
			continue // clix's own: nothing for the app to fix
		}
		if bad := untypable(t.Name); bad != "" || t.Name == "" {
			*errs = append(*errs, fmt.Errorf("%s: %shelp topic name %q cannot be typed as one word", a.Root.Name, extPrefix(t.owner), t.Name))
			continue
		}
		if cmd := a.Root.findChildMatch(t.Name, a.CaseSensitive); cmd != nil {
			*errs = append(*errs, fmt.Errorf("%s: %shelp topic %q is shadowed by the command %q; \"help %s\" shows the command", a.Root.Name, extPrefix(t.owner), t.Name, cmd.Name, t.Name))
		}
	}
}

// topicRow is one line of a built-in topic's table.
type topicRow struct{ name, desc string }

// writeRows writes rows as an indented two-column table, wrapping the
// descriptions at 80 columns.
func writeRows(b *strings.Builder, rows []topicRow) {
	width := 0
	for _, r := range rows {
		width = max(width, len(r.name))
	}
	width = min(width, 24)
	indent := strings.Repeat(" ", 2+width+2)
	for _, r := range rows {
		lines := wrapWords(r.desc, 80-len(indent))
		if len(r.name) > width {
			fmt.Fprintf(b, "  %s\n", r.name)
			fmt.Fprintf(b, "%s%s\n", indent, lines[0])
		} else {
			fmt.Fprintf(b, "  %-*s  %s\n", width, r.name, lines[0])
		}
		for _, l := range lines[1:] {
			fmt.Fprintf(b, "%s%s\n", indent, l)
		}
	}
}

// wrapWords breaks text into lines of at most width bytes, between words.
func wrapWords(text string, width int) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(text) {
		switch {
		case line == "":
			line = w
		case len(line)+1+len(w) > width:
			lines = append(lines, line)
			line = w
		default:
			line += " " + w
		}
	}
	return append(lines, line)
}

// environmentTopic writes the "environment" topic: the variables clix
// reads for this app.
func environmentTopic(a *App) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s reads these environment variables. An empty variable counts as unset.\n\n", a.Name)

	var rows []topicRow
	seen := map[string]bool{}
	add := func(name, desc string) {
		if !seen[name] {
			seen[name] = true
			rows = append(rows, topicRow{name, desc})
		}
	}
	if a.autoEnv() {
		add(a.EnvPrefix+"_<FLAG>", fmt.Sprintf("The value of a flag: --dry-run reads %s, unless the flag is only read from the command line. The command line wins over it, and it wins over the config file.", a.envVarName("dry-run")))
	}
	if a.configurations {
		add(a.ConfigurationEnvVar(), fmt.Sprintf("The named configuration to use, instead of the active one (see \"%s config configurations\").", a.Name))
	}
	if fs := a.rootFlagSet(); fs != nil {
		for _, f := range fs.Flags() {
			if isBuiltinSwitch(f) {
				continue
			}
			for _, v := range a.flagEnvVars(f) {
				desc := "--" + f.Name
				if f.Usage != "" {
					desc += ": " + f.Usage
				}
				add(v, desc)
			}
		}
	}
	add(a.DisablePromptsEnvVar(), "Set to 1, true, yes or on to never prompt for missing required flags: commands then fail with an error that names them, as in a script.")
	if a.ConfigPath == "" {
		add("XDG_CONFIG_HOME", fmt.Sprintf("The directory that holds %s's config directory: $XDG_CONFIG_HOME/%s, and ~/.config/%s when it is not set.", a.Name, a.Name, a.Name))
	}
	add("NO_COLOR", "Set to anything to turn color off (https://no-color.org). It wins over CLICOLOR_FORCE.")
	add("CLICOLOR", "0 turns color off.")
	add("CLICOLOR_FORCE", "Set to anything but 0 to use color even when the output is not a terminal (https://bixense.com/clicolors).")
	add("TERM", "dumb turns color off, and makes prompts read plain lines.")
	writeRows(&b, rows)
	return b.String()
}

// exitCodesTopic writes the "exit-codes" topic: what ExitCode returns.
func exitCodesTopic(a *App) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s exits with one of these statuses:\n\n", a.Name)
	writeRows(&b, []topicRow{
		{"0", "Success."},
		{"1", "An error."},
		{"2", "A usage error: an unknown command or flag, a bad flag value (from the command line, an environment variable or the config file), unexpected arguments, a missing required flag, or flags that cannot be used together."},
		{"130", "Cancelled: interrupted with Ctrl-C (SIGINT), or a prompt was cancelled."},
		{"143", "Terminated by SIGTERM."},
	})
	return b.String()
}
