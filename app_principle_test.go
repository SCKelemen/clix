package clix

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestPrincipleParentCommandsShowHelp(t *testing.T) {
	t.Run("parent command with children shows help when no child", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		childCmd := NewCommand("sub")
		childCmd.Short = "A child command"
		root.AddCommand(childCmd)

		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		// Running parent command with no child should show help
		if err := app.Run(context.Background(), []string{}); err != nil {
			t.Fatalf("expected help, got error: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "TEST") {
			t.Errorf("help should contain 'TEST', got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "sub") {
			t.Errorf("help should show child command, got: %s", outputStr)
		}
	})

	t.Run("invalid child shows parent help", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		childCmd := NewCommand("valid")
		childCmd.Short = "A valid child command"
		root.AddCommand(childCmd)

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

func TestPrincipleRequiredFlagsPrompt(t *testing.T) {
	t.Run("required flag without value prompts interactively", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		actionCmd := NewCommand("action")
		actionCmd.Short = "An actionable command"

		var name string
		actionCmd.Flags.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{
				Name:     "name",
				Usage:    "The name",
				Required: true,
				Prompt:   "Enter name",
			},
			Value: &name,
		})

		actionCmd.Run = func(ctx *Context) error {
			if name == "" {
				return fmt.Errorf("name flag required")
			}
			return nil
		}

		root.AddCommand(actionCmd)
		app.Root = root

		// Mock prompter that returns a value
		var prompted bool
		app.Prompter = prompterFunc(func(ctx context.Context, opts ...PromptOption) (string, error) {
			prompted = true
			return "test-value", nil
		})

		if err := app.Run(context.Background(), []string{"action"}); err != nil {
			t.Fatalf("command should succeed after prompting: %v", err)
		}

		if !prompted {
			t.Error("expected prompt to be triggered for missing required flag")
		}

		if name != "test-value" {
			t.Errorf("expected name to be 'test-value', got %q", name)
		}
	})

	t.Run("required flag with value doesn't prompt", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		actionCmd := NewCommand("action")
		actionCmd.Short = "An actionable command"

		var name string
		actionCmd.Flags.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{
				Name:     "name",
				Usage:    "The name",
				Required: true,
				Prompt:   "Enter name",
			},
			Value: &name,
		})

		actionCmd.Run = func(ctx *Context) error {
			if name == "" {
				return fmt.Errorf("name flag required")
			}
			return nil
		}

		root.AddCommand(actionCmd)
		app.Root = root

		var prompted bool
		app.Prompter = prompterFunc(func(ctx context.Context, opts ...PromptOption) (string, error) {
			prompted = true
			return "unexpected", nil
		})

		if err := app.Run(context.Background(), []string{"action", "--name", "test-value"}); err != nil {
			t.Fatalf("command should succeed: %v", err)
		}

		if prompted {
			t.Error("prompt should NOT be triggered when flag value is provided")
		}
	})

	t.Run("partial flags with missing required returns error", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		actionCmd := NewCommand("action")
		actionCmd.Short = "An actionable command"

		var name, email string
		actionCmd.Flags.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{
				Name:     "name",
				Usage:    "The name",
				Required: true,
			},
			Value: &name,
		})
		actionCmd.Flags.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{
				Name:     "email",
				Usage:    "The email",
				Required: true,
			},
			Value: &email,
		})

		actionCmd.Run = func(ctx *Context) error { return nil }

		root.AddCommand(actionCmd)
		app.Root = root

		// Only provide one of two required flags
		err := app.Run(context.Background(), []string{"action", "--name", "test"})
		if err == nil {
			t.Fatal("expected error for missing required flag")
		}
		if !strings.Contains(err.Error(), "missing required flags") {
			t.Errorf("expected 'missing required flags' error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "--email") {
			t.Errorf("error should mention --email, got: %v", err)
		}
	})

	t.Run("required flag with default is satisfied", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		actionCmd := NewCommand("action")
		actionCmd.Short = "An actionable command"

		var name string
		actionCmd.Flags.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{
				Name:     "name",
				Usage:    "The name",
				Required: true,
			},
			Default: "default-name",
			Value:   &name,
		})

		var executed bool
		actionCmd.Run = func(ctx *Context) error {
			executed = true
			return nil
		}

		root.AddCommand(actionCmd)
		app.Root = root

		// No flags passed, but default satisfies the requirement
		if err := app.Run(context.Background(), []string{"action"}); err != nil {
			t.Fatalf("command should succeed with default: %v", err)
		}
		if !executed {
			t.Error("expected command to execute")
		}
		if name != "default-name" {
			t.Errorf("expected name to be 'default-name', got %q", name)
		}
	})

	t.Run("required flag with env var is satisfied", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")
		app.configLoaded = true

		actionCmd := NewCommand("action")
		actionCmd.Short = "An actionable command"

		var name string
		actionCmd.Flags.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{
				Name:     "name",
				Usage:    "The name",
				Required: true,
				EnvVar:   "TEST_ACTION_NAME",
			},
			Value: &name,
		})

		var executed bool
		actionCmd.Run = func(ctx *Context) error {
			executed = true
			return nil
		}

		root.AddCommand(actionCmd)
		app.Root = root

		t.Setenv("TEST_ACTION_NAME", "env-name")

		// No flags passed, but env var satisfies the requirement
		if err := app.Run(context.Background(), []string{"action"}); err != nil {
			t.Fatalf("command should succeed with env var: %v", err)
		}
		if !executed {
			t.Error("expected command to execute")
		}
		if name != "env-name" {
			t.Errorf("expected name to be 'env-name', got %q", name)
		}
	})
}

// prompterFunc is a helper for testing
type prompterFunc func(context.Context, ...PromptOption) (string, error)

func (f prompterFunc) Prompt(ctx context.Context, opts ...PromptOption) (string, error) {
	return f(ctx, opts...)
}

func TestCommandWithChildrenBehavior(t *testing.T) {
	t.Run("group without Run handler shows help", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		group := NewGroup("group", "A group of commands",
			func() *Command {
				child := NewCommand("child")
				child.Short = "A child command"
				child.Run = func(ctx *Context) error {
					return nil
				}
				return child
			}(),
		)

		root.AddCommand(group)
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"group"}); err != nil {
			t.Fatalf("expected help, got error: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "A group of commands") {
			t.Errorf("help should contain group description, got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "child") {
			t.Errorf("help should show child command, got: %s", outputStr)
		}
		if !strings.Contains(outputStr, "COMMANDS") {
			t.Errorf("help should show commands section, got: %s", outputStr)
		}
	})

	t.Run("command with children and Run handler executes handler when no args", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		auth := NewCommand("auth")
		auth.Short = "Authentication commands"
		var handlerExecuted bool
		auth.Run = func(ctx *Context) error {
			handlerExecuted = true
			fmt.Fprintln(ctx.App.Out, "Auth handler executed!")
			return nil
		}

		login := NewCommand("login")
		login.Short = "Login command"
		login.Run = func(ctx *Context) error {
			fmt.Fprintln(ctx.App.Out, "Login executed!")
			return nil
		}
		auth.AddCommand(login)

		root.AddCommand(auth)
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"auth"}); err != nil {
			t.Fatalf("expected handler execution, got error: %v", err)
		}

		if !handlerExecuted {
			t.Error("auth Run handler should have been executed")
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "Auth handler executed!") {
			t.Errorf("output should contain handler message, got: %s", outputStr)
		}
	})

	t.Run("command with children routes to child when child name provided", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		auth := NewCommand("auth")
		auth.Run = func(ctx *Context) error {
			fmt.Fprintln(ctx.App.Out, "Auth handler executed!")
			return nil
		}

		var loginExecuted bool
		login := NewCommand("login")
		login.Run = func(ctx *Context) error {
			loginExecuted = true
			fmt.Fprintln(ctx.App.Out, "Login executed!")
			return nil
		}
		auth.AddCommand(login)

		root.AddCommand(auth)
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"auth", "login"}); err != nil {
			t.Fatalf("expected login execution, got error: %v", err)
		}

		if !loginExecuted {
			t.Error("login Run handler should have been executed")
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "Login executed!") {
			t.Errorf("output should contain login message, got: %s", outputStr)
		}
	})
}
