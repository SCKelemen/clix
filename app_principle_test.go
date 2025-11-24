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
		app.Prompter = prompterFunc(func(ctx context.Context, opts ...PromptOption) (string, error) {
			prompted = true
			// Convert options to see what was requested
			cfg := &PromptConfig{}
			for _, opt := range opts {
				opt.Apply(cfg)
			}
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
		app.Prompter = prompterFunc(func(ctx context.Context, opts ...PromptOption) (string, error) {
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
type prompterFunc func(context.Context, ...PromptOption) (string, error)

func (f prompterFunc) Prompt(ctx context.Context, opts ...PromptOption) (string, error) {
	return f(ctx, opts...)
}

func TestCommandWithChildrenBehavior(t *testing.T) {
	t.Run("group without Run handler shows help", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		// Create a group (no Run handler, has children)
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

		// Running group without args should show help
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

		// Create a command with both Run handler AND children
		auth := NewCommand("auth")
		auth.Short = "Authentication commands"
		var handlerExecuted bool
		auth.Run = func(ctx *Context) error {
			handlerExecuted = true
			fmt.Fprintln(ctx.App.Out, "Auth handler executed!")
			return nil
		}

		// Add a child command
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

		// Running auth without args should execute the Run handler
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
		if strings.Contains(outputStr, "COMMANDS") || strings.Contains(outputStr, "GROUPS") {
			t.Errorf("should not show help, got: %s", outputStr)
		}
	})

	t.Run("command with children routes to child when child name provided", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		// Create a command with both Run handler AND children
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

		// Running auth login should route to login child, not execute auth handler
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
		if strings.Contains(outputStr, "Auth handler executed!") {
			t.Errorf("auth handler should not have been executed, got: %s", outputStr)
		}
	})

	t.Run("command with children executes handler when args provided", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")

		var handlerExecuted bool
		var receivedArgs []string
		auth := NewCommand("auth")
		auth.Run = func(ctx *Context) error {
			handlerExecuted = true
			receivedArgs = ctx.Args
			fmt.Fprintf(ctx.App.Out, "Auth handler executed with args: %v\n", ctx.Args)
			return nil
		}

		login := NewCommand("login")
		login.Run = func(ctx *Context) error {
			return nil
		}
		auth.AddCommand(login)

		root.AddCommand(auth)
		app.Root = root

		var output bytes.Buffer
		app.Out = &output

		// Running auth with args should execute handler with those args
		if err := app.Run(context.Background(), []string{"auth", "arg1", "arg2"}); err != nil {
			t.Fatalf("expected handler execution, got error: %v", err)
		}

		if !handlerExecuted {
			t.Error("auth Run handler should have been executed")
		}

		if len(receivedArgs) != 2 || receivedArgs[0] != "arg1" || receivedArgs[1] != "arg2" {
			t.Errorf("handler should have received args [arg1 arg2], got: %v", receivedArgs)
		}
	})
}
