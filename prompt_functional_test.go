package clix

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// TestFunctionalOptions tests that functional options work correctly
// alongside the struct-based API.
func TestFunctionalOptions(t *testing.T) {
	t.Run("functional options work with TextPrompter", func(t *testing.T) {
		in := bytes.NewBufferString("test-value\n")
		out := &bytes.Buffer{}

		prompter := TextPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			WithLabel("Enter value"),
			WithTheme(DefaultPromptTheme),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "test-value" {
			t.Fatalf("expected value 'test-value', got %q", value)
		}
	})

	t.Run("functional options with default", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TextPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			WithLabel("Color"),
			WithDefault("blue"),
			WithTheme(DefaultPromptTheme),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "blue" {
			t.Fatalf("expected default value 'blue', got %q", value)
		}
	})

	t.Run("functional options with validation", func(t *testing.T) {
		in := bytes.NewBufferString("invalid\nvalid\n")
		out := &bytes.Buffer{}

		prompter := TextPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			WithLabel("Code"),
			WithTheme(DefaultPromptTheme),
			WithValidate(func(v string) error {
				if v != "valid" {
					return errors.New("must be 'valid'")
				}
				return nil
			}),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "valid" {
			t.Fatalf("expected value 'valid', got %q", value)
		}
	})

	t.Run("functional options with confirm", func(t *testing.T) {
		in := bytes.NewBufferString("y\n")
		out := &bytes.Buffer{}

		prompter := TextPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			WithLabel("Continue?"),
			WithConfirm(),
			WithTheme(DefaultPromptTheme),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "y" {
			t.Fatalf("expected value 'y', got %q", value)
		}

		output := out.String()
		if !strings.Contains(output, "(Y/n)") {
			t.Errorf("output should show default, got: %s", output)
		}
	})

	t.Run("functional options can be mixed with struct", func(t *testing.T) {
		in := bytes.NewBufferString("test\n")
		out := &bytes.Buffer{}

		prompter := TextPrompter{In: in, Out: out}
		// Mix: struct for Label, functional for Default
		value, err := prompter.Prompt(context.Background(),
			PromptRequest{Label: "Name"},
			WithDefault("unknown"),
			WithTheme(DefaultPromptTheme),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "test" {
			t.Fatalf("expected value 'test', got %q", value)
		}
	})

	t.Run("functional options override struct values", func(t *testing.T) {
		in := bytes.NewBufferString("\n")
		out := &bytes.Buffer{}

		prompter := TextPrompter{In: in, Out: out}
		// Struct sets default "old", functional option overrides to "new"
		value, err := prompter.Prompt(context.Background(),
			PromptRequest{Label: "Value", Default: "old"},
			WithDefault("new"),
			WithTheme(DefaultPromptTheme),
		)
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "new" {
			t.Fatalf("expected value 'new' (from functional option), got %q", value)
		}
	})
}
