//go:build darwin

package prompt_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/internal/liveterm"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// TestLivePrompts drives the demo app (live_demo_test.go) in each enabled
// terminal with real key events and checks the answers, that a secret is
// never written to the terminal, that NO_COLOR removes color, and that the
// terminal settings (stty -a) after every run, including a cancelled one,
// are what they were before.
//
//	CLIX_LIVE_TERMINALS=ghostty,cmux go test -run TestLivePrompts -v ./ext/prompt
func TestLivePrompts(t *testing.T) {
	t.Run("ghostty", func(t *testing.T) {
		liveterm.Require(t, "ghostty")
		runLivePrompts(t, func(dir, command string) liveterm.Terminal { return liveterm.NewGhostty(t, dir, command) }, "xterm-ghostty")
	})
	t.Run("cmux", func(t *testing.T) {
		liveterm.RequireCmux(t)
		runLivePrompts(t, func(dir, command string) liveterm.Terminal { return liveterm.NewCmux(t, dir, command) }, "")
	})
}

// demoRun is one run of the demo app in the terminal.
type demoRun struct {
	name   string   // names its files in the session directory
	args   []string // the demo app's arguments
	env    []string // extra environment
	record bool     // run under script(1), keeping the output in <name>.typescript
}

// demoSession is a terminal running demo app runs one after another.
type demoSession struct {
	t    *testing.T
	term liveterm.Terminal
	dir  string
}

// startDemo opens a terminal that runs each run in turn, taking stty -a
// before and after each one.
func startDemo(t *testing.T, open func(dir, command string) liveterm.Terminal, runs []demoRun) *demoSession {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var b strings.Builder
	fmt.Fprintf(&b, "#!/bin/sh\ncd %s || exit 1\nexport XDG_CONFIG_HOME=%s LANG=en_US.UTF-8 LC_ALL= LC_CTYPE=\nunset NO_COLOR CLICOLOR CLICOLOR_FORCE\n",
		shq(dir), shq(filepath.Join(dir, "config")))
	for _, r := range runs {
		f := func(ext string) string { return shq(filepath.Join(dir, r.name+ext)) }
		env := append([]string{liveHelperEnv + "=demo", "CLIX_DEMO_OUT=" + filepath.Join(dir, r.name+".json"), "CLIX_DEMO_PROGRESS=" + filepath.Join(dir, r.name+".progress")}, r.env...)
		var cmd []string
		for _, e := range env {
			cmd = append(cmd, shq(e))
		}
		cmd = append(cmd, shq(exe))
		for _, a := range r.args {
			cmd = append(cmd, shq(a))
		}
		inner := "tty > " + f(".tty") + "; exec env " + strings.Join(cmd, " ")
		fmt.Fprintf(&b, "stty -a > %s\n", f(".stty-before"))
		if r.record {
			fmt.Fprintf(&b, "script -q %s /bin/sh -c %s\n", f(".typescript"), shq(inner))
		} else {
			fmt.Fprintf(&b, "/bin/sh -c %s\n", shq(inner))
		}
		fmt.Fprintf(&b, "echo $? > %s\nstty -a > %s\n", f(".exit"), f(".stty-after"))
	}
	fmt.Fprintf(&b, "touch %s\n", shq(filepath.Join(dir, "finished")))
	script := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(script, []byte(b.String()), 0o700); err != nil {
		t.Fatal(err)
	}
	return &demoSession{t: t, term: open(dir, script), dir: dir}
}

func (s *demoSession) path(name, ext string) string { return filepath.Join(s.dir, name+ext) }

// waitPrompt waits until run has started its n-th prompt and the
// terminal is in raw mode, so keys go to that prompt.
func (s *demoSession) waitPrompt(run string, n int) {
	s.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		b, _ := os.ReadFile(s.path(run, ".progress"))
		if strings.Count(string(b), "\n") >= n && s.isRaw(run) {
			time.Sleep(100 * time.Millisecond) // let the prompt draw
			return
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("%s: prompt %d never started; progress %q", run, n, b)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// isRaw reports whether the terminal the run uses is out of canonical
// mode. The descriptor is only used for the termios query, never read.
func (s *demoSession) isRaw(run string) bool {
	name, err := os.ReadFile(s.path(run, ".tty"))
	if err != nil {
		return false
	}
	f, err := os.OpenFile(strings.TrimSpace(string(name)), os.O_RDONLY|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	canon, err := ptytest.Canonical(f)
	return err == nil && !canon
}

// result waits for a run to end and returns what it answered.
func (s *demoSession) result(run string) demoResult {
	s.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(s.path(run, ".stty-after")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("%s did not finish", run)
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // stty -a may still be writing
	var r demoResult
	b, err := os.ReadFile(s.path(run, ".json"))
	if err != nil {
		s.t.Fatalf("%s wrote no results: %v", run, err)
	}
	if err := json.Unmarshal(b, &r); err != nil {
		s.t.Fatalf("%s results: %v", run, err)
	}
	before, after := sttyModes(s.path(run, ".stty-before")), sttyModes(s.path(run, ".stty-after"))
	if before == "" || before != after {
		rawBefore, _ := os.ReadFile(s.path(run, ".stty-before"))
		rawAfter, _ := os.ReadFile(s.path(run, ".stty-after"))
		s.t.Errorf("%s: terminal settings changed:\nbefore:\n%s\nafter:\n%s", run, rawBefore, rawAfter)
	}
	return r
}

func (s *demoSession) keys(keys ...string) {
	s.t.Helper()
	for _, k := range keys {
		if err := s.term.SendKey(liveterm.K(k)); err != nil {
			s.t.Fatal(err)
		}
	}
}

func (s *demoSession) typeText(text string) {
	s.t.Helper()
	if err := s.term.Type(text); err != nil {
		s.t.Fatal(err)
	}
}

func (s *demoSession) paste(text string) {
	s.t.Helper()
	if err := s.term.Paste(text); err != nil {
		s.t.Fatal(err)
	}
}

// answers returns the values a run answered, with "!cancelled" for a
// cancelled prompt.
func answers(r demoResult) []string {
	var out []string
	for _, a := range r.Answers {
		v := a.Value
		if a.Cancelled {
			v = "!cancelled"
		} else if a.Error != "" {
			v = "!" + a.Error
		}
		out = append(out, v)
	}
	return out
}

func wantAnswers(t *testing.T, run string, r demoResult, want ...string) {
	t.Helper()
	if got := answers(r); !reflect.DeepEqual(got, want) {
		t.Errorf("%s answered %q, want %q (error %q)", run, got, want, r.Error)
	}
}

// screener is a terminal whose screen can be read (cmux).
type screener interface {
	ReadScreen() (string, error)
}

// waitScreen waits until the terminal shows want, when it can be read.
func (s *demoSession) waitScreen(want string) (string, bool) {
	s.t.Helper()
	sc, ok := s.term.(screener)
	if !ok {
		return "", false
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		screen, err := sc.ReadScreen()
		if err == nil && strings.Contains(screen, want) {
			return screen, true
		}
		if time.Now().After(deadline) {
			s.t.Errorf("screen never showed %q:\n%s", want, screen)
			return screen, true
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func runLivePrompts(t *testing.T, open func(dir, command string) liveterm.Terminal, wantTerm string) {
	runs := []demoRun{
		{name: "edit", args: []string{"text"}},
		{name: "select", args: []string{"select"}},
		{name: "multiselect", args: []string{"multiselect"}},
		{name: "paste", args: []string{"text"}},
		{name: "secret", args: []string{"secret"}, record: true},
		{name: "confirm", args: []string{"confirm"}},
		{name: "survey", args: []string{"survey"}},
		{name: "cancel", args: []string{"cancel"}},
		{name: "color", args: []string{"color"}, record: true},
		{name: "nocolor", args: []string{"color"}, env: []string{"NO_COLOR=1"}, record: true},
	}
	s := startDemo(t, open, runs)

	// Editing by grapheme: word moves with Alt and Ctrl, Backspace on é
	// and on a ZWJ emoji, a combining accent, a flag.
	s.waitPrompt("edit", 1)
	s.typeText("naïve café 日本 👩‍💻")
	s.keys("alt+left", "alt+left", "backspace", "backspace")
	s.typeText("e\u0301")
	s.keys("ctrl+right", "right", "right", "backspace", "home", "delete")
	s.typeText("N")
	s.keys("ctrl+e")
	s.typeText("🇸🇪")
	s.keys("left", "left")
	s.typeText("!")
	if screen, ok := s.waitScreen("? Name: Naïve"); ok {
		t.Logf("edit screen:\n%s", screen)
	}
	s.keys("enter")
	r := s.result("edit")
	wantAnswers(t, "edit", r, "Naïve cafe\u0301日本! 🇸🇪")
	if wantTerm != "" && r.Term != wantTerm {
		t.Errorf("TERM in the terminal is %q, want %q", r.Term, wantTerm)
	}
	t.Logf("TERM=%s", r.Term)

	// Select: arrows, Ctrl-P/Ctrl-N, End, wrapping, Enter.
	s.waitPrompt("select", 1)
	s.keys("down", "down", "ctrl+p", "end", "ctrl+n", "down", "enter")
	wantAnswers(t, "select", s.result("select"), "production")

	// Multi-select: Space and Enter toggle, End goes to Continue.
	s.waitPrompt("multiselect", 1)
	s.typeText(" ")
	s.keys("down", "down", "enter", "end", "enter")
	wantAnswers(t, "multiselect", s.result("multiselect"), "production-eu,staging")

	// A multi-line paste in the middle of the text keeps its first line.
	s.waitPrompt("paste", 1)
	s.typeText("[]")
	s.keys("left")
	s.paste("pasted 日本 👩‍💻\nsecond line")
	s.keys("enter")
	wantAnswers(t, "paste", s.result("paste"), "[pasted 日本 👩‍💻]")

	// A secret is never written to the terminal.
	s.waitPrompt("secret", 1)
	s.typeText("hunter2 日本")
	s.keys("backspace")
	if screen, ok := s.waitScreen("? Token: "); ok && strings.Contains(screen, "hunter2") {
		t.Errorf("the secret is on screen:\n%s", screen)
	}
	s.keys("enter")
	wantAnswers(t, "secret", s.result("secret"), "hunter2 日")
	if ts, err := os.ReadFile(s.path("secret", ".typescript")); err != nil {
		t.Error(err)
	} else if !strings.Contains(string(ts), "? Token: ") || strings.Contains(string(ts), "hunter2") {
		t.Errorf("secret run output %q: want the prompt and no secret", ts)
	}

	// Confirm: an invalid answer is refused, then yes.
	s.waitPrompt("confirm", 1)
	s.typeText("nope")
	s.keys("enter")
	s.waitScreen("please enter 'y' or 'n'")
	s.keys("ctrl+u")
	s.typeText("yes")
	s.keys("enter")
	wantAnswers(t, "confirm", s.result("confirm"), "y")

	// A survey: text, select, confirm (default yes).
	s.waitPrompt("survey", 1)
	s.typeText("clix")
	s.keys("enter")
	s.waitPrompt("survey", 2)
	s.keys("down", "enter")
	s.waitPrompt("survey", 3)
	s.keys("enter")
	wantAnswers(t, "survey", s.result("survey"), "clix", "production", "y")

	// Esc cancels a select, Ctrl-C a text prompt; the app exits with 130
	// and the terminal settings are restored (checked by result).
	s.waitPrompt("cancel", 1)
	s.keys("escape")
	s.waitPrompt("cancel", 2)
	s.typeText("abc")
	s.keys("ctrl+c")
	r = s.result("cancel")
	wantAnswers(t, "cancel", r, "!cancelled", "!cancelled")
	if r.Exit != 130 {
		t.Errorf("cancelled run exited with %d, want 130", r.Exit)
	}

	// Theme color is shown, and removed with NO_COLOR.
	for _, run := range []string{"color", "nocolor"} {
		s.waitPrompt(run, 1)
		s.typeText("x")
		s.keys("enter")
		wantAnswers(t, run, s.result(run), "x")
		ts, err := os.ReadFile(s.path(run, ".typescript"))
		if err != nil {
			t.Fatal(err)
		}
		colored := strings.Contains(string(ts), "\x1b[31mName")
		if colored != (run == "color") {
			t.Errorf("%s: label colored = %v; output %q", run, colored, ts)
		}
	}
	waitFile(t, filepath.Join(s.dir, "finished"))
}

func waitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) || time.Now().After(deadline) {
			t.Fatalf("%s: %v", path, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// pendin matches the PENDIN flag in stty -a output. The macOS kernel sets
// it itself whenever a terminal goes from non-canonical back to canonical
// mode (so pending input is reprocessed), whatever the termios a program
// restores says, and clears it at the next read.
var pendin = regexp.MustCompile(`\s-?pendin\b`)

// sttyModes returns the settings in an stty -a listing, without its first
// line (speed and window size, which a window manager may change) and
// without PENDIN.
func sttyModes(path string) string {
	b, _ := os.ReadFile(path)
	if _, rest, ok := strings.Cut(string(b), "\n"); ok {
		return pendin.ReplaceAllString(rest, "")
	}
	return ""
}
