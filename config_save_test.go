package clix_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	configext "github.com/SCKelemen/clix/v2/ext/config"
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

// Save wrote every value as the text Load kept, so a key it did not change
// changed type: 0x1F and 1_000 became strings an int flag rejects, 0755
// (493) became the string "0755" (755), and [a, "b,c"] became "a,b,c",
// three items.
func TestConfigSaveKeepsUnchangedKeys(t *testing.T) {
	for _, tc := range []struct {
		name, yaml, kind string
	}{
		{"octal int", "mode: 0755\n", "int"},
		{"hex int", "mode: 0x1F\n", "int"},
		{"underscore int", "mode: 1_000\n", "int"},
		{"quoted int", "mode: \"0755\"\n", "int"},
		{"exponent float", "mode: 1e3\n", "float"},
		{"list with a comma in an item", "tags: [a, \"b,c\"]\n", "slice"},
		{"list with a quoted item", "tags: ['\"x\"', y]\n", "slice"},
		{"block list", "tags:\n  - a\n  - b,c\n", "slice"},
		{"tagged string", "mode: !!str 0x1F\n", "string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read := func(path string) (string, error) {
				app, _ := newTestApp(t, "rt")
				app.ConfigPath = path
				var i int
				var f float64
				var s []string
				var str string
				var got string
				cmd := clix.NewCommand("go", clix.WithCommandRun(func(*clix.Context) error {
					got = map[string]string{"int": strconv.Itoa(i), "float": strconv.FormatFloat(f, 'g', -1, 64), "slice": strings.Join(s, "|"), "string": str}[tc.kind]
					return nil
				}))
				switch tc.kind {
				case "int":
					cmd.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "mode"}, Value: &i})
				case "float":
					cmd.Flags.Float64Var(clix.Float64VarOptions{FlagOptions: clix.FlagOptions{Name: "mode"}, Value: &f})
				case "slice":
					cmd.Flags.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "tags"}, Value: &s})
				case "string":
					cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "mode"}, Value: &str})
				}
				app.Root.AddCommand(cmd)
				err := app.Run(bg, []string{"go"})
				return got, err
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := read(path)
			if err != nil {
				t.Fatalf("before Save: %v", err)
			}
			// What `config set other x` does.
			m := clix.NewConfigManager("rt")
			if err := m.Load(path); err != nil {
				t.Fatal(err)
			}
			m.Set("other", "x")
			if err := m.Save(path); err != nil {
				t.Fatal(err)
			}
			body, _ := os.ReadFile(path)
			if after, err := read(path); err != nil || after != before {
				t.Errorf("after an unrelated Save the flag reads %q (%v), before it read %q; file:\n%s", after, err, before, body)
			}
			if !strings.Contains(string(body), strings.TrimSpace(strings.SplitN(tc.yaml, "\n", 2)[0])) {
				t.Errorf("Save rewrote the key:\n%s", body)
			}
		})
	}
}

// End to end: `app config set other x` made the next run of a command
// whose int flags read hand-written 0x1F and 0755 fail with exit status 2.
func TestConfigSetKeepsOtherKeys(t *testing.T) {
	newApp := func(path string) (*clix.App, *int, *int) {
		app, _ := newTestApp(t, "e2e")
		app.ConfigPath = path
		var mask, mode int
		cmd := clix.NewCommand("go", clix.WithCommandRun(noop))
		cmd.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "mask"}, Value: &mask})
		cmd.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "mode"}, Value: &mode})
		app.Root.AddCommand(cmd)
		app.AddExtension(configext.Extension{})
		return app, &mask, &mode
	}
	app, _ := newTestApp(t, "e2e")
	path := writeConfigFile(t, app, "mask: 0x1F\nmode: 0755\n")
	app, mask, mode := newApp(path)
	if err := app.Run(bg, []string{"go"}); err != nil || *mask != 31 || *mode != 493 {
		t.Fatalf("before: err=%v mask=%d mode=%d", err, *mask, *mode)
	}
	if err := app.Run(bg, []string{"config", "set", "other", "x"}); err != nil {
		t.Fatalf("config set: %v", err)
	}
	app, mask, mode = newApp(path) // a new process
	err := app.Run(bg, []string{"go"})
	body, _ := os.ReadFile(path)
	if err != nil || *mask != 31 || *mode != 493 {
		t.Errorf("after config set other x: err=%v (exit %d) mask=%d mode=%d; file:\n%s", err, clix.ExitCode(err), *mask, *mode, body)
	}
}

// A value given to Set reads back after Save exactly as before it: as the
// same text, and as the same number for an int flag. Numbers and booleans
// in canonical form are written as YAML numbers and booleans.
func TestConfigSaveTypesNewValues(t *testing.T) {
	values := map[string]string{
		"retries": "5", "ratio": "1.5", "debug": "true", "name": "hello",
		"mode": "0755", "mask": "0x1F", "version": "1.10", "big": "1_000",
		"empty": "", "null": "null", "plus": "+5", "spaced": " a ", "colon": "a: b",
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	m := clix.NewConfigManager("x")
	for k, v := range values {
		m.Set(k, v)
	}
	if err := m.Save(path); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	for _, line := range []string{"retries: 5\n", "ratio: 1.5\n", "debug: true\n", "name: hello\n", `mode: "0755"`, `version: "1.10"`, `empty: ""`} {
		if !strings.Contains(string(body), line) {
			t.Errorf("file lacks %q:\n%s", line, body)
		}
	}
	reloaded := clix.NewConfigManager("x")
	if err := reloaded.Load(path); err != nil {
		t.Fatal(err)
	}
	for k, want := range values {
		if got, ok := reloaded.Get(k); !ok || got != want {
			t.Errorf("%s = %q (%v), want %q", k, got, ok, want)
		}
		gotN, gotOK := reloaded.Int(k)
		wantN, wantOK := m.Int(k)
		if gotN != wantN || gotOK != wantOK {
			t.Errorf("Int(%s) = %d, %v after Save, %d, %v before", k, gotN, gotOK, wantN, wantOK)
		}
	}
}
