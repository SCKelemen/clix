package prompt

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"clix"
)

func TestPromptExtension(t *testing.T) {
	t.Run("extension replaces SimpleTextPrompter with EnhancedTerminalPrompter", func(t *testing.T) {
		app := clix.NewApp("test")
		app.In = bytes.NewBufferString("")
		app.Out = &bytes.Buffer{}

		// Initially has SimpleTextPrompter
		_, ok := app.Prompter.(clix.SimpleTextPrompter)
		if !ok {
			t.Fatal("expected SimpleTextPrompter initially")
		}

		// Apply extension
		ext := Extension{}
		if err := ext.Extend(app); err != nil {
			t.Fatalf("extension failed: %v", err)
		}

		// Should now have EnhancedTerminalPrompter
		_, ok = app.Prompter.(EnhancedTerminalPrompter)
		if !ok {
			t.Fatal("expected EnhancedTerminalPrompter after extension")
		}
	})

	t.Run("extension enables select prompts", func(t *testing.T) {
		app := clix.NewApp("test")
		app.In = bytes.NewBufferString("1\n")
		app.Out = &bytes.Buffer{}

		app.AddExtension(Extension{})
		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		value, err := app.Prompter.Prompt(context.Background(), clix.PromptRequest{
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
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
	})

	t.Run("extension enables multi-select prompts", func(t *testing.T) {
		app := clix.NewApp("test")
		app.In = bytes.NewBufferString("1,2\ndone\n")
		app.Out = &bytes.Buffer{}

		app.AddExtension(Extension{})
		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		value, err := app.Prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Select",
			Theme: clix.DefaultPromptTheme,
			Options: []clix.SelectOption{
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

	t.Run("extension enables confirm prompts", func(t *testing.T) {
		app := clix.NewApp("test")
		app.In = bytes.NewBufferString("y\n")
		app.Out = &bytes.Buffer{}

		app.AddExtension(Extension{})
		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		value, err := app.Prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "y" {
			t.Fatalf("expected value 'y', got %q", value)
		}
	})

	t.Run("extension preserves text prompt functionality", func(t *testing.T) {
		app := clix.NewApp("test")
		app.In = bytes.NewBufferString("test-value\n")
		app.Out = &bytes.Buffer{}

		app.AddExtension(Extension{})
		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		value, err := app.Prompter.Prompt(context.Background(), clix.PromptRequest{
			Label: "Enter value",
			Theme: clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "test-value" {
			t.Fatalf("expected value 'test-value', got %q", value)
		}
	})

	t.Run("extension works with nil IO", func(t *testing.T) {
		app := clix.NewApp("test")
		app.In = nil
		app.Out = nil

		ext := Extension{}
		// Should not panic
		if err := ext.Extend(app); err != nil {
			t.Fatalf("extension should not fail with nil IO: %v", err)
		}
		// Prompter should still be set (though it won't work without IO)
		if app.Prompter == nil {
			t.Fatal("prompter should be set even with nil IO")
		}
	})
}

func TestPromptExtensionIntegration(t *testing.T) {
	t.Run("app without extension rejects advanced prompts", func(t *testing.T) {
		app := clix.NewApp("test")
		app.In = bytes.NewBufferString("")
		app.Out = &bytes.Buffer{}

		// Don't add extension

		_, err := app.Prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err == nil {
			t.Fatal("expected error for confirm prompt without extension")
		}
		if !strings.Contains(err.Error(), "prompt extension") {
			t.Fatalf("expected error about extension, got: %v", err)
		}
	})

	t.Run("app with extension accepts advanced prompts", func(t *testing.T) {
		app := clix.NewApp("test")
		app.In = bytes.NewBufferString("y\n")
		app.Out = &bytes.Buffer{}

		app.AddExtension(Extension{})
		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("failed to apply extensions: %v", err)
		}

		_, err := app.Prompter.Prompt(context.Background(), clix.PromptRequest{
			Label:   "Continue?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		})
		if err != nil {
			t.Fatalf("unexpected error with extension: %v", err)
		}
	})
}

