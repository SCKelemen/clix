package autocomplete

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// raceEnabled is set when the tests are built with -race.
var raceEnabled bool

// appEnv makes the test binary act as the "probe" app instead of running
// the tests, so real shells can call it through a wrapper script on PATH.
const appEnv = "CLIX_AUTOCOMPLETE_TEST_APP"

func TestMain(m *testing.M) {
	if os.Getenv(appEnv) == "1" {
		app := newProbeApp(os.Stdout, os.Stderr)
		if err := app.Run(context.Background(), os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	code := m.Run()
	cleanupShellEnv()
	os.Exit(code)
}

// weirdNames are command names that are hard to quote. None of them may
// ever run as shell code: "$(touch pwned)" would create a file.
var weirdNames = []string{`say"hi`, `it's`, `$(touch pwned)`, `two words`, `x:y`, `back\slash`}

// recorded holds the last request the "name" completion hook received.
var recorded struct {
	sync.Mutex
	req clix.CompletionRequest
}

func noop(*clix.Context) error { return nil }

// newProbeApp builds the app that every completion test drives:
//
//	probe [--verbose|-v] [--org|-o <org>]
//	  widgets (alias w) [--all|-a]       Manage widgets
//	    get [--watch|-w] [--output|-o <fmt>] <name>
//	    list (alias ls)
//	  rm [--force | --dry-run] <path>     Remove files (no hook: file names)
//	  cp [--tag <t>]... <files>...        Copy files (repeatable, variadic)
//	  weird                               commands with odd names
//	  internal-debug [--secret-flag]      Hidden
//	  autocomplete, __complete            from the Extension
func newProbeApp(out, errw io.Writer) *clix.App {
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(errw))
	app.Root.Short = "Probe completion"
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Short: "v", Usage: "Print more detail"}})
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "org", Short: "o", Usage: "Organization to use"}})
	app.Flags().SetCompletion("org", clix.CompleteValues("acme", "initech"))

	get := clix.NewCommand("get", clix.WithCommandShort("Show one widget"), clix.WithCommandRun(noop))
	get.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "watch", Short: "w", Usage: "Keep watching for changes"}})
	get.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "output", Short: "o", Usage: "Output format"}})
	get.Flags.SetCompletion("output", clix.CompleteOptions(
		clix.SelectOption{Value: "json", Label: "JSON document"},
		clix.SelectOption{Value: "yaml", Label: "YAML document"},
	))
	get.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Usage: "Widget name", Positional: true}})
	get.Flags.SetCompletion("name", func(req clix.CompletionRequest) []clix.Completion {
		recorded.Lock()
		recorded.req = req
		recorded.Unlock()
		return []clix.Completion{
			{Value: "alpha", Description: "The first widget"},
			{Value: "beta", Description: "The second widget"},
			{Value: "gamma ray", Description: "A widget with a space"},
		}
	})
	list := clix.NewCommand("list", clix.WithCommandShort("List widgets"), clix.WithCommandAliases("ls"), clix.WithCommandRun(noop))
	widgets := clix.NewGroup("widgets", "Manage widgets", get, list)
	widgets.Aliases = []string{"w"}
	widgets.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "all", Short: "a", Usage: "Include archived widgets"}})
	app.Root.AddCommand(widgets)

	rm := clix.NewCommand("rm", clix.WithCommandShort("Remove files"), clix.WithCommandRun(noop))
	rm.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force", Usage: "Do not ask"}})
	rm.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "dry-run", Usage: "Only print what would be removed"}})
	rm.Flags.MarkMutuallyExclusive("force", "dry-run")
	rm.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "path", Usage: "File to remove", Positional: true}})
	app.Root.AddCommand(rm)

	cp := clix.NewCommand("cp", clix.WithCommandShort("Copy files"), clix.WithCommandRun(noop))
	cp.Flags.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "tag", Usage: "Tag to apply (repeatable)"}})
	cp.Flags.SetCompletion("tag", clix.CompleteValues("red", "blue"))
	cp.Flags.ArgsVar(clix.ArgsVarOptions{Name: "files", Usage: "Files to copy"})
	app.Root.AddCommand(cp)

	weird := clix.NewGroup("weird", "Odd names")
	for _, name := range weirdNames {
		c := clix.NewCommand(name, clix.WithCommandRun(noop))
		c.Short = `Says "hi" [loudly]: it's \ fine`
		weird.AddCommand(c)
	}
	app.Root.AddCommand(weird)

	secret := clix.NewCommand("internal-debug", clix.WithCommandShort("Internal"), clix.WithCommandRun(noop))
	secret.Hidden = true
	secret.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "secret-flag", Usage: "Secret"}})
	app.Root.AddCommand(secret)

	// Plugins come from PATH; only the plugin tests put one there (see
	// plugin_shells_test.go).
	app.Plugins = true
	app.AddExtension(Extension{})
	return app
}

// testApp returns a probe app whose output is captured, with its config
// directory in a temporary directory.
func testApp(t *testing.T) (*clix.App, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	return newProbeApp(out, io.Discard), out
}

// runComplete runs "probe __complete args..." in process and returns the
// candidate lines and the directive.
func runComplete(t *testing.T, app *clix.App, out *bytes.Buffer, args ...string) (lines []string, directive string) {
	t.Helper()
	out.Reset()
	if err := app.Run(context.Background(), append([]string{completeName}, args...)); err != nil {
		t.Fatalf("__complete %q: %v", args, err)
	}
	text := strings.TrimSuffix(out.String(), "\n")
	all := strings.Split(text, "\n")
	directive = all[len(all)-1]
	if directive != directiveFiles && directive != directiveNoFiles {
		t.Fatalf("__complete %q: last line %q is not a directive; output:\n%s", args, directive, out.String())
	}
	return all[:len(all)-1], directive
}

// values returns the values of plain-format candidate lines.
func values(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		v, _, _ := strings.Cut(l, "\t")
		out = append(out, v)
	}
	return out
}

// completeWords completes the word cur after words (which start after the
// program name) in the plain format and returns the values and the
// directive.
func completeWords(t *testing.T, cur string, words ...string) ([]string, string) {
	t.Helper()
	app, out := testApp(t)
	lines, directive := runComplete(t, app, out, append([]string{"--cur=" + cur, "--", "probe"}, words...)...)
	return values(lines), directive
}

// sameSet reports whether got and want hold the same strings.
func sameSet(got, want []string) bool {
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	return strings.Join(g, "\x00") == strings.Join(w, "\x00")
}

func assertSet(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !sameSet(got, want) {
		t.Errorf("%s:\n got  %q\n want %q", what, got, want)
	}
}

// --- the environment real shells run in ---

var shellEnvState struct {
	once sync.Once
	dir  string // holds bin/, home/ and work/
	err  error
}

// shellEnv returns a directory with bin/probe, a wrapper that runs this
// test binary as the probe app, an empty home/ and a work/ directory
// with a few files to complete.
func shellEnv(t *testing.T) (bin, home, work string) {
	t.Helper()
	s := &shellEnvState
	s.once.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			s.err = err
			return
		}
		dir, err := os.MkdirTemp("", "clix-autocomplete-")
		if err != nil {
			s.err = err
			return
		}
		s.dir = dir
		if raceEnabled {
			// A race-enabled binary takes about a second to start on
			// macOS, and the shells start the app on every completion.
			// Use a plain build of this package's tests when go is at hand.
			plain := filepath.Join(dir, "probe.test")
			if goTool, err := exec.LookPath("go"); err == nil {
				if err := exec.Command(goTool, "test", "-c", "-o", plain, ".").Run(); err == nil {
					exe = plain
				}
			}
		}
		for _, d := range []string{"bin", "home", "work/dir-a", "work/evil"} {
			if s.err = os.MkdirAll(filepath.Join(dir, d), 0o755); s.err != nil {
				return
			}
		}
		// The names in evil/ run "touch pwned" if a shell ever reads them
		// as code instead of as a file name.
		files := []string{"work/file-one.txt", "work/-x.txt", "work/sp ace.txt", "work/dir-a/inner.txt", "work/dir-a/other.txt", evilDollar, evilBackquote}
		for _, f := range files {
			if s.err = os.WriteFile(filepath.Join(dir, f), nil, 0o644); s.err != nil {
				return
			}
		}
		wrapper := "#!/bin/sh\n" + appEnv + "=1\nexport " + appEnv + "\nexec " + shellSingleQuote(exe) + ` "$@"` + "\n"
		s.err = os.WriteFile(filepath.Join(dir, "bin", "probe"), []byte(wrapper), 0o755)
	})
	if s.err != nil {
		t.Fatalf("set up the shell environment: %v", s.err)
	}
	return filepath.Join(s.dir, "bin"), filepath.Join(s.dir, "home"), filepath.Join(s.dir, "work")
}

func cleanupShellEnv() {
	if shellEnvState.dir != "" {
		_ = os.RemoveAll(shellEnvState.dir)
	}
}

// shellEnviron returns the environment for a shell under test: probe on
// PATH, a private home and config directory, and no user configuration.
func shellEnviron(t *testing.T, extra ...string) []string {
	t.Helper()
	bin, home, _ := shellEnv(t)
	env := []string{
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + home,
		"ZDOTDIR=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"INPUTRC=/dev/null",
		"HISTFILE=/dev/null",
		"TERM=dumb",
		"LANG=C",
		"BASH_SILENCE_DEPRECATION_WARNING=1",
		"POWERSHELL_TELEMETRY_OPTOUT=1",
		"POWERSHELL_UPDATECHECK=Off",
	}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		env = append(env, "TMPDIR="+tmp)
	}
	// Later entries win (os/exec keeps the last value of a duplicate key).
	return append(env, extra...)
}

// File names in the work directory that run "touch pwned" when a shell
// evaluates them instead of reading them literally.
const (
	evilDollar    = "work/evil/zz$(touch pwned).txt"
	evilBackquote = "work/evil/yy`touch pwned`.txt"
)

// configCases are broken configurations that completion must survive:
// each is extra environment for the shell, pointing XDG_CONFIG_HOME at a
// probe/config.yaml with the given content, or setting a variable.
func configCases(t *testing.T) map[string][]string {
	t.Helper()
	dir := func(content string) string {
		d := t.TempDir()
		if err := os.MkdirAll(filepath.Join(d, "probe"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "probe", "config.yaml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return d
	}
	return map[string][]string{
		"MalformedFile":  {"XDG_CONFIG_HOME=" + dir("key: [unclosed\n")},
		"BadConfigValue": {"XDG_CONFIG_HOME=" + dir("verbose: zzz\n")},
		"BadEnvValue":    {"PROBE_VERBOSE=zzz"},
	}
}

// assertNotPwned fails the test if completing the "$(touch pwned)"
// command ran it.
func assertNotPwned(t *testing.T) {
	t.Helper()
	_, home, work := shellEnv(t)
	for _, dir := range []string{work, home} {
		if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
			t.Fatalf("a command name was executed as shell code: %s/pwned exists", dir)
		}
	}
}

// writeScript writes the probe app's completion script for shell to a
// temporary file and returns its path.
func writeScript(t *testing.T, shell, name string) string {
	t.Helper()
	text, err := script("probe", shell)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
