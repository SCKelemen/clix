package autocomplete

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix"
)

func TestAutocompleteExtension(t *testing.T) {
	t.Run("autocomplete command exists with extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{})

		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		autocompleteCmd := findChildInTest(root, "autocomplete")
		if autocompleteCmd == nil {
			t.Fatal("autocomplete command should exist with extension")
		}
	})

	t.Run("autocomplete command does not exist without extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		// Don't add extension

		autocompleteCmd := findChildInTest(root, "autocomplete")
		if autocompleteCmd != nil {
			t.Fatal("autocomplete command should not exist without extension")
		}
	})

	t.Run("autocomplete command works", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		sub := clix.NewCommand("sub")
		sub.Short = "A subcommand"
		root.AddCommand(sub)
		app.Root = root

		app.AddExtension(Extension{})

		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		var output bytes.Buffer
		app.Out = &output

		// Test bash completion
		if err := app.Run(context.Background(), []string{"autocomplete", "bash"}); err != nil {
			t.Fatalf("autocomplete command failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "_test_completions") {
			t.Errorf("bash completion should contain '_test_completions', got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "sub") {
			t.Errorf("bash completion should contain 'sub', got: %s", outputStr)
		}
	})

	t.Run("autocomplete command shows help by default", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{})

		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"autocomplete"}); err != nil {
			t.Fatalf("autocomplete should show help: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "autocomplete") {
			t.Errorf("help should contain 'autocomplete', got: %s", outputStr)
		}
	})

	t.Run("autocomplete supports all shells", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{})

		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		shells := []string{"bash", "zsh", "fish"}
		for _, shell := range shells {
			var output bytes.Buffer
			app.Out = &output

			if err := app.Run(context.Background(), []string{"autocomplete", shell}); err != nil {
				t.Errorf("autocomplete for %s failed: %v", shell, err)
			}

			outputStr := output.String()
			if outputStr == "" {
				t.Errorf("autocomplete for %s produced empty output", shell)
			}
		}
	})

	t.Run("autocomplete rejects unsupported shell", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		app.AddExtension(Extension{})

		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		err := app.Run(context.Background(), []string{"autocomplete", "powershell"})
		if err == nil {
			t.Fatal("autocomplete should fail for unsupported shell")
		}
		if !strings.Contains(err.Error(), "unsupported shell") {
			t.Errorf("error should mention 'unsupported shell', got: %v", err)
		}
	})
}

func findChildInTest(cmd *clix.Command, name string) *clix.Command {
	for _, child := range cmd.Children {
		if child.Name == name {
			return child
		}
		if found := findChildInTest(child, name); found != nil {
			return found
		}
	}
	return nil
}
