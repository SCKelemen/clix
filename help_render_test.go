package clix

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
)

// renderHelp runs "args... --help" and returns the output.
func renderHelp(t *testing.T, app *App, args ...string) string {
	t.Helper()
	out := &bytes.Buffer{}
	app.Out = out
	if err := app.Run(context.Background(), append(args, "--help")); err != nil {
		t.Fatalf("%q --help: %v", args, err)
	}
	return out.String()
}

func newHelpTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	return NewApp("probe", WithAppErr(&bytes.Buffer{}))
}

func TestHelpRendersVariadicArgument(t *testing.T) {
	app := newHelpTestApp(t)
	add := NewCommand("add", WithCommandRun(func(*Context) error { return nil }))
	var dest string
	add.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "dest", Usage: "Destination", Positional: true, Required: true}, Value: &dest})
	add.Flags.ArgsVar(ArgsVarOptions{Name: "path", Usage: "Files to add"})
	app.Root.AddCommand(add)

	out := renderHelp(t, app, "add")
	if !strings.Contains(out, "probe add [flags] <dest> [path...]") {
		t.Errorf("usage line should mark the variadic argument:\n%s", out)
	}
	if !strings.Contains(out, "  <path>...") || !strings.Contains(out, "  <dest> ") {
		t.Errorf("ARGUMENTS should mark the variadic argument:\n%s", out)
	}
}

// globalFlagsApp has root flags --format/-f, --verbose/-v and --region, and
// a "deploy" command that shadows --region and claims -f for --force.
func globalFlagsApp(t *testing.T) *App {
	t.Helper()
	app := newHelpTestApp(t)
	var format, region, cmdRegion string
	var verbose, force bool
	app.Flags().StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "format", Short: "f", Usage: "Output format"}, Value: &format})
	app.Flags().BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "verbose", Short: "v", Usage: "Verbose output"}, Value: &verbose})
	app.Flags().StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "region", Usage: "Default region"}, Value: &region})
	deploy := NewCommand("deploy", WithCommandRun(func(*Context) error { return nil }))
	deploy.Short = "Deploy the app"
	deploy.Flags.BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "force", Short: "f", Usage: "Skip checks"}, Value: &force})
	deploy.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "region", Usage: "Deploy region"}, Value: &cmdRegion})
	app.Root.AddCommand(NewGroup("apps", "Manage apps", deploy))
	return app
}

// section returns the lines of the named help section, without the
// heading.
func section(out, heading string) []string {
	var lines []string
	in := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == heading:
			in = true
		case in && line == "":
			return lines
		case in:
			lines = append(lines, line)
		}
	}
	return lines
}

func TestHelpListsGlobalFlags(t *testing.T) {
	app := globalFlagsApp(t)
	out := renderHelp(t, app, "apps", "deploy")
	global := strings.Join(section(out, "GLOBAL FLAGS"), "\n")
	if !strings.Contains(global, "--format") || !strings.Contains(global, "-v, --verbose") {
		t.Errorf("GLOBAL FLAGS should list the root flags:\n%s", out)
	}
	// deploy's -f is --force, so the root's --format is listed without it.
	if strings.Contains(global, "-f, --format") {
		t.Errorf("GLOBAL FLAGS shows -f for --format, but -f means --force here:\n%s", out)
	}
	// deploy defines its own --region and --help.
	if strings.Contains(global, "--region") || strings.Contains(global, "--help") {
		t.Errorf("GLOBAL FLAGS lists a flag the command shadows:\n%s", out)
	}
	if local := strings.Join(section(out, "FLAGS"), "\n"); !strings.Contains(local, "Deploy region") || strings.Contains(local, "--format") {
		t.Errorf("FLAGS should hold only the command's own flags:\n%s", out)
	}

	if root := renderHelp(t, globalFlagsApp(t)); strings.Contains(root, "GLOBAL FLAGS") {
		t.Errorf("root help should not repeat its flags as GLOBAL FLAGS:\n%s", root)
	}
}

func TestHelpGlobalFlagsUseAppFlagStyles(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	app := globalFlagsApp(t)
	app.Styles = Styles{
		AppFlagName:     StyleFunc(func(s ...string) string { return "APP:" + s[0] }),
		AppFlagUsage:    StyleFunc(func(s ...string) string { return "APPU:" + s[0] }),
		CommandFlagName: StyleFunc(func(s ...string) string { return "CMD:" + s[0] }),
	}
	out := renderHelp(t, app, "apps", "deploy")
	global := strings.Join(section(out, "GLOBAL FLAGS"), "\n")
	if !strings.Contains(global, "APP:-v, --verbose") || !strings.Contains(global, "APPU:Verbose output") {
		t.Errorf("GLOBAL FLAGS should use the app flag styles:\n%s", out)
	}
	if local := strings.Join(section(out, "FLAGS"), "\n"); !strings.Contains(local, "CMD:-f, --force") {
		t.Errorf("FLAGS should use the command flag styles:\n%s", out)
	}
}

var ansiSGR = regexp.MustCompile("\x1b\\[[0-9;]*m")

// Padding with %-20s counted escape bytes as width, so the description
// column moved when styles were set.
func TestHelpColumnsAlignWithStyles(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	red := StyleFunc(func(s ...string) string { return "\x1b[31m" + strings.Join(s, "") + "\x1b[0m" })
	for _, args := range [][]string{nil, {"apps"}, {"apps", "deploy"}} {
		plain := renderHelp(t, globalFlagsApp(t), args...)
		app := globalFlagsApp(t)
		app.Styles = Styles{FlagName: red, ChildName: red, ChildDesc: red, FlagUsage: red, SectionHeading: red}
		styled := renderHelp(t, app, args...)
		if !strings.Contains(styled, "\x1b[31m") {
			t.Fatalf("%q: styles were not applied:\n%s", args, styled)
		}
		if got := ansiSGR.ReplaceAllString(styled, ""); got != plain {
			t.Errorf("%q: visible help changes when styles are applied.\nplain:\n%s\nstyled, escapes removed:\n%s", args, plain, got)
		}
	}
}

func TestHelpPrintsLongOnceWithoutShort(t *testing.T) {
	app := newHelpTestApp(t)
	cmd := NewCommand("sync", WithCommandRun(func(*Context) error { return nil }))
	cmd.Long = "Synchronize the local cache with the server."
	app.Root.AddCommand(cmd)
	out := renderHelp(t, app, "sync")
	if n := strings.Count(out, cmd.Long); n != 1 {
		t.Errorf("Long appears %d times:\n%s", n, out)
	}

	cmd.Short = "Sync the cache"
	out = renderHelp(t, app, "sync")
	if !strings.HasPrefix(out, "Sync the cache\n") || strings.Count(out, cmd.Long) != 1 {
		t.Errorf("with Short set, help should show Short as the title and Long once:\n%s", out)
	}
}

func TestHelpShowsAliases(t *testing.T) {
	app := newHelpTestApp(t)
	get := NewCommand("get", WithCommandRun(func(*Context) error { return nil }), WithCommandAliases("show", "g"))
	get.Short = "Get a widget"
	app.Root.AddCommand(NewGroup("widgets", "Manage widgets", get))

	out := renderHelp(t, app, "widgets")
	if !strings.Contains(out, "  get, show, g         Get a widget\n") {
		t.Errorf("COMMANDS should list the aliases:\n%s", out)
	}
	out = renderHelp(t, app, "widgets", "get")
	if got := section(out, "ALIASES"); len(got) != 1 || got[0] != "  get, show, g" {
		t.Errorf("command help should have an ALIASES section, got %q:\n%s", got, out)
	}
}

// Help without styles keeps its layout byte for byte.
func TestHelpPlainLayout(t *testing.T) {
	app := globalFlagsApp(t)
	got := renderHelp(t, app, "apps", "deploy")
	want := `Deploy the app

USAGE
  probe apps deploy [flags]

FLAGS
  -h, --help           Show help information
  -f, --force          Skip checks
  --region             Deploy region

GLOBAL FLAGS
  --format             Output format
  -v, --verbose        Verbose output

`
	if got != want {
		t.Errorf("help output:\n%s\nwant:\n%s", got, want)
	}
}

// Styles must not write ANSI escapes to pipes and files, or when NO_COLOR
// or TERM=dumb asks for plain output; CLICOLOR_FORCE=1 forces them.
func TestHelpColorFollowsEnvironment(t *testing.T) {
	bold := StyleFunc(func(s ...string) string { return "\x1b[1m" + strings.Join(s, "") + "\x1b[0m" })
	marker := StyleFunc(func(s ...string) string { return "APP:" + strings.Join(s, "") })
	for _, tc := range []struct {
		name                      string
		noColor, force, term, cli string
		wantANSI                  bool
	}{
		{name: "non-terminal output", term: "xterm-256color"},
		{name: "NO_COLOR", noColor: "1", term: "xterm-256color"},
		{name: "CLICOLOR_FORCE", force: "1", term: "xterm-256color", wantANSI: true},
		{name: "CLICOLOR_FORCE=0", force: "0", term: "xterm-256color"},
		{name: "NO_COLOR beats CLICOLOR_FORCE", noColor: "1", force: "1", term: "xterm-256color"},
		{name: "TERM=dumb", term: "dumb"},
		{name: "CLICOLOR=0", cli: "0", term: "xterm-256color"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tc.noColor)
			t.Setenv("CLICOLOR_FORCE", tc.force)
			t.Setenv("CLICOLOR", tc.cli)
			t.Setenv("TERM", tc.term)
			app := globalFlagsApp(t)
			app.Styles = Styles{SectionHeading: bold, AppTitle: bold, ChildName: bold, AppFlagName: marker}
			for _, args := range [][]string{nil, {"apps", "deploy"}} {
				out := renderHelp(t, app, args...)
				if got := strings.Contains(out, "\x1b["); got != tc.wantANSI {
					t.Errorf("%q: ANSI written = %v, want %v:\n%q", args, got, tc.wantANSI, out)
				}
				// Text a style adds besides escapes is kept either way.
				if visible := ansiSGR.ReplaceAllString(out, ""); !strings.Contains(visible, "USAGE\n") || !strings.Contains(visible, "APP:") {
					t.Errorf("%q: styled text lost:\n%q", args, out)
				}
			}
		})
	}
}

// The parser looks a flag after the command name up on the command, then
// each ancestor, so help lists the groups' flags too, the way the parser
// resolves them.
func TestHelpListsInheritedFlags(t *testing.T) {
	app := newHelpTestApp(t)
	var verbose bool
	var org, groupRegion, region, output string
	app.Flags().BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "verbose", Short: "v", Usage: "Verbose output"}, Value: &verbose})
	deploy := NewCommand("deploy", WithCommandRun(func(*Context) error { return nil }))
	deploy.Short = "Deploy the app"
	deploy.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "region", Usage: "Deploy region"}, Value: &region})
	deploy.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "output", Short: "o", Usage: "Output file"}, Value: &output})
	apps := NewGroup("apps", "Manage apps", deploy)
	apps.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "org", Short: "o", Usage: "Organization"}, Value: &org})
	apps.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "region", Usage: "Default region"}, Value: &groupRegion})
	app.Root.AddCommand(apps)

	out := renderHelp(t, app, "apps", "deploy")
	if got := section(out, "INHERITED FLAGS"); len(got) != 1 || !strings.HasPrefix(got[0], "  --org ") {
		t.Errorf("INHERITED FLAGS = %q, want only --org (without -o, which is --output here):\n%s", got, out)
	}
	if got := section(out, "GLOBAL FLAGS"); len(got) != 1 || !strings.HasPrefix(got[0], "  -v, --verbose ") {
		t.Errorf("GLOBAL FLAGS = %q:\n%s", got, out)
	}
	// The group's own help lists its flags under FLAGS and nothing inherited.
	group := renderHelp(t, app, "apps")
	if strings.Contains(group, "INHERITED FLAGS") || !strings.Contains(strings.Join(section(group, "FLAGS"), "\n"), "-o, --org") {
		t.Errorf("group help:\n%s", group)
	}

	// Help matches the parser.
	app.Out = &bytes.Buffer{}
	if err := app.Run(context.Background(), []string{"apps", "deploy", "--org", "acme", "-o", "out.txt", "--region", "eu", "-v"}); err != nil {
		t.Fatal(err)
	}
	if org != "acme" || output != "out.txt" || region != "eu" || groupRegion != "" || !verbose {
		t.Errorf("parsed org=%q output=%q region=%q group region=%q verbose=%v", org, output, region, groupRegion, verbose)
	}
}
