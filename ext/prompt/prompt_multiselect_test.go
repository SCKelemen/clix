package prompt

import (
	"bytes"
	"context"
	"github.com/SCKelemen/clix/v2"
	"strings"
	"testing"
)

func TestPromptMultiSelect(t *testing.T) {
	t.Run("multi-select with number input", func(t *testing.T) {
		in := bytes.NewBufferString("1\n\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select options", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
			{Label: "Option C", Value: "c"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}

		output := out.String()
		if !strings.Contains(output, "[ ]") || !strings.Contains(output, "[x]") {
			t.Errorf("output should contain checkboxes, got: %s", output)
		}
		if !strings.Contains(output, "Option A") {
			t.Errorf("output should contain Option A, got: %s", output)
		}
	})

	t.Run("multi-select with multiple numbers", func(t *testing.T) {
		in := bytes.NewBufferString("1,2\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select options", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
			{Label: "Option C", Value: "c"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		// Should contain both a and b
		if !strings.Contains(value, "a") || !strings.Contains(value, "b") {
			t.Fatalf("expected value to contain 'a' and 'b', got %q", value)
		}
	})

	t.Run("multi-select with space-separated numbers", func(t *testing.T) {
		in := bytes.NewBufferString("1 3\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select options", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
			{Label: "Option C", Value: "c"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		// Should contain both a and c
		if !strings.Contains(value, "a") || !strings.Contains(value, "c") {
			t.Fatalf("expected value to contain 'a' and 'c', got %q", value)
		}
	})

	t.Run("multi-select toggles selections", func(t *testing.T) {
		in := bytes.NewBufferString("1\n1\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select options", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		// Option 1 should be toggled off, so value should be empty
		if value != "" {
			t.Fatalf("expected empty value after toggle, got %q", value)
		}
	})

	t.Run("multi-select with default selections as values", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			clix.PromptRequest{
				Label:   "Select options",
				Default: "a,b",
				Theme:   clix.DefaultPromptTheme,
				Options: []clix.SelectOption{
					{Label: "Option A", Value: "a"},
					{Label: "Option B", Value: "b"},
					{Label: "Option C", Value: "c"},
				},
				MultiSelect: true,
			})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		// Should contain default selections
		if !strings.Contains(value, "a") || !strings.Contains(value, "b") {
			t.Fatalf("expected value to contain 'a' and 'b', got %q", value)
		}
	})

	t.Run("multi-select with default selections as indices", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			clix.PromptRequest{
				Label:   "Select options",
				Default: "1,2",
				Theme:   clix.DefaultPromptTheme,
				Options: []clix.SelectOption{
					{Label: "Option A", Value: "a"},
					{Label: "Option B", Value: "b"},
					{Label: "Option C", Value: "c"},
				},
				MultiSelect: true,
			})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		// Should contain default selections
		if !strings.Contains(value, "a") || !strings.Contains(value, "b") {
			t.Fatalf("expected value to contain 'a' and 'b', got %q", value)
		}
	})

	t.Run("multi-select requires at least one selection", func(t *testing.T) {
		in := bytes.NewBufferString("\n1\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select options", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		// Should eventually get value after selecting
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}

		output := out.String()
		if !strings.Contains(output, "Please select at least one option") {
			t.Errorf("output should show error for empty selection, got: %s", output)
		}
	})

	t.Run("multi-select accepts 'done' to finish", func(t *testing.T) {
		in := bytes.NewBufferString("1,2\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select options", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if !strings.Contains(value, "a") || !strings.Contains(value, "b") {
			t.Fatalf("expected value to contain 'a' and 'b', got %q", value)
		}
	})

	t.Run("multi-select accepts 'finish' to finish", func(t *testing.T) {
		in := bytes.NewBufferString("1\nfinish\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select options", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
	})
}
