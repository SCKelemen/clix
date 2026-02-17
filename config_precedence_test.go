package clix

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigurationPrecedence(t *testing.T) {
	t.Run("Flag > Env > Config > Default", func(t *testing.T) {
		// Use XDG_CONFIG_HOME for reliable config isolation across environments.
		// ConfigDir() checks XDG_CONFIG_HOME first, so this avoids any
		// dependency on HOME or os.UserHomeDir() behavior.
		tempDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tempDir)

		app := NewApp("test")
		configDir := filepath.Join(tempDir, "test")
		os.MkdirAll(configDir, 0755)
		configPath := filepath.Join(configDir, "config.yaml")

		// Write config file with value
		os.WriteFile(configPath, []byte("value: from-config\n"), 0644)

		var value string
		root := NewCommand("test")
		root.Flags.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "value"},
			Default:     "default-value",
			Value:       &value,
		})
		root.Run = func(ctx *Context) error {
			return nil
		}
		app.Root = root

		// Test 1: Flag should override everything
		value = ""
		t.Setenv("TEST_VALUE", "from-env")

		app.configLoaded = false // Force reload
		if err := app.Run(context.Background(), []string{"--value", "from-flag"}); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if value != "from-flag" {
			t.Errorf("expected 'from-flag', got %q", value)
		}

		// Test 2: Env should override config and default (when no flag)
		value = ""
		app.configLoaded = false // Force reload
		if err := app.Run(context.Background(), []string{}); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if value != "from-env" {
			t.Errorf("expected 'from-env', got %q", value)
		}

		// Test 3: Config should override default (when no flag or env)
		value = ""
		os.Unsetenv("TEST_VALUE")
		app.configLoaded = false // Force reload
		if err := app.Run(context.Background(), []string{}); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if value != "from-config" {
			t.Errorf("expected 'from-config', got %q", value)
		}

		// Test 4: Default should be used when nothing else is set
		value = ""
		app.Config.Reset()
		app.SaveConfig()         // Clear config file
		app.configLoaded = false // Force reload
		if err := app.Run(context.Background(), []string{}); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if value != "default-value" {
			t.Errorf("expected 'default-value', got %q", value)
		}
	})
}
