package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A malformed config file made every command fail before it ran,
// including "config reset", the command meant to fix it.
func TestConfigE2E_ResetRepairsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("project: [unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, out, _ := e2eApp(t, path)

	// Other commands still report the broken file, and leave it alone.
	for _, args := range [][]string{{"config", "list"}, {"config", "set", "a", "1"}, {"config", "unset", "a"}} {
		if _, err := tryRun(app, out, args...); err == nil || !strings.Contains(err.Error(), "YAML") {
			t.Errorf("%q on a malformed file: err %v, want the parse error", args, err)
		}
	}
	if data, _ := os.ReadFile(path); string(data) != "project: [unterminated\n" {
		t.Fatalf("a failing command changed the file:\n%s", data)
	}

	if got := run(t, app, out, "config", "reset"); got != "Configuration cleared\n" {
		t.Errorf("config reset printed %q", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("config reset left the file (%v)", err)
	}
	// The next run loads the (now missing) file again.
	if got := run(t, app, out, "config", "list"); got != "" {
		t.Errorf("config list after reset printed %q", got)
	}
	run(t, app, out, "config", "set", "a", "1")
	if got := run(t, app, out, "config", "get", "a"); got != "1\n" {
		t.Errorf("config get after reset and set printed %q", got)
	}
}

func TestConfigE2E_ResetClearsEverything(t *testing.T) {
	app, out, path := e2eApp(t, "")
	run(t, app, out, "config", "set", "a", "1")
	run(t, app, out, "config", "set", "b.c", "2")
	run(t, app, out, "config", "reset")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("config reset left the file (%v)", err)
	}
	if got := run(t, app, out, "config", "list"); got != "" {
		t.Errorf("config list after reset printed %q", got)
	}
	// Reset without a file is fine.
	run(t, app, out, "config", "reset")
}
