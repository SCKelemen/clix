package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// reset removes only a regular file: never a directory (even an empty
// one), and never a special file at the end of a symlink.
func TestConfigE2E_ResetRemovesOnlyRegularFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	app, out, _ := e2eApp(t, dir)
	if _, err := tryRun(app, out, "config", "reset"); err == nil || !strings.Contains(err.Error(), "is not a regular file (a directory); not removing it") {
		t.Errorf("reset of a directory: %v", err)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("reset removed the directory (%v)", err)
	}

	linkDir := t.TempDir()
	target := filepath.Join(linkDir, "target-dir")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(linkDir, "config.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	app, out, _ = e2eApp(t, link)
	if _, err := tryRun(app, out, "config", "reset"); err == nil || !strings.Contains(err.Error(), "a directory") {
		t.Errorf("reset through a link to a directory: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("reset removed the link's target directory: %v", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("reset removed the link: %v", err)
	}

	if runtime.GOOS == "windows" {
		return
	}
	devLink := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.Symlink(os.DevNull, devLink); err != nil {
		t.Fatal(err)
	}
	app, out, _ = e2eApp(t, devLink)
	if _, err := tryRun(app, out, "config", "reset"); err == nil || !strings.Contains(err.Error(), "a device") {
		t.Errorf("reset through a link to %s: %v", os.DevNull, err)
	}
	if _, err := os.Stat(os.DevNull); err != nil {
		t.Fatalf("%s is gone: %v", os.DevNull, err)
	}
}
