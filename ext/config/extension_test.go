package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix"
)

func TestConfigExtension(t *testing.T) {
	t.Run("config command exists with extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		// Add config extension
		app.AddExtension(Extension{})

		// Run to apply extensions
		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("ApplyExtensions failed: %v", err)
		}

		// Check that config command was added
		configCmd := findChild(root, "config")
		if configCmd == nil {
			t.Fatal("config command was not added")
		}

		if configCmd.Name != "config" {
			t.Errorf("expected command name 'config', got %q", configCmd.Name)
		}
	})

	t.Run("config command does not exist without extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		// Don't add config extension
		// No default commands needed - extensions are opt-in

		// Check that config command was NOT added
		configCmd := findChild(root, "config")
		if configCmd != nil {
			t.Fatal("config command should not exist without extension")
		}
	})

	t.Run("config command shows help", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		app.AddExtension(Extension{})

		// Run config command - it should show help
		if err := app.Run(context.Background(), []string{"config"}); err != nil {
			t.Fatalf("config command failed: %v", err)
		}

		outputStr := output.String()
		// Config command should show help by default
		if !strings.Contains(outputStr, "config") {
			t.Errorf("config output should contain 'config', got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "list") || !strings.Contains(outputStr, "get") || !strings.Contains(outputStr, "unset") || !strings.Contains(outputStr, "reset") {
			t.Errorf("config help should show subcommands, got: %s", outputStr)
		}
	})

	t.Run("config list command lists values", func(t *testing.T) {
		// Use a temporary home directory for config
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)
		defer os.Unsetenv("HOME")

		app := clix.NewApp("test")
		configDir := filepath.Join(tempHome, ".config", "test")
		os.MkdirAll(configDir, 0755)
		configPath := filepath.Join(configDir, "config.yaml")
		os.WriteFile(configPath, []byte("api.timeout: 30\nproject.default: dev\n"), 0644)

		root := clix.NewCommand("test")
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		app.AddExtension(Extension{})

		// Run config list command - it should list values
		if err := app.Run(context.Background(), []string{"config", "list"}); err != nil {
			t.Fatalf("config list command failed: %v", err)
		}

		outputStr := output.String()
		expected := "api:\n  timeout: 30\nproject:\n  default: dev"
		if strings.TrimSpace(outputStr) != expected {
			t.Fatalf("unexpected list output.\nexpected:\n%s\n\ngot:\n%s", expected, outputStr)
		}
	})

	t.Run("config get command works", func(t *testing.T) {
		// Use a temporary home directory for config
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)
		defer os.Unsetenv("HOME")

		app := clix.NewApp("test")
		configDir := filepath.Join(tempHome, ".config", "test")
		os.MkdirAll(configDir, 0755)
		configPath := filepath.Join(configDir, "config.yaml")
		os.WriteFile(configPath, []byte("project.default: dev\n"), 0644)

		root := clix.NewCommand("test")
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		app.AddExtension(Extension{})

		// Run config get command
		if err := app.Run(context.Background(), []string{"config", "get", "project.default"}); err != nil {
			t.Fatalf("config get command failed: %v", err)
		}

		outputStr := strings.TrimSpace(output.String())
		if outputStr != "dev" {
			t.Errorf("expected 'dev', got %q", outputStr)
		}
	})

	t.Run("config get missing key returns error", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)
		defer os.Unsetenv("HOME")

		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{})

		err := app.Run(context.Background(), []string{"config", "get", "does.not.exist"})
		if err == nil {
			t.Fatalf("expected error for missing key")
		}
		if !strings.Contains(err.Error(), `config key "does.not.exist" not found`) {
			t.Fatalf("unexpected error message: %v", err)
		}
	})

	t.Run("config set command works", func(t *testing.T) {
		// Use a temporary home directory for config
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)
		defer os.Unsetenv("HOME")

		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		app.AddExtension(Extension{})

		// Run config set command
		if err := app.Run(context.Background(), []string{"config", "set", "project.default", "staging"}); err != nil {
			t.Fatalf("config set command failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "project.default = staging") {
			t.Errorf("config set output should contain assignment, got: %s", outputStr)
		}

		// Verify value was saved
		if val, ok := app.Config.Get("project.default"); !ok || val != "staging" {
			t.Errorf("expected config to have project.default=staging, got %q, %v", val, ok)
		}
	})

	t.Run("config set enforces schema when registered", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)
		defer os.Unsetenv("HOME")

		app := clix.NewApp("test")
		app.Config.RegisterSchema(clix.ConfigSchema{
			Key:  "service.retries",
			Type: clix.ConfigInt,
		})

		root := clix.NewCommand("test")
		app.Root = root
		app.AddExtension(Extension{})

		// Non-integer should fail.
		err := app.Run(context.Background(), []string{"config", "set", "service.retries", "abc"})
		if err == nil {
			t.Fatalf("expected schema enforcement error")
		}

		// Valid integer should be accepted and canonicalised.
		if err := app.Run(context.Background(), []string{"config", "set", "service.retries", "08"}); err != nil {
			t.Fatalf("config set should accept canonical integer: %v", err)
		}
		if val, ok := app.Config.Get("service.retries"); !ok || val != "8" {
			t.Fatalf("expected stored canonical integer, got %q %v", val, ok)
		}
	})

	t.Run("config unset command removes value", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)
		defer os.Unsetenv("HOME")

		app := clix.NewApp("test")
		app.Config.Set("project.default", "beta")
		app.SaveConfig()

		root := clix.NewCommand("test")
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		app.AddExtension(Extension{})

		if err := app.Run(context.Background(), []string{"config", "unset", "project.default"}); err != nil {
			t.Fatalf("config unset failed: %v", err)
		}

		if _, ok := app.Config.Get("project.default"); ok {
			t.Fatalf("expected key to be removed")
		}

		if !strings.Contains(output.String(), "project.default") {
			t.Fatalf("unset output should mention key, got: %s", output.String())
		}
	})

	t.Run("config reset clears persisted config", func(t *testing.T) {
		tempHome := t.TempDir()
		t.Setenv("HOME", tempHome)
		defer os.Unsetenv("HOME")

		app := clix.NewApp("test")
		app.Config.Set("api.timeout", "30")
		app.SaveConfig()

		root := clix.NewCommand("test")
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		app.AddExtension(Extension{})

		if err := app.Run(context.Background(), []string{"config", "reset"}); err != nil {
			t.Fatalf("config reset failed: %v", err)
		}

		if output.String() == "" {
			t.Fatalf("reset should print confirmation")
		}

		configPath, _ := app.ConfigFile()
		if _, err := os.Stat(configPath); err == nil {
			t.Fatalf("config file should be removed")
		}
	})

	t.Run("config API still works without extension", func(t *testing.T) {
		// Config API should work even without the extension
		app := clix.NewApp("test")

		app.Config.Set("testkey", "testvalue")

		if val, ok := app.Config.Get("testkey"); !ok || val != "testvalue" {
			t.Errorf("expected config API to work, got %q, %v", val, ok)
		}
	})

	t.Run("multiple extensions can be added", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		// Add config extension
		app.AddExtension(Extension{})
		// Add a custom test extension using extensionFunc
		testExt := extensionFunc(func(a *clix.App) error {
			if a.Root != nil {
				cmd := clix.NewCommand("testhelp")
				cmd.Short = "Test help command"
				a.Root.AddCommand(cmd)
			}
			return nil
		})
		app.AddExtension(testExt)

		// Run to apply extensions
		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("ApplyExtensions failed: %v", err)
		}

		// Config command should exist
		if findChild(root, "config") == nil {
			t.Fatal("config command should exist")
		}
		// Test extension should exist
		if findChild(root, "testhelp") == nil {
			t.Fatal("testhelp command should exist")
		}
	})
}

// extensionFunc is a helper for testing
type extensionFunc func(*clix.App) error

func (f extensionFunc) Extend(app *clix.App) error {
	return f(app)
}
