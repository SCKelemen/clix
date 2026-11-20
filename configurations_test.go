package clix_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func newConfigurationsApp(t *testing.T) (*clix.App, string) {
	t.Helper()
	app, _ := newTestApp(t, "myapp")
	t.Setenv("MYAPP_CONFIGURATION", "")
	dir := filepath.Join(t.TempDir(), "cfg")
	app.ConfigPath = filepath.Join(dir, "settings.yaml")
	app.EnableConfigurations()
	app.Root.AddCommand(clix.NewCommand("noop", clix.WithCommandRun(noop)))
	return app, dir
}

func TestConfigurationsAPI(t *testing.T) {
	app, dir := newConfigurationsApp(t)
	if name, src, err := app.Configuration(); name != "default" || src != clix.SourceDefault || err != nil {
		t.Fatalf("Configuration() = %q, %v, %v", name, src, err)
	}
	if file, err := app.ConfigurationFile("default"); err != nil || file != app.ConfigPath {
		t.Fatalf("default file = %q, %v", file, err)
	}
	if file, err := app.ConfigurationFile("work"); err != nil || file != filepath.Join(dir, "configurations", "config_work.yaml") {
		t.Fatalf("work file = %q, %v", file, err)
	}
	for _, bad := range []string{"", "Work", "1x", "-x", "x_y", "x.y", "../x", "x/y"} {
		if _, err := app.ConfigurationFile(bad); err == nil {
			t.Errorf("ConfigurationFile(%q) accepted", bad)
		}
	}
	if err := app.ActivateConfiguration("work"); !errors.Is(err, clix.ErrConfigurationNotFound) {
		t.Fatalf("activate a missing configuration: %v", err)
	}
	if err := app.CreateConfiguration("work"); err != nil {
		t.Fatal(err)
	}
	if err := app.CreateConfiguration("work"); err == nil {
		t.Fatal("created work twice")
	}
	if err := app.CreateConfiguration("default"); err == nil {
		t.Fatal("created default")
	}
	// Stray files are not configurations.
	for _, f := range []string{"notes.txt", "config_Bad.yaml", "config_x.yml", "config_default.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, "configurations", f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "configurations", "config_dir.yaml"), 0o700); err != nil {
		t.Fatal(err)
	}
	if names, err := app.Configurations(); err != nil || !reflect.DeepEqual(names, []string{"default", "work"}) {
		t.Fatalf("Configurations() = %q, %v", names, err)
	}
	if err := app.ActivateConfiguration("work"); err != nil {
		t.Fatal(err)
	}
	if name, src, err := app.Configuration(); name != "work" || src != clix.SourceConfigFile || err != nil {
		t.Fatalf("Configuration() = %q, %v, %v", name, src, err)
	}
	if file, err := app.ConfigFile(); err != nil || file != filepath.Join(dir, "configurations", "config_work.yaml") {
		t.Fatalf("ConfigFile() = %q, %v", file, err)
	}
	if d, err := app.ConfigDir(); err != nil || d != dir {
		t.Fatalf("ConfigDir() = %q, %v", d, err)
	}
	app.Config.Set("k", "v")
	if err := app.SaveConfig(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(app.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("SaveConfig wrote the default file: %v", err)
	}
	t.Setenv("MYAPP_CONFIGURATION", "default")
	if name, src, _ := app.Configuration(); name != "default" || src != clix.SourceEnvVar {
		t.Fatalf("with the environment: %q, %v", name, src)
	}
	if err := app.Run(bg, []string{"--configuration", "work", "noop"}); err != nil {
		t.Fatal(err)
	}
	if name, src, _ := app.Configuration(); name != "work" || src != clix.SourceAppFlag {
		t.Fatalf("with --configuration: %q, %v", name, src)
	}
	if v, ok := app.Config.Get("k"); !ok || v != "v" {
		t.Fatalf("work's values not loaded: %q", v)
	}
	if err := app.Run(bg, []string{"noop"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := app.Config.Get("k"); ok {
		t.Fatal("work's values stayed loaded for default")
	}
	if err := app.Validate(); err != nil {
		t.Fatal(err)
	}
}

// Readers of active_config see one name or the other, never a partial
// write, while it is switched over and over.
func TestActivateConfigurationIsAtomic(t *testing.T) {
	app, dir := newConfigurationsApp(t)
	for _, n := range []string{"a", "b"} {
		if err := app.CreateConfiguration(n); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.ActivateConfiguration("a"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if err := app.ActivateConfiguration([]string{"a", "b"}[i%2]); err != nil {
				t.Error(err)
				return
			}
		}
		close(stop)
	}()
	path := filepath.Join(dir, "active_config")
	for done := false; !done; {
		select {
		case <-stop:
			done = true
		default:
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if s := string(data); s != "a\n" && s != "b\n" {
			t.Fatalf("active_config read as %q", s)
		}
	}
	wg.Wait()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "active_config" && e.Name() != "configurations" {
			t.Errorf("left behind: %s", e.Name())
		}
	}
}

func TestNestedRunRestoresNamedConfiguration(t *testing.T) {
	app, _ := newConfigurationsApp(t)
	for _, name := range []string{"prod", "dev"} {
		if err := app.CreateConfiguration(name); err != nil {
			t.Fatal(err)
		}
		path, err := app.ConfigurationFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("account: "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	app.Root.AddCommand(clix.NewCommand("inner", clix.WithCommandRun(func(ctx *clix.Context) error {
		if got, _ := ctx.String("account"); got != "dev" {
			t.Fatalf("inner account = %q", got)
		}
		return nil
	})))
	app.Root.AddCommand(clix.NewCommand("outer", clix.WithCommandRun(func(ctx *clix.Context) error {
		app.Config.Set("account", "edited")
		if err := app.Run(ctx, []string{"--configuration", "dev", "inner"}); err != nil {
			return err
		}
		if err := app.Run(ctx, []string{"--configuration", "missing", "inner"}); !errors.Is(err, clix.ErrConfigurationNotFound) {
			t.Fatalf("missing configuration: %v", err)
		}
		if name, _, err := app.Configuration(); err != nil || name != "prod" {
			t.Fatalf("outer config = %q, %v", name, err)
		}
		if got, _ := ctx.String("account"); got != "edited" {
			t.Fatalf("outer account = %q", got)
		}
		return app.SaveConfig()
	})))
	if err := app.Run(t.Context(), []string{"--configuration", "prod", "outer"}); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"prod": "edited", "dev": "dev"} {
		path, err := app.ConfigurationFile(name)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "account: "+want+"\n" {
			t.Fatalf("%s holds %q, %v", name, data, err)
		}
	}
	if _, err := os.Stat(app.ConfigPath); !os.IsNotExist(err) {
		t.Fatalf("default config: %v", err)
	}
}
