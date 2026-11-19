package clix

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunDoesNotCreateConfigDir(t *testing.T) {
	app, out, _ := newIsolatedApp(t, "moln")
	if err := app.Run(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out.String(), "USAGE") {
		t.Fatal("no help printed")
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "moln")); !os.IsNotExist(err) {
		t.Fatalf("Run created the config directory (stat err=%v)", err)
	}
}

func TestUnwritableConfigHomeDoesNotBreakRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not enforced on Windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Error(err)
		}
	})
	if f, err := os.Create(filepath.Join(dir, "probe")); err == nil {
		_ = f.Close()
		t.Skip("directory is still writable (running as root?)")
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	app := NewApp("moln", WithAppOut(&strings.Builder{}))
	ran := false
	app.Root.AddCommand(NewCommand("ls", WithCommandRun(func(*Context) error { ran = true; return nil })))
	if err := app.Run(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("--help: %v", err)
	}
	if err := app.Run(context.Background(), []string{"ls"}); err != nil || !ran {
		t.Fatalf("ls: ran=%v err=%v", ran, err)
	}
}

func TestSaveConfigUsesPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not enforced on Windows")
	}
	app, _, _ := newIsolatedApp(t, "moln")
	app.Config.Set("token", "secret")
	if err := app.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	path, err := app.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config file mode = %o, want 600", perm)
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Fatalf("config dir mode = %o, want 700", perm)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("leftover temporary files: %v", entries)
	}

	loaded := NewConfigManager("moln")
	if err := loaded.Load(path); err != nil {
		t.Fatal(err)
	}
	if v, _ := loaded.Get("token"); v != "secret" {
		t.Fatalf("round trip lost the value: %q", v)
	}
}

func TestConfigPathOverrides(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	custom := filepath.Join(t.TempDir(), "custom.yaml")
	if err := os.WriteFile(custom, []byte("project: from-config-path\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app.ConfigPath = custom
	if got, _ := app.ConfigFile(); got != custom {
		t.Fatalf("ConfigFile() = %q, want %q", got, custom)
	}

	var project string
	cmd := NewCommand("ls", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "project"}, Value: &project})
	app.Root.AddCommand(cmd)
	if err := app.Run(context.Background(), []string{"ls"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if project != "from-config-path" {
		t.Fatalf("project = %q, want the value from App.ConfigPath", project)
	}

	// <PREFIX>_CONFIG belongs to an app's own --config flag, not to clix.
	t.Setenv("MOLN_CONFIG", filepath.Join(t.TempDir(), "env.yaml"))
	if got, _ := app.ConfigFile(); got != custom {
		t.Fatalf("ConfigFile() = %q, want App.ConfigPath (%q); MOLN_CONFIG must not move it", got, custom)
	}
}

func TestAppConfigFlagKeepsItsEnvVar(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "app")
	settings := filepath.Join(t.TempDir(), "settings.toml")
	if err := os.WriteFile(settings, []byte("[server]\nport = 8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_CONFIG", settings)

	var config string
	app.Flags().StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "config"}, Value: &config})
	app.Root.AddCommand(NewCommand("serve", WithCommandRun(func(*Context) error { return nil })))
	if err := app.Run(context.Background(), []string{"serve"}); err != nil {
		t.Fatalf("Run error: %v (APP_CONFIG is the app's --config flag, not a clix config file)", err)
	}
	if config != settings {
		t.Fatalf("--config = %q, want APP_CONFIG (%q)", config, settings)
	}
}

func TestConfigFileCreatesItsDirectory(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "fresh")
	path, err := app.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("k: v\n"), 0o600); err != nil {
		t.Fatalf("writing ConfigFile's path failed: %v", err)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("config directory mode = %o, want 0700", perm)
	}
}
