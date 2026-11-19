package help

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// e2eHelpApp builds:
//
//	probe [--format/-f] [--verbose]
//	├── widgets [--org]   (group)
//	│   ├── get (alias show) [--watch/-w]
//	│   └── list
//	├── secret            (hidden)
//	└── help              (extension)
func e2eHelpApp(t *testing.T) (*clix.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(errOut), clix.WithAppInteractive(clix.InteractiveNever))
	app.Description = "Probe the widgets"
	app.AddExtension(Extension{})
	app.AddExtension(format.Extension{})
	var verbose bool
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Usage: "Verbose output"}, Value: &verbose})
	noop := func(*clix.Context) error { return nil }
	get := clix.NewCommand("get", clix.WithCommandRun(noop), clix.WithCommandShort("Get a widget"), clix.WithCommandAliases("show"))
	get.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "watch", Short: "w", Usage: "Watch for changes"}})
	list := clix.NewCommand("list", clix.WithCommandRun(noop), clix.WithCommandShort("List widgets"))
	widgets := clix.NewGroup("widgets", "Manage widgets", get, list)
	var org string
	widgets.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "org", Usage: "Organization"}, Value: &org})
	app.Root.AddCommand(widgets)
	app.Root.AddCommand(clix.NewCommand("secret", clix.WithCommandRun(noop), clix.WithCommandShort("Hidden tools"), clix.WithCommandHidden(true)))
	return app, out, errOut
}

func helpRun(t *testing.T, app *clix.App, out *bytes.Buffer, args ...string) string {
	t.Helper()
	out.Reset()
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("%q: %v", args, err)
	}
	return out.String()
}

func TestHelpE2E_Root(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	got := helpRun(t, app, out, "help")
	for _, want := range []string{"PROBE\nProbe the widgets\n", "GROUPS\n  widgets ", "COMMANDS\n  help ", "--format", "--verbose"} {
		if !strings.Contains(got, want) {
			t.Errorf("help lacks %q:\n%s", want, got)
		}
	}
	if flagHelp := helpRun(t, app, out, "--help"); flagHelp != got {
		t.Errorf("help and --help differ:\n%s\n---\n%s", got, flagHelp)
	}
}

func TestHelpE2E_Command(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	got := helpRun(t, app, out, "help", "widgets")
	if !strings.HasPrefix(got, "Manage widgets\n") || !strings.Contains(got, "  get, show ") || !strings.Contains(got, "  list ") {
		t.Errorf("help widgets:\n%s", got)
	}
	if flagHelp := helpRun(t, app, out, "widgets", "--help"); flagHelp != got {
		t.Errorf("help widgets and widgets --help differ:\n%s\n---\n%s", got, flagHelp)
	}
}

func TestHelpE2E_NestedPath(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	want := helpRun(t, app, out, "widgets", "get", "--help")
	for _, args := range [][]string{
		{"help", "widgets", "get"}, {"help", "widgets get"}, {"help", "--command", "widgets get"}, {"help", "--command=widgets", "get"}, {"help", "WIDGETS", "Get"},
	} {
		if got := helpRun(t, app, out, args...); got != want {
			t.Errorf("%q:\n%s\nwant:\n%s", args, got, want)
		}
	}
}

func TestHelpE2E_AliasPath(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	got := helpRun(t, app, out, "help", "widgets", "show")
	if !strings.HasPrefix(got, "Get a widget\n") || !strings.Contains(got, "ALIASES\n  get, show\n") {
		t.Errorf("help widgets show:\n%s", got)
	}
}

func TestHelpE2E_HelpHelp(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	got := helpRun(t, app, out, "help", "help")
	for _, want := range []string{"Show help for commands\n", "probe help [command...]", "<command>...", "--command"} {
		if !strings.Contains(got, want) {
			t.Errorf("help help lacks %q:\n%s", want, got)
		}
	}
}

// Hidden commands are left out of listings but shown when asked for.
func TestHelpE2E_HiddenCommand(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	if got := helpRun(t, app, out, "help"); strings.Contains(got, "secret") {
		t.Errorf("root help lists the hidden command:\n%s", got)
	}
	if got := helpRun(t, app, out, "help", "secret"); !strings.HasPrefix(got, "Hidden tools\n") {
		t.Errorf("help secret:\n%s", got)
	}
	if _, err := tryHelp(app, out, "help", "secrt"); err == nil || strings.Contains(err.Error(), "did you mean") {
		t.Errorf("help secrt suggests a hidden command: %v", err)
	}
}

func tryHelp(app *clix.App, out *bytes.Buffer, args ...string) (string, error) {
	out.Reset()
	err := app.Run(context.Background(), args)
	return out.String(), err
}

// Unknown commands are an error naming the word and suggesting close
// names. clix has no exported usage error, so the exit code is 1.
func TestHelpE2E_UnknownCommand(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	for args, want := range map[string]string{
		"help wigdets":       `unknown command "wigdets" for "probe"; did you mean "widgets"?`,
		"help widgets gte":   `unknown command "gte" for "probe widgets"; did you mean "get"?`,
		"help widgets sho":   `unknown command "sho" for "probe widgets"; did you mean "get"?`,
		"help widgets l":     `unknown command "l" for "probe widgets"; did you mean "list"?`,
		"help zzzzzz":        `unknown command "zzzzzz" for "probe"`,
		"help widgets get x": `unknown command "x" for "probe widgets get"`,
	} {
		got, err := tryHelp(app, out, strings.Fields(args)...)
		if err == nil || err.Error() != want {
			t.Errorf("%s: err %v, want %q", args, err, want)
			continue
		}
		if code := clix.ExitCode(err); code != 1 {
			t.Errorf("%s: exit code %d, want 1", args, code)
		}
		if got != "" {
			t.Errorf("%s: printed %q", args, got)
		}
	}
}

func TestHelpE2E_GlobalAndInheritedFlags(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	got := helpRun(t, app, out, "help", "widgets", "get")
	for _, want := range []string{
		"FLAGS\n  -h, --help           Show help information\n  -w, --watch          Watch for changes\n",
		"INHERITED FLAGS\n  --org                Organization\n",
		// Registration order: the app's --verbose, then the extension's --format.
		"GLOBAL FLAGS\n  --verbose            Verbose output\n  -f, --format         Output format (json, yaml, text)\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("help widgets get lacks %q:\n%s", want, got)
		}
	}
}

func TestHelpE2E_WritesToOut(t *testing.T) {
	app, out, errOut := e2eHelpApp(t)
	if got := helpRun(t, app, out, "help", "widgets"); got == "" || errOut.Len() != 0 {
		t.Errorf("help wrote %d bytes to Out and %q to Err", len(got), errOut.String())
	}
}

// No ANSI when NO_COLOR is set or Out is not a terminal; CLICOLOR_FORCE
// forces it.
func TestHelpE2E_Color(t *testing.T) {
	bold := clix.StyleFunc(func(s ...string) string { return "\x1b[1m" + strings.Join(s, "") + "\x1b[0m" })
	for _, tc := range []struct {
		name, noColor, force string
		want                 bool
	}{
		{"not a terminal", "", "", false},
		{"NO_COLOR", "1", "", false},
		{"NO_COLOR beats CLICOLOR_FORCE", "1", "1", false},
		{"CLICOLOR_FORCE", "", "1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, out, _ := e2eHelpApp(t)
			t.Setenv("NO_COLOR", tc.noColor)
			t.Setenv("CLICOLOR_FORCE", tc.force)
			app.Styles = clix.Styles{SectionHeading: bold, ChildName: bold, FlagName: bold}
			for _, args := range [][]string{{"help"}, {"help", "widgets", "get"}} {
				if got := strings.Contains(helpRun(t, app, out, args...), "\x1b["); got != tc.want {
					t.Errorf("%q: ANSI = %v, want %v", args, got, tc.want)
				}
			}
		})
	}
}

func TestHelpE2E_AddedTwice(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	app.AddExtension(Extension{})
	got := helpRun(t, app, out, "help")
	if n := strings.Count(got, "\n  help "); n != 1 {
		t.Errorf("help listed %d times:\n%s", n, got)
	}
	if err := app.Validate(); err != nil {
		t.Errorf("App.Validate() = %v", err)
	}
}

func TestHelpE2E_AppHelpCommandWins(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	ran := false
	app.Root.AddCommand(clix.NewCommand("help", clix.WithCommandRun(func(*clix.Context) error { ran = true; return nil })))
	if got := helpRun(t, app, out, "help"); !ran || got != "" {
		t.Errorf("help ran the app's command = %v, printed %q", ran, got)
	}
}

func TestHelpE2E_SequentialRuns(t *testing.T) {
	app, out, _ := e2eHelpApp(t)
	first := helpRun(t, app, out, "help", "widgets", "get")
	helpRun(t, app, out, "help")
	if got := helpRun(t, app, out, "help", "widgets", "get"); got != first {
		t.Errorf("third run differs:\n%s\n---\n%s", got, first)
	}
	if got := helpRun(t, app, out, "help"); !strings.HasPrefix(got, "PROBE\n") {
		t.Errorf("help after help widgets get:\n%s", got)
	}
}
