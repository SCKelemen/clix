package liveterm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// CmuxCLI is the cmux command-line client. It talks to the running app
// over a Unix socket.
const CmuxCLI = "/Applications/cmux.app/Contents/Resources/bin/cmux"

// Cmux is a cmux workspace a test created with "cmux new-workspace". Every
// command names the workspace and surface by the refs the creating calls
// returned; nothing is sent to the focused workspace or surface.
//
// cmux's default socket mode ("cmux processes only") lets only processes
// started inside cmux terminals connect, so run the live tests from a
// terminal inside cmux, or choose a socket mode that admits them in
// cmux's settings.
type Cmux struct {
	t         testing.TB
	workspace string
	surface   string
	pane      string
	once      sync.Once
}

var cmuxRef = regexp.MustCompile(`\b(workspace|surface|pane):[0-9]+\b`)

// cmuxKeys maps key names to cmux send-key names.
var cmuxKeys = map[string]string{
	"pageup": "page_up", "pagedown": "page_down",
}

var cmuxMods = map[string]string{"shift": "shift", "ctrl": "ctrl", "alt": "alt", "cmd": "cmd"}

// RequireCmux skips the test when the cmux app does not answer, for
// example because it is not running or its socket mode does not admit
// this process.
func RequireCmux(t testing.TB) {
	t.Helper()
	Require(t, "cmux")
	if _, err := os.Stat(CmuxCLI); err != nil {
		t.Skipf("cmux is not installed: %v", err)
	}
	out, err := cmuxRun("ping")
	if err != nil {
		t.Skipf("cmux does not answer (%v); start it with open -a cmux and run the tests from a terminal inside cmux, or allow external clients in cmux's socket settings", strings.TrimSpace(out))
	}
}

// CmuxVersion returns the version of the installed cmux.
func CmuxVersion(t testing.TB) string {
	t.Helper()
	out, err := cmuxRun("version")
	if err != nil {
		t.Fatalf("cmux version: %v: %s", err, out)
	}
	if f := strings.Fields(out); len(f) >= 2 {
		return f[1]
	}
	return strings.TrimSpace(out)
}

// NewCmux creates a workspace in dir that runs command (an executable, no
// arguments). Like NewGhostty, a launcher first prints a unique marker and
// waits; the command starts only after the marker was read back from the
// workspace's surface. The workspace is closed when the test ends.
func NewCmux(t testing.TB, dir, command string) *Cmux {
	t.Helper()
	marker := Title("clix-live")
	goFile := filepath.Join(dir, ".liveterm-go")
	launcher := filepath.Join(dir, ".liveterm-launch.sh")
	script := fmt.Sprintf("#!/bin/sh\nclear\nprintf '\\033]2;%%s\\007%%s\\n' %s %s\nwhile [ ! -e %s ]; do sleep 0.05; done\nclear\nexec %s\n",
		shellQuote(marker), shellQuote(marker), shellQuote(goFile), shellQuote(command))
	if err := os.WriteFile(launcher, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	out, err := cmuxRun("new-workspace", "--cwd", dir, "--command", "exec "+shellQuote(launcher))
	if err != nil {
		t.Fatalf("cmux new-workspace: %v: %s", err, out)
	}
	ws := cmuxRef.FindString(out)
	if !strings.HasPrefix(ws, "workspace:") {
		t.Fatalf("cmux new-workspace printed %q, not a workspace ref; nothing was sent", out)
	}
	c := &Cmux{t: t, workspace: ws}
	t.Cleanup(c.Close)

	out, err = cmuxRun("list-pane-surfaces", "--workspace", ws)
	if err != nil {
		t.Fatalf("cmux list-pane-surfaces: %v: %s", err, out)
	}
	var surfaces []string
	for _, m := range cmuxRef.FindAllString(out, -1) {
		if strings.HasPrefix(m, "surface:") {
			surfaces = append(surfaces, m)
		}
	}
	if len(surfaces) != 1 {
		t.Fatalf("cmux workspace %s has surfaces %q, want exactly one; nothing was sent", ws, surfaces)
	}
	c.surface = surfaces[0]
	if out, err := cmuxRun("list-panes", "--workspace", ws); err == nil {
		c.pane = regexp.MustCompile(`\bpane:[0-9]+\b`).FindString(out)
	}

	ok := waitFor(15*time.Second, func() bool {
		screen, err := c.ReadScreen()
		return err == nil && strings.Contains(screen, marker)
	})
	if !ok {
		screen, _ := c.ReadScreen()
		t.Fatalf("cmux surface %s of %s never showed the marker %q; not sending anything to it. Screen:\n%s", c.surface, ws, marker, screen)
	}
	if err := os.WriteFile(goFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return c
}

// Kind returns "cmux".
func (c *Cmux) Kind() string { return "cmux" }

func (c *Cmux) target() []string {
	return []string{"--workspace", c.workspace, "--surface", c.surface}
}

func (c *Cmux) keyName(k Key) string {
	name, ok := cmuxKeys[k.Name]
	if !ok {
		name = k.Name
	}
	parts := make([]string, 0, len(k.Mods)+1)
	for _, m := range k.Mods {
		parts = append(parts, cmuxMods[m])
	}
	return strings.Join(append(parts, name), "+")
}

// SendKey presses a key with "cmux send-key".
func (c *Cmux) SendKey(k Key) error {
	args := append(append([]string{"send-key"}, c.target()...), "--", c.keyName(k))
	if out, err := cmuxRun(args...); err != nil {
		return fmt.Errorf("cmux send-key %s: %v: %s", k, err, out)
	}
	return nil
}

// Type sends text with "cmux send". cmux turns the two-character
// sequences \n, \r and \t into Enter and Tab.
func (c *Cmux) Type(text string) error {
	args := append(append([]string{"send"}, c.target()...), "--", text)
	if out, err := cmuxRun(args...); err != nil {
		return fmt.Errorf("cmux send: %v: %s", err, out)
	}
	return nil
}

// Paste puts text in a buffer of its own and pastes it with
// "cmux paste-buffer".
func (c *Cmux) Paste(text string) error {
	name := Title("clix-live-buffer")
	if out, err := cmuxRun("set-buffer", "--name", name, "--", text); err != nil {
		return fmt.Errorf("cmux set-buffer: %v: %s", err, out)
	}
	args := append([]string{"paste-buffer", "--name", name}, c.target()...)
	if out, err := cmuxRun(args...); err != nil {
		return fmt.Errorf("cmux paste-buffer: %v: %s", err, out)
	}
	return nil
}

// Describe says which cmux command sends a key, typed text or a paste.
func (c *Cmux) Describe(what string, k Key, text string) string {
	switch what {
	case "key":
		return "cmux send-key " + c.keyName(k)
	case "type":
		return fmt.Sprintf("cmux send %q", text)
	}
	return fmt.Sprintf("cmux set-buffer %q + paste-buffer", text)
}

// ReadScreen returns the text the surface shows.
func (c *Cmux) ReadScreen() (string, error) {
	args := append([]string{"read-screen"}, c.target()...)
	out, err := cmuxRun(args...)
	if err != nil {
		return "", fmt.Errorf("cmux read-screen: %v: %s", err, out)
	}
	return out, nil
}

// Resize widens (amount > 0) or narrows the workspace's pane, which sends
// SIGWINCH to the program in it.
func (c *Cmux) Resize(amount int) error {
	if c.pane == "" {
		return fmt.Errorf("cmux: no pane ref for %s", c.workspace)
	}
	dir := "-R"
	if amount < 0 {
		dir, amount = "-L", -amount
	}
	if out, err := cmuxRun("resize-pane", "--pane", c.pane, "--workspace", c.workspace, dir, "--amount", fmt.Sprint(amount)); err != nil {
		return fmt.Errorf("cmux resize-pane: %v: %s", err, out)
	}
	return nil
}

// Close closes the workspace.
func (c *Cmux) Close() {
	c.once.Do(func() {
		if out, err := cmuxRun("close-workspace", "--workspace", c.workspace); err != nil {
			c.t.Logf("cmux close-workspace %s: %v: %s", c.workspace, err, out)
		}
	})
}

func cmuxRun(args ...string) (string, error) {
	cmd := exec.Command(CmuxCLI, args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.String(), err
}
