package prompt

import (
	"bytes"
	"context"
	"github.com/SCKelemen/clix/v2"
	"strings"
	"testing"
)

// This test demonstrates that lipgloss.Style can be used with prompts.
// Note: This test doesn't actually import lipgloss to avoid making it a required dependency,
// but it verifies the interface compatibility.
func TestPromptSupportsLipglossStyles(t *testing.T) {
	// Create custom styles using clix.StyleFunc (same interface that lipgloss.Style implements)
	prefixStyle := clix.StyleFunc(func(strs ...string) string {
		return "🔹 " + strs[0]
	})

	labelStyle := clix.StyleFunc(func(strs ...string) string {
		return strings.ToUpper(strs[0])
	})

	theme := clix.PromptTheme{
		Prefix:      "? ",
		PrefixStyle: prefixStyle,
		LabelStyle:  labelStyle,
	}

	in := bytes.NewBufferString("test\n")
	out := &bytes.Buffer{}

	prompter := TerminalPrompter{In: in, Out: out}
	value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
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
	accentStyle := clix.StyleFunc(func(strs ...string) string {
		return "→ " + strs[0]
	})

	theme := clix.PromptTheme{
		Prefix:       "? ",
		LabelStyle:   accentStyle,
		DefaultStyle: accentStyle,
	}

	in := bytes.NewBufferString("1\n")
	out := &bytes.Buffer{}

	prompter := TerminalPrompter{In: in, Out: out}
	_, err := prompter.Prompt(context.Background(),
		clix.PromptRequest{
			Label: "Choose option",
			Theme: theme,
			Options: []clix.SelectOption{
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
	errorStyle := clix.StyleFunc(func(strs ...string) string {
		return "⚠️  " + strs[0]
	})

	theme := clix.PromptTheme{
		Prefix:     "? ",
		Error:      "! ",
		ErrorStyle: errorStyle,
	}

	in := bytes.NewBufferString("invalid\ny\n")
	out := &bytes.Buffer{}

	prompter := TerminalPrompter{In: in, Out: out}
	value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
		Label:   "Continue?",
		Theme:   theme,
		Confirm: true,
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

func TestPromptMultiSelectSupportsLipglossStyles(t *testing.T) {
	accentStyle := clix.StyleFunc(func(strs ...string) string {
		return "✓ " + strs[0]
	})

	theme := clix.PromptTheme{
		Prefix:     "? ",
		LabelStyle: accentStyle,
	}

	in := bytes.NewBufferString("1,2\ndone\n")
	out := &bytes.Buffer{}

	prompter := TerminalPrompter{In: in, Out: out}
	value, err := prompter.Prompt(context.Background(), clix.PromptRequest{
		Label: "Select features",
		Theme: theme,
		Options: []clix.SelectOption{
			{Label: "Feature A", Value: "a"},
			{Label: "Feature B", Value: "b"},
			{Label: "Feature C", Value: "c"},
		},
		MultiSelect: true,
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if !strings.Contains(value, "a") || !strings.Contains(value, "b") {
		t.Fatalf("expected value to contain 'a' and 'b', got %q", value)
	}

	output := out.String()
	if !strings.Contains(output, "✓") {
		t.Errorf("output should contain styled elements, got: %s", output)
	}
}
