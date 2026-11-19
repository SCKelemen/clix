package clix_test

import (
	"fmt"
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

// A relative link target was joined with the link's path lexically. With
// ~/.config a link into a dotfiles tree and app/config.yaml a link to
// ../../shared/app.yaml, Save wrote home/shared/app.yaml while Load read
// dotfiles/shared/app.yaml, and Save returned nil.
func TestConfigSaveFollowsLinksLikeLoad(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mkdir := func(p string) {
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	link := func(target, at string) {
		if err := os.Symlink(target, at); err != nil {
			t.Fatal(err)
		}
	}
	dot := filepath.Join(root, "dotfiles", "config")
	mkdir(filepath.Join(dot, "app"))
	mkdir(filepath.Join(root, "dotfiles", "shared"))
	mkdir(filepath.Join(root, "home", "shared")) // the lexical (wrong) target's directory
	real := filepath.Join(root, "dotfiles", "shared", "app.yaml")
	if err := os.WriteFile(real, []byte("k: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link(dot, filepath.Join(root, "home", ".config"))
	link("../../shared/app.yaml", filepath.Join(dot, "app", "config.yaml"))
	// A chain: l2 -> l1 -> the configured path.
	link(filepath.Join(root, "home", ".config", "app", "config.yaml"), filepath.Join(root, "l1"))
	link("l1", filepath.Join(root, "l2"))

	for i, p := range []string{
		filepath.Join(root, "home", ".config", "app", "config.yaml"),
		filepath.Join(root, "l2"),
	} {
		m := clix.NewConfigManager("x")
		value := fmt.Sprintf("new%d", i)
		m.Set("k", value)
		if err := m.Save(p); err != nil {
			t.Fatal(err)
		}
		for _, read := range []string{p, real} {
			got := clix.NewConfigManager("x")
			if err := got.Load(read); err != nil {
				t.Fatal(err)
			}
			if v, _ := got.Get("k"); v != value {
				t.Fatalf("saved through %s, Load(%s) = %q, want %q", p, read, v, value)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, "home", "shared", "app.yaml")); !os.IsNotExist(err) {
		t.Fatalf("Save wrote the lexical target: %v", err)
	}
	if info, err := os.Lstat(filepath.Join(dot, "app", "config.yaml")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link replaced: %v", err)
	}
}

// A link loop is an error, not a hang.
func TestConfigSaveLinkLoop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if err := os.Symlink(b, a); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, b); err != nil {
		t.Fatal(err)
	}
	m := clix.NewConfigManager("x")
	m.Set("k", "v")
	if err := m.Save(a); err == nil {
		t.Fatal("Save through a link loop succeeded")
	}
}
