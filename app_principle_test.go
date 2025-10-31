package clix

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestPrincipleParentCommandsShowHelp(t *testing.T) {
	t.Run("parent command with subcommands shows help when no subcommand", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")
		
		subCmd := NewCommand("sub")
		subCmd.Short = "A subcommand"
		root.AddCommand(subCmd)
		
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		// Running parent command with no subcommand should show help
		if err := app.Run(context.Background(), []string{}); err != nil {
			t.Fatalf("expected help, got error: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "TEST") {
			t.Errorf("help should contain 'TEST', got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "sub") {
			t.Errorf("help should show subcommand, got: %s", outputStr)
		}
	})

	t.Run("invalid subcommand shows parent help", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")
		
		subCmd := NewCommand("valid")
		subCmd.Short = "A valid subcommand"
		root.AddCommand(subCmd)
		
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		// Running with invalid subcommand should show parent help or error
		err := app.Run(context.Background(), []string{"invalid"})
		if err != nil {
			// Error is acceptable for invalid command
			if !strings.Contains(err.Error(), "unknown command") {
				t.Errorf("expected 'unknown command' error, got: %v", err)
			}
		} else {
			// If no error, should show help
			outputStr := output.String()
			if !strings.Contains(outputStr, "test") {
				t.Errorf("should show help for parent, got: %s", outputStr)
			}
		}
	})
}

func TestPrincipleActionableCommandsPrompt(t *testing.T) {
	t.Run("actionable command without args prompts", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")
		
		actionCmd := NewCommand("action")
		actionCmd.Short = "An actionable command"
		actionCmd.Arguments = []*Argument{
			{Name: "name", Required: true, Prompt: "Enter name"},
		}
		actionCmd.Run = func(ctx *Context) error {
			if len(ctx.Args) == 0 || ctx.Args[0] == "" {
				return fmt.Errorf("name argument required")
			}
			return nil
		}
		
		root.AddCommand(actionCmd)
		app.Root = root

		// Mock prompter that returns a value
		var prompted bool
		app.Prompter = prompterFunc(func(ctx context.Context, req PromptRequest) (string, error) {
			prompted = true
			return "test-value", nil
		})

		if err := app.Run(context.Background(), []string{"action"}); err != nil {
			t.Fatalf("command should succeed after prompting: %v", err)
		}

		if !prompted {
			t.Error("expected prompt to be triggered for missing required argument")
		}
	})

	t.Run("actionable command with positional args doesn't prompt", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")
		
		actionCmd := NewCommand("action")
		actionCmd.Short = "An actionable command"
		actionCmd.Arguments = []*Argument{
			{Name: "name", Required: true, Prompt: "Enter name"},
		}
		actionCmd.Run = func(ctx *Context) error {
			if len(ctx.Args) == 0 || ctx.Args[0] == "" {
				return fmt.Errorf("name argument required")
			}
			return nil
		}
		
		root.AddCommand(actionCmd)
		app.Root = root

		var prompted bool
		app.Prompter = prompterFunc(func(ctx context.Context, req PromptRequest) (string, error) {
			prompted = true
			return "unexpected", nil
		})

		// Use positional argument
		if err := app.Run(context.Background(), []string{"action", "test-value"}); err != nil {
			t.Fatalf("command should succeed: %v", err)
		}

		if prompted {
			t.Error("prompt should NOT be triggered when positional argument is provided")
		}
	})
}

// prompterFunc is a helper for testing
type prompterFunc func(context.Context, PromptRequest) (string, error)

func (f prompterFunc) Prompt(ctx context.Context, req PromptRequest) (string, error) {
	return f(ctx, req)
}

