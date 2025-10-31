package prompt

import (
	"bytes"
	"clix"
	"context"
	"strings"
	"testing"
)

// TestFunctionalOptions tests that functional options work correctly
// with TerminalPrompter for advanced prompts.
func TestFunctionalOptions(t *testing.T) {
	t.Run("functional options for select prompt", func(t *testing.T) {
		in := bytes.NewBufferString("1\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			clix.WithLabel("Choose"),
			clix.WithTheme(clix.DefaultPromptTheme),
			Select([]clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
			}),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
	})

	t.Run("functional options for multi-select prompt", func(t *testing.T) {
		in := bytes.NewBufferString("1,2\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			clix.WithLabel("Select"),
			clix.WithTheme(clix.DefaultPromptTheme),
			MultiSelect([]clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
				{Label: "Option C", Value: "c"},
			}),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if !strings.Contains(value, "a") || !strings.Contains(value, "b") {
			t.Fatalf("expected value to contain 'a' and 'b', got %q", value)
		}
	})

	t.Run("functional options for confirm prompt", func(t *testing.T) {
		in := bytes.NewBufferString("y\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			clix.WithLabel("Continue?"),
			clix.WithTheme(clix.DefaultPromptTheme),
			Confirm(),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "y" {
			t.Fatalf("expected value 'y', got %q", value)
		}
	})

	t.Run("functional options with continue text", func(t *testing.T) {
		in := bytes.NewBufferString("1\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			clix.WithLabel("Select"),
			clix.WithTheme(clix.DefaultPromptTheme),
			MultiSelect([]clix.SelectOption{
				{Label: "Option A", Value: "a"},
			}),
			WithContinueText("Finish"),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
	})

	t.Run("functional options can be mixed with struct", func(t *testing.T) {
		in := bytes.NewBufferString("1\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		// Mix: struct for Label/Theme, functional for Select
		value, err := prompter.Prompt(context.Background(),
			clix.PromptRequest{
				Label: "Choose",
				Theme: clix.DefaultPromptTheme,
			},
			Select([]clix.SelectOption{
				{Label: "Option A", Value: "a"},
			}),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
	})

	t.Run("functional options override struct values", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		// Struct sets label "Old", functional option overrides to "New"
		value, err := prompter.Prompt(context.Background(),
			clix.PromptRequest{
				Label: "Old",
				Theme: clix.DefaultPromptTheme,
			},
			clix.WithLabel("New"),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		output := out.String()
		if !strings.Contains(output, "New") {
			t.Fatalf("expected output to contain 'New' (from functional option), got: %s", output)
		}
		if strings.Contains(output, "Old") {
			t.Errorf("output should not contain 'Old' (overridden by functional option), got: %s", output)
		}
		_ = value // Value is empty since input was empty and no default
	})
}
