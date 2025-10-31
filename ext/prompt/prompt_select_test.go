package prompt

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"clix"
)

func TestPromptSelect(t *testing.T) {
	t.Run("select prompt with number input", func(t *testing.T) {
		in := bytes.NewBufferString("1\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "What would you like to do?",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
				{Label: "Option C", Value: "c"},
			},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}

		output := out.String()
		if !strings.Contains(output, "What would you like to do?") {
			t.Errorf("output should contain label, got: %s", output)
		}
		if !strings.Contains(output, "Option A") {
			t.Errorf("output should contain Option A, got: %s", output)
		}
		if !strings.Contains(output, ">") {
			t.Errorf("output should show selection marker, got: %s", output)
		}
	})

	t.Run("select prompt with label match", func(t *testing.T) {
		in := bytes.NewBufferString("Option B\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Choose",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
			},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "b" {
			t.Fatalf("expected value 'b', got %q", value)
		}
	})

	t.Run("select prompt with partial match", func(t *testing.T) {
		in := bytes.NewBufferString("Option C\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Choose",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "Create a new repository", Value: "create"},
				{Label: "Option C", Value: "c"},
			},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "c" {
			t.Fatalf("expected value 'c', got %q", value)
		}
	})

	t.Run("select prompt with empty input uses default", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Choose",
			Default: "b",
			Theme:   clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
			},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "b" {
			t.Fatalf("expected default value 'b', got %q", value)
		}
	})

	t.Run("select prompt shows descriptions", func(t *testing.T) {
		in := bytes.NewBufferString("1\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		_, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "What would you like to do?",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{
					Label:       "Create a new repository on github.com from scratch",
					Value:       "create",
					Description: "Create a new repository",
				},
			},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}

		output := out.String()
		if !strings.Contains(output, "Create a new repository on github.com from scratch") {
			t.Errorf("output should contain label, got: %s", output)
		}
		if !strings.Contains(output, "Create a new repository") {
			t.Errorf("output should contain description, got: %s", output)
		}
	})
}

func TestPromptConfirm(t *testing.T) {
	t.Run("confirm prompt with yes default", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "y" {
			t.Fatalf("expected default 'y', got %q", value)
		}

		output := out.String()
		if !strings.Contains(output, "Continue?") {
			t.Errorf("output should contain label, got: %s", output)
		}
		if !strings.Contains(output, "(Y/n)") {
			t.Errorf("output should show Y/n default, got: %s", output)
		}
	})

	t.Run("confirm prompt with no default", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Default: "n",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "n" {
			t.Fatalf("expected default 'n', got %q", value)
		}

		output := out.String()
		if !strings.Contains(output, "(y/N)") {
			t.Errorf("output should show y/N default, got: %s", output)
		}
	})

	t.Run("confirm prompt accepts y", func(t *testing.T) {
		in := bytes.NewBufferString("y\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "y" {
			t.Fatalf("expected 'y', got %q", value)
		}
	})

	t.Run("confirm prompt accepts yes", func(t *testing.T) {
		in := bytes.NewBufferString("yes\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "y" {
			t.Fatalf("expected 'y', got %q", value)
		}
	})

	t.Run("confirm prompt accepts n", func(t *testing.T) {
		in := bytes.NewBufferString("n\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "n" {
			t.Fatalf("expected 'n', got %q", value)
		}
	})

	t.Run("confirm prompt rejects invalid input", func(t *testing.T) {
		in := bytes.NewBufferString("maybe\ny\n")
		out := &bytes.Buffer{}

		prompter := EnhancedTerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "y" {
			t.Fatalf("expected 'y' after retry, got %q", value)
		}

		output := out.String()
		if !strings.Contains(output, "please enter 'y' or 'n'") {
			t.Errorf("output should show error message, got: %s", output)
		}
	})
}
