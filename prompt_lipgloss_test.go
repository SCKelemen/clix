package clix

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// This test demonstrates that lipgloss.Style can be used with prompts.
// Note: This test doesn't actually import lipgloss to avoid making it a required dependency,
// but it verifies the interface compatibility.
func TestPromptSupportsLipglossStyles(t *testing.T) {
	// Create custom styles using StyleFunc (same interface that lipgloss.Style implements)
	prefixStyle := StyleFunc(func(strs ...string) string {
		return "🔹 " + strs[0]
	})

	labelStyle := StyleFunc(func(strs ...string) string {
		return strings.ToUpper(strs[0])
	})

	theme := PromptTheme{
		Prefix:      "? ",
		PrefixStyle: prefixStyle,
		LabelStyle:  labelStyle,
	}

	in := bytes.NewBufferString("test\n")
	out := &bytes.Buffer{}

	prompter := TerminalPrompter{In: in, Out: out}
	value, err := prompter.Prompt(context.Background(), PromptRequest{
		Label: "Enter name",
		Theme: theme,
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if value != "test" {
		t.Fatalf("expected value 'test', got %q", value)
	}

	output := out.String()
	if !strings.Contains(output, "🔹") {
		t.Errorf("output should contain styled prefix, got: %s", output)
	}
	if !strings.Contains(output, "ENTER NAME") {
		t.Errorf("output should contain styled label, got: %s", output)
	}
}

func TestPromptSelectSupportsLipglossStyles(t *testing.T) {
	accentStyle := StyleFunc(func(strs ...string) string {
		return "→ " + strs[0]
	})

	theme := PromptTheme{
		Prefix:       "? ",
		LabelStyle:   accentStyle,
		DefaultStyle: accentStyle,
	}

	in := bytes.NewBufferString("1\n")
	out := &bytes.Buffer{}

	prompter := TerminalPrompter{In: in, Out: out}
	_, err := prompter.Prompt(context.Background(), PromptRequest{
		Label: "Choose option",
		Theme: theme,
		Options: []SelectOption{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
		},
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "→") {
		t.Errorf("output should contain styled elements, got: %s", output)
	}
}

func TestPromptConfirmSupportsLipglossStyles(t *testing.T) {
	errorStyle := StyleFunc(func(strs ...string) string {
		return "⚠️  " + strs[0]
	})

	theme := PromptTheme{
		Prefix:     "? ",
		Error:      "! ",
		ErrorStyle: errorStyle,
	}

	in := bytes.NewBufferString("invalid\ny\n")
	out := &bytes.Buffer{}

	prompter := TerminalPrompter{In: in, Out: out}
	value, err := prompter.Prompt(context.Background(), PromptRequest{
		Label:   "Continue?",
		Confirm: true,
		Theme:   theme,
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if value != "y" {
		t.Fatalf("expected value 'y', got %q", value)
	}

	output := out.String()
	if !strings.Contains(output, "⚠️") {
		t.Errorf("output should contain styled error, got: %s", output)
	}
}
