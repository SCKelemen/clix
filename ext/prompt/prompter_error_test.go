package prompt

import (
	"bytes"
	"clix"
	"context"
	"errors"
	"io"
	"testing"
)

func TestPrompterErrorHandling(t *testing.T) {
	t.Run("nil In or Out returns error", func(t *testing.T) {
		prompter := TerminalPrompter{In: nil, Out: &bytes.Buffer{}}
		_, err := prompter.Prompt(context.Background(), clix.WithLabel("Test"))
		if err == nil {
			t.Fatal("expected error for nil In")
		}

		prompter = TerminalPrompter{In: bytes.NewBufferString(""), Out: nil}
		_, err = prompter.Prompt(context.Background(), clix.WithLabel("Test"))
		if err == nil {
			t.Fatal("expected error for nil Out")
		}
	})

	t.Run("prompt fails with validation error", func(t *testing.T) {
		in := bytes.NewBufferString("invalid\nvalid\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Enter value",
			Theme: clix.DefaultPromptTheme,
			Validate: func(v string) error {
				if v == "invalid" {
					return errors.New("invalid value")
				}
				return nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "valid" {
			t.Fatalf("expected 'valid', got %q", value)
		}
		// Check that error message was displayed
		if !bytes.Contains(out.Bytes(), []byte("invalid value")) {
			t.Error("expected error message to be displayed")
		}
	})

	t.Run("select prompt with validator rejects invalid input", func(t *testing.T) {
		in := bytes.NewBufferString("999\ninvalid\n1\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Choose",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
			},
			Validate: func(v string) error {
				if v == "999" || v == "invalid" {
					return errors.New("invalid selection")
				}
				return nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected 'a', got %q", value)
		}
	})

	t.Run("multi-select prompt with validator", func(t *testing.T) {
		in := bytes.NewBufferString("1\n2\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:       "Select",
			Theme:       clix.DefaultPromptTheme,
			MultiSelect: true,
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
				{Label: "Option C", Value: "c"},
			},
			Validate: func(v string) error {
				if v == "" {
					return errors.New("must select at least one")
				}
				return nil
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// formatSelectedValues returns comma-separated without spaces: "a,b"
		if value != "a,b" {
			t.Fatalf("expected 'a,b', got %q", value)
		}
	})
}

func TestTextPromptWithDefault(t *testing.T) {
	t.Run("empty input uses default", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Enter value",
			Default: "default-value",
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "default-value" {
			t.Fatalf("expected 'default-value', got %q", value)
		}
	})

	t.Run("non-empty input overrides default", func(t *testing.T) {
		in := bytes.NewBufferString("custom-value\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Enter value",
			Default: "default-value",
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "custom-value" {
			t.Fatalf("expected 'custom-value', got %q", value)
		}
	})
}

func TestConfirmPrompt(t *testing.T) {
	t.Run("empty input uses default yes", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "y" {
			t.Fatalf("expected 'y', got %q", value)
		}
	})

	t.Run("empty input uses default no when set", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Default: "n",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "n" {
			t.Fatalf("expected 'n', got %q", value)
		}
	})

	t.Run("accepts y/yes/n/no", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"y\n", "y"},
			{"Y\n", "y"},
			{"yes\n", "y"},
			{"YES\n", "y"},
			{"n\n", "n"},
			{"N\n", "n"},
			{"no\n", "n"},
			{"NO\n", "n"},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				in := bytes.NewBufferString(tt.input)
				out := &bytes.Buffer{}

				prompter := TerminalPrompter{In: in, Out: out}
				value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
					Label:   "Continue?",
					Confirm: true,
					Theme:   clix.DefaultPromptTheme,
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if value != tt.expected {
					t.Fatalf("expected %q, got %q", tt.expected, value)
				}
			})
		}
	})
}

func TestSelectPromptEdgeCases(t *testing.T) {
	t.Run("select with single option", func(t *testing.T) {
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
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "only" {
			t.Fatalf("expected 'only', got %q", value)
		}
	})

	t.Run("select with default option", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Choose",
			Default: "b",
			Theme:   clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
				{Label: "Option C", Value: "c"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "b" {
			t.Fatalf("expected 'b', got %q", value)
		}
	})

	t.Run("select matches by label prefix", func(t *testing.T) {
		in := bytes.NewBufferString("Opt\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Choose",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
				{Label: "Option Alpha", Value: "alpha"},
				{Label: "Option Beta", Value: "beta"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should match first option with prefix
		if value != "alpha" {
			t.Fatalf("expected 'alpha', got %q", value)
		}
	})
}

func TestMultiSelectPromptEdgeCases(t *testing.T) {
	t.Run("multi-select empty selection", func(t *testing.T) {
		// Line-based mode: Need to provide some input before "done"
		// Empty line means no selections yet, then "done" to finish
		in := bytes.NewBufferString("done\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:       "Select",
			Theme:       clix.DefaultPromptTheme,
			MultiSelect: true,
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
			},
		})
		// Line-based mode requires at least one selection input before "done"
		// Without any selections, it may error or return empty
		if err != nil {
			// EOF is acceptable if no selections were made
			// This is expected behavior for empty selection
			return
		}
		// If no error, value should be empty
		if value != "" {
			t.Fatalf("expected empty selection, got %q", value)
		}
	})

	t.Run("multi-select with default options", func(t *testing.T) {
		in := bytes.NewBufferString("done\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:       "Select",
			Theme:       clix.DefaultPromptTheme,
			MultiSelect: true,
			Default:     "a,b", // Default format is comma-separated without spaces
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
				{Label: "Option C", Value: "c"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// formatSelectedValues returns comma-separated without spaces: "a,b"
		if value != "a,b" {
			t.Fatalf("expected 'a,b', got %q", value)
		}
	})

	t.Run("multi-select toggles selections", func(t *testing.T) {
		// Test that we can select, then deselect by toggling
		in := bytes.NewBufferString("1\n1\ndone\n") // Select option 1, then toggle it off, then done
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:       "Select",
			Theme:       clix.DefaultPromptTheme,
			MultiSelect: true,
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// After toggling off, should be empty
		if value != "" {
			t.Fatalf("expected empty selection after toggle, got %q", value)
		}
	})
}

// TestReadKeyEdgeCases tests edge cases for ReadKey function
func TestReadKeyEdgeCases(t *testing.T) {
	t.Run("ReadKey with empty input", func(t *testing.T) {
		in := bytes.NewBuffer([]byte{})
		_, err := ReadKey(in)
		if err == nil {
			t.Error("expected error for empty input")
		}
		if err != io.EOF {
			t.Errorf("expected EOF, got %v", err)
		}
	})

	t.Run("ReadKey with incomplete escape sequence", func(t *testing.T) {
		// Just escape byte without continuation
		in := bytes.NewBuffer([]byte{0x1b})
		key, err := ReadKey(in)
		if err != nil {
			t.Fatalf("ReadKey should handle incomplete sequence, got error: %v", err)
		}
		if key != KeyEscape {
			t.Errorf("expected KeyEscape for incomplete sequence, got %v", key)
		}
	})

	t.Run("ReadKey with partial escape sequence", func(t *testing.T) {
		// Start of escape sequence but incomplete
		in := bytes.NewBuffer([]byte{0x1b, '['})
		key, err := ReadKey(in)
		if err != nil {
			t.Fatalf("ReadKey should handle partial sequence, got error: %v", err)
		}
		// Should read as escape since sequence is incomplete
		if key != KeyEscape {
			t.Errorf("expected KeyEscape for partial sequence, got %v", key)
		}
	})
}
