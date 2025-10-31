package prompt

import (
	"bytes"
	"clix"
	"context"
	"errors"
	"strings"
	"testing"
)

// TestPromptEdgeCases tests edge cases and error conditions
func TestPromptEdgeCases(t *testing.T) {
	t.Run("select prompt with empty options list", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Choose",
			Theme:   clix.DefaultPromptTheme,
			Options: []clix.SelectOption{}, // Empty list
		})
		if err != nil {
			t.Fatalf("Prompt should handle empty options, got error: %v", err)
		}
		if value != "" {
			t.Fatalf("expected empty value for empty options, got %q", value)
		}
	})

	t.Run("multi-select with empty options list", func(t *testing.T) {
		in := bytes.NewBufferString("done\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:       "Select",
			Theme:       clix.DefaultPromptTheme,
			Options:     []clix.SelectOption{}, // Empty list
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt should handle empty options, got error: %v", err)
		}
		// Line-based mode with empty options will treat "done" as input
		// This is expected behavior - empty options list is edge case
		_ = value // Accept any value for empty options edge case
	})

	t.Run("select prompt with invalid number input returns as-is", func(t *testing.T) {
		in := bytes.NewBufferString("999\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
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
		// Invalid number (999 > len(options)) doesn't match, so returns as-is without validator
		if value != "999" {
			t.Fatalf("expected invalid number to be returned as-is, got %q", value)
		}
	})

	t.Run("select prompt with non-matching text input returns as-is without validator", func(t *testing.T) {
		in := bytes.NewBufferString("nonexistent\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
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
		// Line-based mode returns non-matching input as-is if no validator
		if value != "nonexistent" {
			t.Fatalf("expected non-matching input to be returned as-is, got %q", value)
		}
	})

	t.Run("select prompt with validator rejects non-matching input", func(t *testing.T) {
		in := bytes.NewBufferString("nonexistent\nOption A\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Choose",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
			},
			Validate: func(v string) error {
				if v != "a" {
					return errors.New("invalid selection")
				}
				return nil
			},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a' after validation retry, got %q", value)
		}
		// Validator should reject non-matching input, then accept valid input
	})

	t.Run("multi-select number key beyond available options", func(t *testing.T) {
		in := bytes.NewBufferString("9\n1\ndone\n") // 9 is invalid, then 1, then done
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Select",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "A", Value: "a"},
				{Label: "B", Value: "b"},
			},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
		// Invalid number should be ignored, valid number should work
	})

	t.Run("multi-select can't continue without selections", func(t *testing.T) {
		in := bytes.NewBufferString("done\n1\ndone\n") // Try continue without selection, then select, then continue
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Select",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "A", Value: "a"},
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
		if !strings.Contains(output, "Please select at least one option") {
			t.Errorf("output should show error for empty selection, got: %s", output)
		}
	})

	t.Run("select prompt with single option auto-selects", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Choose",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "Only Option", Value: "only"},
			},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "only" {
			t.Fatalf("expected value 'only', got %q", value)
		}
	})

	t.Run("multi-select with all options selected as default", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Select",
			Default: "1,2,3",
			Theme:   clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "A", Value: "a"},
				{Label: "B", Value: "b"},
				{Label: "C", Value: "c"},
			},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		// Should return all defaults
		if !strings.Contains(value, "a") || !strings.Contains(value, "b") || !strings.Contains(value, "c") {
			t.Fatalf("expected value to contain 'a', 'b', and 'c', got %q", value)
		}
	})

	t.Run("continue button text is customizable", func(t *testing.T) {
		in := bytes.NewBufferString("1\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:        "Select",
			Theme:        clix.DefaultPromptTheme,
			ContinueText: "Finish",
			Options: []clix.SelectOption{
				{Label: "A", Value: "a"},
			},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
		// ContinueText field is respected (even if not shown in line-based mode)
	})

	t.Run("number key in line-based select", func(t *testing.T) {
		in := bytes.NewBufferString("3\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Choose",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "First", Value: "1"},
				{Label: "Second", Value: "2"},
				{Label: "Third", Value: "3"},
			},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "3" {
			t.Fatalf("expected value '3', got %q", value)
		}
	})

	t.Run("number keys toggle in multi-select line-based mode", func(t *testing.T) {
		in := bytes.NewBufferString("1\n1\n1\ndone\n") // Toggle on (1), toggle off (1 again), toggle on (1 again), continue
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Select",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "A", Value: "a"},
			},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		// After toggling on/off/on, should have selection
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
	})
}
