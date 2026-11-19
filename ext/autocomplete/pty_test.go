//go:build darwin || linux

package autocomplete

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// waitTimeout bounds every wait for shell output. It is generous: the
// probe app is this test binary, which can be slow to start under -race.
const waitTimeout = 30 * time.Second

// session is an interactive shell running on a pseudo-terminal, driven
// the way a person types: completion happens in the real line editor
// (readline or zle), not in a simulation.
type session struct {
	t    *testing.T
	term *ptytest.Term
	n    int // counter for end markers
}

// startSession starts an interactive shell on a new pty in the work
// directory and runs setup in it.
func startSession(t *testing.T, path string, args []string, setup string) *session {
	t.Helper()
	_, _, work := shellEnv(t)
	term := ptytest.New(t)
	cmd := exec.Command(path, args...)
	cmd.Env = shellEnviron(t)
	cmd.Dir = work
	cmd.Stdin, cmd.Stdout, cmd.Stderr = term.TTY, term.TTY, term.TTY
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", path, err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	s := &session{t: t, term: term}
	s.run(setup)
	return s
}

// marker returns a command that prints a new end marker, and the marker.
// The command is written with arithmetic so its echo never contains the
// marker itself.
func (s *session) marker() (cmd, mark string) {
	s.n++
	return fmt.Sprintf("echo END_$((%d+1))_", s.n), fmt.Sprintf("END_%d_", s.n+1)
}

// waitMarker waits for mark and for the prompt that follows it.
func (s *session) waitMarker(mark string) string {
	s.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		out := s.term.Output()
		if i := strings.Index(out, mark); i >= 0 && strings.Contains(out[i:], "PROMPT>") {
			return out
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("timed out waiting for %q; terminal output:\n%s", mark, clean(out))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// run types a command line and waits for it to finish.
func (s *session) run(line string) string {
	s.t.Helper()
	s.term.ResetOutput()
	cmd, mark := s.marker()
	s.term.Type(s.t, line+"; "+cmd+"\r")
	return s.waitMarker(mark)
}

var wordLine = regexp.MustCompile(`(?m)^W<(.*)>\r?$`)

// insert types line, presses Tab, and returns the words of the resulting
// command line as the shell parses them. Each one is printed by printf, so
// a candidate that was not quoted correctly shows up as a different word
// (or runs, which assertNotPwned catches).
func (s *session) insert(line string) []string {
	s.t.Helper()
	s.term.ResetOutput()
	cmd, mark := s.marker()
	// Ctrl-A goes to the start of the line, Ctrl-E to its end.
	s.term.Type(s.t, line, "\t", "\x01", `printf 'W<%s>\n' `, "\x05", " ; "+cmd+"\r")
	out := clean(s.waitMarker(mark))
	var words []string
	for _, m := range wordLine.FindAllStringSubmatch(out, -1) {
		words = append(words, m[1])
	}
	return words
}

// list types line and key (the shell's "list choices" key) and returns
// the words of the listing. It first waits for waitFor, a word the listing
// must contain, and then clears the line.
func (s *session) list(line, key, waitFor string) []string {
	s.t.Helper()
	s.term.ResetOutput()
	s.term.Type(s.t, line, key)
	if waitFor != "" {
		deadline := time.Now().Add(waitTimeout)
		for !containsWord(listing(clean(s.term.Output())), waitFor) {
			if time.Now().After(deadline) {
				s.t.Fatalf("timed out waiting for %q in the listing for %q:\n%s", waitFor, line, clean(s.term.Output()))
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	cmd, mark := s.marker()
	// Ctrl-U clears the line.
	s.term.Type(s.t, "\x15", cmd+"\r")
	out := clean(s.waitMarker(mark))
	if i := strings.Index(out, "echo END_"); i >= 0 {
		out = out[:i]
	}
	return listing(out)
}

// listing returns the words printed below the edited line: everything
// after its first line break, without lines that show the prompt.
func listing(out string) []string {
	i := strings.IndexByte(out, '\n')
	if i < 0 {
		return nil
	}
	var words []string
	for _, l := range strings.Split(out[i+1:], "\n") {
		if strings.Contains(l, "PROMPT>") {
			continue
		}
		words = append(words, strings.Fields(l)...)
	}
	return words
}

func containsWord(words []string, w string) bool {
	for _, x := range words {
		if x == w {
			return true
		}
	}
	return false
}

var ansi = regexp.MustCompile(`\x1b(\[[0-9;?]*[ -/]*[@-~]|\][^\x07]*\x07|[()][0-9A-Za-z]|[=>])`)

// clean removes escape sequences and carriage returns from terminal
// output.
func clean(s string) string {
	s = ansi.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

type insertCase struct {
	name, line string
	want       []string
}

// insertCases are completions with a single candidate: Tab must insert it
// so that the shell reads back exactly the intended words.
var insertCases = []insertCase{
	{"Command", "probe wid", []string{"probe", "widgets"}},
	{"NestedCommand", "probe widgets g", []string{"probe", "widgets", "get"}},
	{"Alias", "probe w li", []string{"probe", "w", "list"}},
	{"Flag", "probe widgets get --wa", []string{"probe", "widgets", "get", "--watch"}},
	{"AncestorFlag", "probe widgets get --al", []string{"probe", "widgets", "get", "--all"}},
	{"FlagValue", "probe widgets get --output j", []string{"probe", "widgets", "get", "--output", "json"}},
	{"FlagValueEquals", "probe widgets get --output=y", []string{"probe", "widgets", "get", "--output=yaml"}},
	{"PositionalWithSpace", "probe widgets get gam", []string{"probe", "widgets", "get", "gamma ray"}},
	{"PositionalDoubleQuoted", `probe widgets get "gam`, []string{"probe", "widgets", "get", "gamma ray"}},
	{"PositionalSingleQuoted", `probe widgets get 'gam`, []string{"probe", "widgets", "get", "gamma ray"}},
	{"NameWithDoubleQuote", "probe weird say", []string{"probe", "weird", `say"hi`}},
	{"NameWithSingleQuote", "probe weird it", []string{"probe", "weird", "it's"}},
	{"NameWithSingleQuoteQuoted", "probe weird 'it", []string{"probe", "weird", "it's"}},
	{"NameWithCommandSubstitution", "probe weird '$(t", []string{"probe", "weird", "$(touch pwned)"}},
	{"NameWithCommandSubstitutionBare", `probe weird \$`, []string{"probe", "weird", "$(touch pwned)"}},
	{"NameWithSpace", "probe weird tw", []string{"probe", "weird", "two words"}},
	{"NameWithColon", "probe weird x", []string{"probe", "weird", "x:y"}},
	{"NameWithBackslash", "probe weird ba", []string{"probe", "weird", `back\slash`}},
	{"FileFallback", "probe rm fi", []string{"probe", "rm", "file-one.txt"}},
	{"FileWithSpace", "probe rm sp", []string{"probe", "rm", "sp ace.txt"}},
	{"FileAfterDoubleDash", "probe rm -- -", []string{"probe", "rm", "--", "-x.txt"}},
}

type listCase struct {
	name, line string
	want, not  []string
}

// listCases are listings of several candidates.
var listCases = []listCase{
	{"TopLevel", "probe ", []string{"autocomplete", "cp", "rm", "weird", "widgets", "w"},
		[]string{"probe", "get", "list", "internal-debug", "__complete", "--watch", "--force"}},
	{"NestedCommands", "probe widgets ", []string{"get", "list", "ls"},
		[]string{"rm", "cp", "weird", "internal-debug"}},
	{"FlagsInScope", "probe widgets get -", allFlagsOfGet,
		[]string{"--force", "--tag", "--secret-flag", "--path"}},
	{"FlagsOutOfScope", "probe rm --", []string{"--force", "--path", "--help", "--verbose", "--org"},
		[]string{"--watch", "--all", "--output"}},
	{"FlagValueHook", "probe widgets get --output ", []string{"json", "yaml"},
		[]string{"--watch", "alpha"}},
	{"PositionalHook", "probe widgets get ", []string{"alpha", "beta"},
		[]string{"get", "json", "--watch"}},
	{"AfterDoubleDash", "probe widgets get -- ", []string{"alpha", "beta"},
		[]string{"--watch", "--output"}},
	{"HiddenOmitted", "probe in", nil, []string{"internal-debug"}},
}

func runInsertCases(t *testing.T, s *session) {
	for _, c := range insertCases {
		t.Run("Insert"+c.name, func(t *testing.T) {
			s.t = t
			got := s.insert(c.line)
			if strings.Join(got, "\x00") != strings.Join(c.want, "\x00") {
				t.Errorf("%q<TAB> gave the words %q, want %q", c.line, got, c.want)
			}
		})
	}
}

func runListCases(t *testing.T, s *session, key string) {
	for _, c := range listCases {
		t.Run("List"+c.name, func(t *testing.T) {
			s.t = t
			wait := ""
			if len(c.want) > 0 {
				wait = c.want[0]
			}
			got := s.list(c.line, key, wait)
			t.Logf("listing for %q: %q", c.line, got)
			for _, w := range c.want {
				if !containsWord(got, w) {
					t.Errorf("listing for %q lacks %q: %q", c.line, w, got)
				}
			}
			for _, w := range c.not {
				if containsWord(got, w) {
					t.Errorf("listing for %q offers %q: %q", c.line, w, got)
				}
			}
		})
	}
}

// bashSession starts an interactive bash with the probe script loaded
// (after bash-completion, when bc is set).
func bashSession(t *testing.T, bash bashInfo, bc string) *session {
	path := writeScript(t, "bash", "probe.bash")
	setup := "PS1='PROMPT> '; bind 'set page-completions off'; bind 'set completion-query-items 0'"
	if bash.major >= 4 {
		setup += "; bind 'set enable-bracketed-paste off'"
	}
	if bc != "" {
		setup += "; source " + shellSingleQuote(bc)
	}
	setup += "; source " + shellSingleQuote(path)
	return startSession(t, bash.path, []string{"--norc", "--noprofile", "-i"}, setup)
}

func runBashReadline(t *testing.T, bash bashInfo, bc string) {
	s := bashSession(t, bash, bc)
	runInsertCases(t, s)
	// Esc-? is readline's possible-completions: it lists without changing
	// the line.
	runListCases(t, s, "\x1b?")
	t.Run("InsertDirectory", func(t *testing.T) {
		s.t = t
		got := s.insert("probe rm di")
		// bash 4+ completes through readline (dir-a/ without a space);
		// bash 3.2 lists it with compgen.
		if len(got) != 3 || got[2] != "dir-a/" {
			t.Errorf("probe rm di<TAB> gave %q, want dir-a/", got)
		}
	})
	assertNotPwned(t)
}

// TestShell_Bash32_Readline completes in an interactive /bin/bash 3.2.
func TestShell_Bash32_Readline(t *testing.T) {
	runBashReadline(t, needBash(t, "bash32"), "")
}

// TestShell_Bash5_Readline completes in an interactive bash 4 or later.
func TestShell_Bash5_Readline(t *testing.T) {
	runBashReadline(t, needBash(t, "bash5"), "")
}

// TestShell_Bash5_Readline_BashCompletion does the same with the
// bash-completion package loaded.
func TestShell_Bash5_Readline_BashCompletion(t *testing.T) {
	bash := needBash(t, "bash5")
	bc := bashCompletionScript()
	if bc == "" {
		skipShell(t, "bash-completion", "the bash-completion package is not installed")
	}
	runBashReadline(t, bash, bc)
}

// zshSetup configures a zsh started with -f (no startup files).
const zshSetup = "PS1='PROMPT> '; RPS1=''; unsetopt BEEP LIST_BEEP; bindkey -e; " +
	"zstyle ':completion:*' list-prompt ''; zstyle ':completion:*' select-prompt ''"

func runZsh(t *testing.T, setup string) {
	zsh := needShell(t, "zsh", "zsh")
	s := startSession(t, zsh, []string{"-f", "-i"}, setup)
	runInsertCases(t, s)
	// Ctrl-D at the end of the line is delete-char-or-list: it lists.
	runListCases(t, s, "\x04")
	t.Run("ListDescriptions", func(t *testing.T) {
		s.t = t
		got := strings.Join(s.list("probe widgets ", "\x04", "get"), " ")
		for _, d := range []string{"Show one widget", "List widgets"} {
			if !strings.Contains(got, d) {
				t.Errorf("zsh listing lacks the description %q: %s", d, got)
			}
		}
	})
	assertNotPwned(t)
}

// TestShell_Zsh_Sourced loads the script with source after compinit.
func TestShell_Zsh_Sourced(t *testing.T) {
	path := writeScript(t, "zsh", "probe.zsh")
	runZsh(t, zshSetup+"; autoload -Uz compinit; compinit -u -D; source "+shellSingleQuote(path))
}

// TestShell_Zsh_Fpath installs the script as _probe on $fpath, the way
// zsh autoloads completion functions.
func TestShell_Zsh_Fpath(t *testing.T) {
	path := writeScript(t, "zsh", "_probe")
	runZsh(t, zshSetup+"; fpath=("+shellSingleQuote(filepath.Dir(path))+" $fpath); autoload -Uz compinit; compinit -u -D")
}

// zshListsFlags checks in a real zsh, when there is one, that the flags
// of the command at the cursor are listed.
func zshListsFlags(t *testing.T) {
	t.Helper()
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Log("zsh is not installed; TestShell_Zsh_* cover a real zsh in CI")
		return
	}
	path := writeScript(t, "zsh", "probe.zsh")
	s := startSession(t, zsh, []string{"-f", "-i"}, zshSetup+"; autoload -Uz compinit; compinit -u -D; source "+shellSingleQuote(path))
	got := s.list("probe widgets get --", "\x04", "--watch")
	for _, w := range []string{"--watch", "--output", "--all", "--verbose"} {
		if !containsWord(got, w) {
			t.Errorf("zsh lists no %s for `probe widgets get --<Ctrl-D>`: %q", w, got)
		}
	}
	if containsWord(got, "--force") {
		t.Errorf("zsh lists rm's --force for `probe widgets get --<Ctrl-D>`: %q", got)
	}
}
