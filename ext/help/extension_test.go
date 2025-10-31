package help

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"clix"
)

func TestHelpExtension(t *testing.T) {
	t.Run("help command exists with extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		// Add help extension
		app.AddExtension(Extension{})

		// Run to apply extensions
		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("ApplyExtensions failed: %v", err)
		}

		// Check that help command was added
		helpCmd := findSubcommand(root, "help")
		if helpCmd == nil {
			t.Fatal("help command was not added")
		}

		if helpCmd.Name != "help" {
			t.Errorf("expected command name 'help', got %q", helpCmd.Name)
		}
	})

	t.Run("help command does not exist without extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		// Don't add help extension
		app.AddDefaultCommands()

		// Check that help command was NOT added
		helpCmd := findSubcommand(root, "help")
		if helpCmd != nil {
			t.Fatal("help command should not exist without extension")
		}
	})

	t.Run("help command works with extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		root.Short = "Test command"
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		app.AddExtension(Extension{})

		// Run help command
		if err := app.Run(context.Background(), []string{"help"}); err != nil {
			t.Fatalf("help command failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "TEST") {
			t.Errorf("help output should contain 'TEST', got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "Test command") && !strings.Contains(outputStr, "test") {
			t.Errorf("help output should contain command info, got: %s", outputStr)
		}
	})

	t.Run("help command with subcommand works", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		app.Root = root

		subCmd := clix.NewCommand("subcommand")
		subCmd.Short = "A subcommand"
		root.AddCommand(subCmd)

		var output bytes.Buffer
		app.Out = &output

		app.AddExtension(Extension{})

		// Run help for subcommand
		if err := app.Run(context.Background(), []string{"help", "subcommand"}); err != nil {
			t.Fatalf("help subcommand failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "subcommand") {
			t.Errorf("help output should contain 'subcommand', got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "A subcommand") {
			t.Errorf("help output should contain 'A subcommand', got: %s", outputStr)
		}
	})

	t.Run("flag-based help works without extension", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		root.Short = "Test command"
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		// Don't add help extension
		// Flag-based help should still work
		if err := app.Run(context.Background(), []string{"--help"}); err != nil {
			t.Fatalf("flag-based help failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "TEST") {
			t.Errorf("help output should contain 'TEST', got: %s", outputStr)
		}
	})

	t.Run("flag-based help works with extension too", func(t *testing.T) {
		app := clix.NewApp("test")
		root := clix.NewCommand("test")
		root.Short = "Test command"
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		app.AddExtension(Extension{})

		// Flag-based help should still work even with extension
		if err := app.Run(context.Background(), []string{"--help"}); err != nil {
			t.Fatalf("flag-based help failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "TEST") {
			t.Errorf("help output should contain 'TEST', got: %s", outputStr)
		}
	})
}

