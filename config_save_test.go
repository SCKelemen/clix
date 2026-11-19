package clix_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// Save used to rename its temporary file over the path, which replaced a
// symbolic link with a regular file.
func TestConfigSaveWritesThroughSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	for _, relative := range []bool{false, true} {
		dir := t.TempDir()
		real := filepath.Join(dir, "dotfiles", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(real), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(real, []byte("old: 1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "config.yaml")
		target := real
		if relative {
			target = filepath.Join("dotfiles", "config.yaml")
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}

		m := clix.NewConfigManager("x")
		m.Set("project", "p1")
		if err := m.Save(link); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(link)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("relative=%v: %s is no longer a symbolic link (%v)", relative, link, err)
		}
		reloaded := clix.NewConfigManager("x")
		if err := reloaded.Load(real); err != nil {
			t.Fatal(err)
		}
		if v, _ := reloaded.Get("project"); v != "p1" {
			t.Fatalf("relative=%v: target holds project=%q", relative, v)
		}
		if info, _ := os.Stat(real); info.Mode().Perm() != 0o600 {
			t.Fatalf("target mode = %v, want 0600", info.Mode().Perm())
		}
		entries, _ := os.ReadDir(filepath.Dir(real))
		if len(entries) != 1 {
			t.Fatalf("temporary files left behind: %v", entries)
		}
	}
}

// A dangling link is followed, and the file it names is created.
func TestConfigSaveCreatesSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "elsewhere", "config.yaml")
	link := filepath.Join(dir, "config.yaml")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	m := clix.NewConfigManager("x")
	m.Set("a", "b")
	if err := m.Save(link); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link replaced: %v", err)
	}
	if info, err := os.Stat(real); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("target: %v %v", info, err)
	}
}
