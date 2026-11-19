package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// "config unset" of a key that does not exist created the config file (and
// its directory).
func TestConfigE2E_UnsetMissingKeyWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.yaml")
	app, out, _ := e2eApp(t, path)
	if got := run(t, app, out, "config", "unset", "nope"); got != "nope removed (no value stored)\n" {
		t.Errorf("config unset nope printed %q", got)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Errorf("unset of a missing key created %s (%v)", filepath.Dir(path), err)
	}

	// An existing file is not rewritten either.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	const original = "# hand-written\nkeep: me\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	run(t, app, out, "config", "unset", "nope")
	data, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(data) != original || !info.ModTime().Equal(old) {
		t.Errorf("unset of a missing key rewrote the file:\n%s", data)
	}
}

func TestConfigE2E_UnsetRemovesValue(t *testing.T) {
	app, out, path := e2eApp(t, "")
	run(t, app, out, "config", "set", "project.default", "p1")
	run(t, app, out, "config", "set", "project.region", "eu")
	if got := run(t, app, out, "config", "unset", "project.default"); got != "project.default removed\n" {
		t.Errorf("config unset printed %q", got)
	}
	if got := run(t, app, out, "config", "list"); got != "project:\n  region: eu\n" {
		t.Errorf("config list after unset:\n%s", got)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "project.region: eu\n" {
		t.Errorf("file after unset:\n%s", data)
	}
	if _, err := tryRun(app, out, "config", "get", "project.default"); err == nil {
		t.Error("config get of an unset key succeeded")
	}
}
