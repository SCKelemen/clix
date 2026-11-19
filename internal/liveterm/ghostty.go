package liveterm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Ghostty is a Ghostty window a test opened through Ghostty's AppleScript
// dictionary (/Applications/Ghostty.app/Contents/Resources/Ghostty.sdef).
// Every command names the window and terminal by the IDs "new window"
// returned; nothing is ever sent to the front, selected or focused window.
type Ghostty struct {
	t        testing.TB
	windowID string
	termID   string
	once     sync.Once
}

// ghosttyKeys maps key names to Ghostty's "send key" names, which are
// its key codes in camel case (the W3C UI Events code values).
var ghosttyKeys = map[string]string{
	"up": "arrowUp", "down": "arrowDown", "left": "arrowLeft", "right": "arrowRight",
	"home": "home", "end": "end", "pageup": "pageUp", "pagedown": "pageDown",
	"insert": "insert", "delete": "delete", "backspace": "backspace",
	"tab": "tab", "enter": "enter", "escape": "escape",
}

var ghosttyMods = map[string]string{"shift": "shift", "ctrl": "control", "alt": "option", "cmd": "command"}

// GhosttyVersion returns the version of the installed Ghostty.
func GhosttyVersion(t testing.TB) string {
	t.Helper()
	return strings.TrimSpace(osascript(t, `on run argv
	tell application "Ghostty" to return version
end run`))
}

// NewGhostty opens a new Ghostty window that runs command (an executable,
// no arguments) in dir. The window first runs a launcher that sets the
// title to a unique value and waits; the command starts only after the
// window's terminal has been found by its ID and its title checked, so no
// input is ever sent to a terminal that is not this test's. The window is
// closed when the test ends.
func NewGhostty(t testing.TB, dir, command string) *Ghostty {
	t.Helper()
	title := Title("clix-live")
	goFile := filepath.Join(dir, ".liveterm-go")
	launcher := filepath.Join(dir, ".liveterm-launch.sh")
	script := fmt.Sprintf("#!/bin/sh\nprintf '\\033]2;%%s\\007' %s\nwhile [ ! -e %s ]; do sleep 0.05; done\nexec %s\n",
		shellQuote(title), shellQuote(goFile), shellQuote(command))
	if err := os.WriteFile(launcher, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	out := osascript(t, `on run argv
	tell application "Ghostty"
		set cfg to new surface configuration
		set command of cfg to (item 1 of argv)
		set initial working directory of cfg to (item 2 of argv)
		set wait after command of cfg to false
		set w to new window with configuration cfg
		set t to terminal 1 of w
		return (id of w) & linefeed & (id of t)
	end tell
end run`, launcher, dir)
	ids := strings.Fields(out)
	if len(ids) != 2 || ids[0] == "" || ids[1] == "" {
		t.Fatalf("ghostty: new window returned %q, not a window and terminal ID; nothing was sent", out)
	}
	g := &Ghostty{t: t, windowID: ids[0], termID: ids[1]}
	t.Cleanup(g.Close)

	// The terminal must be the one in our window and show our title.
	var name string
	ok := waitFor(10*time.Second, func() bool {
		name, _ = g.run(`on run argv
	tell application "Ghostty"
		set w to first window whose id is (item 1 of argv)
		set t to first terminal of w whose id is (item 2 of argv)
		return name of t
	end tell
end run`)
		return strings.TrimSpace(name) == title
	})
	if !ok {
		t.Fatalf("ghostty: terminal %s in window %s has title %q, want %q; not sending anything to it", g.termID, g.windowID, name, title)
	}
	if err := os.WriteFile(goFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return g
}

// Kind returns "ghostty".
func (g *Ghostty) Kind() string { return "ghostty" }

// run runs an AppleScript with the window and terminal IDs as its first
// two arguments.
func (g *Ghostty) run(script string, args ...string) (string, error) {
	return runOsascript(script, append([]string{g.windowID, g.termID}, args...)...)
}

const ghosttyTarget = `
		set w to first window whose id is (item 1 of argv)
		set t to first terminal of w whose id is (item 2 of argv)`

// SendKey presses a key with "send key", a key event that goes through
// Ghostty's key encoder (and its keybindings). Printable keys need text,
// which "send key" does not carry: use Type for them.
func (g *Ghostty) SendKey(k Key) error {
	name, ok := ghosttyKeys[k.Name]
	if !ok {
		name = k.Name // f1-f12 and letters keep their names
	}
	var mods []string
	for _, m := range k.Mods {
		gm, ok := ghosttyMods[m]
		if !ok {
			return fmt.Errorf("ghostty: unknown modifier %q", m)
		}
		mods = append(mods, gm)
	}
	_, err := g.run(`on run argv
	tell application "Ghostty"`+ghosttyTarget+`
		if (item 4 of argv) is "" then
			send key (item 3 of argv) to t
		else
			send key (item 3 of argv) modifiers (item 4 of argv) to t
		end if
	end tell
end run`, name, strings.Join(mods, ","))
	return err
}

// Type sends text with the "text:" action, which writes it to the
// terminal as typed, the way a keybinding's text action (or an input
// method committing a character) does.
func (g *Ghostty) Type(text string) error {
	_, err := g.run(`on run argv
	tell application "Ghostty"`+ghosttyTarget+`
		if not (perform action (item 3 of argv) on t) then error "text action failed"
	end tell
end run`, "text:"+ghosttyTextEscape(text))
	return err
}

// Paste sends text with "input text", which Ghostty delivers as a paste
// (inside bracketed paste markers when the program turned them on).
func (g *Ghostty) Paste(text string) error {
	_, err := g.run(`on run argv
	tell application "Ghostty"`+ghosttyTarget+`
		input text (item 3 of argv) to t
	end tell
end run`, text)
	return err
}

// Describe says which AppleScript command sends a key, typed text or a
// paste.
func (g *Ghostty) Describe(what string, k Key, text string) string {
	switch what {
	case "key":
		name, ok := ghosttyKeys[k.Name]
		if !ok {
			name = k.Name
		}
		var mods []string
		for _, m := range k.Mods {
			mods = append(mods, ghosttyMods[m])
		}
		if len(mods) == 0 {
			return fmt.Sprintf("send key %q", name)
		}
		return fmt.Sprintf("send key %q modifiers %q", name, strings.Join(mods, ","))
	case "type":
		return fmt.Sprintf("perform action %q", "text:"+ghosttyTextEscape(text))
	}
	return fmt.Sprintf("input text %q", text)
}

// Close closes the window. Ghostty hangs up the programs running in it.
func (g *Ghostty) Close() {
	g.once.Do(func() {
		if _, err := g.run(`on run argv
	tell application "Ghostty"
		close window (first window whose id is (item 1 of argv))
	end tell
end run`); err != nil {
			g.t.Logf("ghostty: close window %s: %v", g.windowID, err)
		}
	})
}

// ghosttyTextEscape writes text the way Ghostty's text action reads it:
// a Zig string literal body, where backslash starts an escape.
func ghosttyTextEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// osascript runs an AppleScript and fails the test on error.
func osascript(t testing.TB, script string, args ...string) string {
	t.Helper()
	out, err := runOsascript(script, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// runOsascript runs script with args (as "on run argv"), so no argument is
// ever parsed as AppleScript.
func runOsascript(script string, args ...string) (string, error) {
	cmd := exec.Command("osascript", append([]string{"-"}, args...)...)
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "-1743") || strings.Contains(strings.ToLower(msg), "not authorized") {
			return "", fmt.Errorf("osascript is not allowed to control Ghostty (%s): allow it in System Settings > Privacy & Security > Automation", msg)
		}
		return "", fmt.Errorf("osascript: %v: %s", err, msg)
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
