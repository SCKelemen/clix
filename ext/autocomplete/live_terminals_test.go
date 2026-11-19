//go:build darwin

package autocomplete

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/internal/liveterm"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// TestLiveCompletion completes with the probe app's scripts in shells
// running in real terminals, with the terminal's shell integration
// loaded the way the terminal loads it: for zsh, ZDOTDIR pointing at
// Ghostty's integration directory (what Ghostty's automatic injection
// does); for bash, Ghostty's ghostty.bash sourced (Ghostty does not
// inject into macOS's bash 3.2, and its documentation has users source
// it). Keys are real key events (Tab, Ctrl-A, Ctrl-E, Enter); the
// completed line is printed to a file by the shell itself, so the test
// needs no screen reading. In cmux, which can read its screen, listings
// are checked too.
//
//	CLIX_LIVE_TERMINALS=ghostty,cmux go test -run TestLiveCompletion -v ./ext/autocomplete
func TestLiveCompletion(t *testing.T) {
	terminals := []struct {
		kind string
		open func(t *testing.T, dir, command string) liveterm.Terminal
	}{
		{"ghostty", func(t *testing.T, dir, command string) liveterm.Terminal { return liveterm.NewGhostty(t, dir, command) }},
		{"cmux", func(t *testing.T, dir, command string) liveterm.Terminal { return liveterm.NewCmux(t, dir, command) }},
	}
	for _, term := range terminals {
		t.Run(term.kind, func(t *testing.T) {
			if term.kind == "cmux" {
				liveterm.RequireCmux(t)
			} else {
				liveterm.Require(t, term.kind)
			}
			open := func(dir, command string) liveterm.Terminal { return term.open(t, dir, command) }
			t.Run("zsh", func(t *testing.T) {
				zsh := needShell(t, "zsh", "zsh")
				s := startLiveShell(t, open, zsh, []string{"-d", "-i"}, true,
					zshSetup+"; autoload -Uz compinit; compinit -u -D; source <(probe autocomplete zsh)",
					"(( $+functions[_ghostty_deferred_init] || $+functions[_ghostty_precmd] ))")
				runLiveInserts(t, s, insertCases)
				runLiveLists(t, s, "\x04")
			})
			for _, key := range []string{"bash32", "bash5"} {
				t.Run(key, func(t *testing.T) {
					bash := needBash(t, key)
					setup := `[ -r "$GHOSTTY_RESOURCES_DIR/shell-integration/bash/ghostty.bash" ] && source "$GHOSTTY_RESOURCES_DIR/shell-integration/bash/ghostty.bash"; ` +
						"PS1='PROMPT> '; bind 'set page-completions off'; bind 'set completion-query-items 0'"
					if bash.major >= 4 {
						setup += "; bind 'set enable-bracketed-paste off'"
					}
					setup += `; eval "$(probe autocomplete bash)"`
					s := startLiveShell(t, open, bash.path, []string{"--norc", "--noprofile", "-i"}, false, setup, "declare -F __ghostty_precmd >/dev/null")
					runLiveInserts(t, s, append(append([]insertCase(nil), insertCases...), bashInsertCases...))
					runLiveLists(t, s, "\x1b?")
				})
			}
			assertNotPwned(t)
		})
	}
}

// liveSession is an interactive shell in a real terminal.
type liveSession struct {
	t    *testing.T
	term liveterm.Terminal
	out  string // directory the shell writes results to
	tty  string
	n    int
}

// startLiveShell starts shell in a new terminal with the completion test
// environment (shellEnviron) and the terminal's own TERM and integration
// variables, runs setup in it, and reports whether integration (a shell
// test) is active.
func startLiveShell(t *testing.T, open func(dir, command string) liveterm.Terminal, shell string, args []string, zshInject bool, setup, integration string) *liveSession {
	t.Helper()
	_, home, work := shellEnv(t)
	dir := t.TempDir()
	var env []string
	for _, kv := range shellEnviron(t, "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8", "OUT="+dir) {
		if !strings.HasPrefix(kv, "TERM=") {
			env = append(env, shellSingleQuote(kv))
		}
	}
	// Variables the terminal set, passed on when set.
	for _, name := range []string{"TERM", "TERMINFO", "COLORTERM", "TERM_PROGRAM", "TERM_PROGRAM_VERSION",
		"GHOSTTY_RESOURCES_DIR", "GHOSTTY_SHELL_FEATURES", "GHOSTTY_BIN_DIR",
		"CMUX_WORKSPACE_ID", "CMUX_SURFACE_ID", "CMUX_TAB_ID", "CMUX_PANEL_ID", "CMUX_SOCKET_PATH"} {
		env = append(env, fmt.Sprintf(`${%s:+"%s=$%s"}`, name, name, name))
	}
	if zshInject {
		// What Ghostty's automatic zsh integration does: zsh reads the
		// .zshenv in Ghostty's directory, which restores ZDOTDIR from
		// GHOSTTY_ZSH_ZDOTDIR and loads ghostty-integration.
		env = append(env, `${GHOSTTY_RESOURCES_DIR:+"ZDOTDIR=$GHOSTTY_RESOURCES_DIR/shell-integration/zsh"}`,
			`${GHOSTTY_RESOURCES_DIR:+"GHOSTTY_ZSH_ZDOTDIR=`+home+`"}`)
	}
	quotedArgs := make([]string, len(args))
	for i, a := range args {
		quotedArgs[i] = shellSingleQuote(a)
	}
	script := fmt.Sprintf("#!/bin/sh\ncd %s || exit 1\ntty > %s\nexec env -i %s %s %s\n",
		shellSingleQuote(work), shellSingleQuote(filepath.Join(dir, "tty")), strings.Join(env, " "), shellSingleQuote(shell), strings.Join(quotedArgs, " "))
	run := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(run, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	// w prints its arguments one per line as W<word>; it is defined in a
	// file so nothing typed needs a backslash (cmux send reads \n as Enter).
	if err := os.WriteFile(filepath.Join(dir, "w.sh"), []byte("w() { printf 'W<%s>\\n' \"$@\"; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &liveSession{t: t, term: open(dir, run), out: dir}
	s.waitLineEditor()
	s.typeLine(setup + `; . "$OUT/w.sh"; if ` + integration + `; then echo integration > "$OUT/ready"; else echo none > "$OUT/ready"; fi`)
	ready := s.waitFile("ready")
	t.Logf("%s in %s: shell integration %s", filepath.Base(shell), s.term.Kind(), map[string]string{"integration": "loaded", "none": "not loaded"}[strings.TrimSpace(ready)])
	return s
}

// waitLineEditor waits until the shell's line editor has the terminal
// out of canonical mode, so typed keys reach it one at a time.
func (s *liveSession) waitLineEditor() {
	s.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if b, err := os.ReadFile(filepath.Join(s.out, "tty")); err == nil {
			s.tty = strings.TrimSpace(string(b))
			if f, err := os.OpenFile(s.tty, os.O_RDONLY|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0); err == nil {
				canon, err := ptytest.Canonical(f)
				_ = f.Close()
				if err == nil && !canon {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			s.t.Fatal("the shell never started reading a line")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *liveSession) key(k string) {
	s.t.Helper()
	if err := s.term.SendKey(liveterm.K(k)); err != nil {
		s.t.Fatal(err)
	}
}

func (s *liveSession) typeText(text string) {
	s.t.Helper()
	if text == "" {
		return
	}
	if err := s.term.Type(text); err != nil {
		s.t.Fatal(err)
	}
}

func (s *liveSession) typeLine(line string) {
	s.t.Helper()
	s.typeText(line)
	s.key("enter")
}

// waitFile waits for the shell to write a file in the output directory
// and returns its content.
func (s *liveSession) waitFile(name string) string {
	s.t.Helper()
	path := filepath.Join(s.out, name)
	deadline := time.Now().Add(waitTimeout)
	for {
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
			time.Sleep(50 * time.Millisecond)
			b, _ = os.ReadFile(path)
			return string(b)
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("the shell never wrote %s", name)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

var liveWordLine = regexp.MustCompile(`(?m)^W<(.*)>$`)

// insert types line (a tab in it is a Tab press), presses Tab, and
// returns the words of the completed line, which the shell prints to a
// file: Ctrl-A puts w in front of the line, Ctrl-E goes back to its end
// to add the redirection.
func (s *liveSession) insert(line string) []string {
	s.t.Helper()
	s.n++
	name := fmt.Sprintf("words-%d", s.n)
	for _, part := range strings.Split(line, "\t") {
		s.typeText(part)
		s.key("tab")
	}
	s.key("ctrl+a")
	s.typeText("w ")
	s.key("ctrl+e")
	s.typeLine(` > "$OUT/` + name + `"`)
	var words []string
	for _, m := range liveWordLine.FindAllStringSubmatch(s.waitFile(name), -1) {
		words = append(words, m[1])
	}
	return words
}

func runLiveInserts(t *testing.T, s *liveSession, cases []insertCase) {
	for _, c := range append(append([]insertCase(nil), cases...), liveNonASCIICases...) {
		t.Run("Insert"+c.name, func(t *testing.T) {
			s.t = t
			if got := s.insert(c.line); strings.Join(got, "\x00") != strings.Join(c.want, "\x00") {
				t.Errorf("%q<TAB> gave the words %q, want %q", c.line, got, c.want)
			}
		})
	}
}

// liveNonASCIICases complete with multibyte text on the line; \x02 is
// Ctrl-B, typed as its byte.
var liveNonASCIICases = []insertCase{
	{"NonASCIIBeforeCursor", "probe --org 日本 wid zz\x02\x02\x02", []string{"probe", "--org", "日本", "widgets", "zz"}},
	{"NonASCIIAtEnd", "probe --org 日本 widgets get --wa", []string{"probe", "--org", "日本", "widgets", "get", "--watch"}},
}

// screenReader is a terminal whose screen can be read (cmux).
type screenReader interface {
	ReadScreen() (string, error)
}

// runLiveLists checks the listings of several candidates, in terminals
// that can read their screen.
func runLiveLists(t *testing.T, s *liveSession, key string) {
	sc, ok := s.term.(screenReader)
	if !ok {
		t.Logf("%s cannot read its screen: listings are checked in the pseudo-terminal tests only", s.term.Kind())
		return
	}
	for _, c := range listCases {
		t.Run("List"+c.name, func(t *testing.T) {
			s.t = t
			s.typeText(c.line + key)
			var words []string
			deadline := time.Now().Add(waitTimeout)
			for {
				screen, err := sc.ReadScreen()
				if err != nil {
					t.Fatal(err)
				}
				words = nil
				if i := strings.LastIndex(screen, c.line); i >= 0 {
					for _, w := range strings.Fields(screen[i+len(c.line):]) {
						if w != "PROMPT>" {
							words = append(words, w)
						}
					}
				}
				if len(c.want) == 0 || containsWord(words, c.want[0]) || time.Now().After(deadline) {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			for _, w := range c.want {
				if !containsWord(words, w) {
					t.Errorf("listing for %q lacks %q: %q", c.line, w, words)
				}
			}
			for _, w := range c.not {
				if containsWord(words, w) {
					t.Errorf("listing for %q offers %q: %q", c.line, w, words)
				}
			}
			s.key("ctrl+u")
			s.n++
			s.typeLine(fmt.Sprintf(`echo > "$OUT/list-%d"`, s.n))
			s.waitFile(fmt.Sprintf("list-%d", s.n))
		})
	}
}
