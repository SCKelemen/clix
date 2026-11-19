package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Saving through a symlinked config path (a dotfiles repository) writes the
// target and keeps the link.
func TestConfigE2E_SymlinkedFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "real.yaml")
	link := filepath.Join(dir, "link.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("a: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	app, out, _ := e2eApp(t, link)
	run(t, app, out, "config", "set", "b", "2")
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the symlink was replaced (%v)", err)
	}
	if data, _ := os.ReadFile(target); string(data) != "a: \"1\"\nb: \"2\"\n" {
		t.Errorf("symlink target after set:\n%s", data)
	}
	run(t, app, out, "config", "reset")
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("config reset removed the symlink: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("config reset kept the target (%v)", err)
	}
}
