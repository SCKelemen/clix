package clix

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSimpleTextPrompterReadsInput(t *testing.T) {
	in := bytes.NewBufferString("custom\n")
	out := &bytes.Buffer{}

	prompter := SimpleTextPrompter{In: in, Out: out}
	value, err := prompter.Prompt(context.Background(), PromptRequest{
		Label: "Enter value",
		Theme: DefaultPromptTheme,
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if value != "custom" {
		t.Fatalf("expected value 'custom', got %q", value)
	}

	expected := "? Enter value: "
	if out.String() != expected {
		t.Fatalf("expected prompt %q, got %q", expected, out.String())
	}
}

func TestSimpleTextPrompterUsesDefault(t *testing.T) {
	in := bytes.NewBufferString("\n")
	out := &bytes.Buffer{}

	prompter := SimpleTextPrompter{In: in, Out: out}
	value, err := prompter.Prompt(context.Background(), PromptRequest{
		Label:   "Colour",
		Default: "blue",
		Theme:   DefaultPromptTheme,
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if value != "blue" {
		t.Fatalf("expected default value 'blue', got %q", value)
	}

	expected := "? Colour [blue]: "
	if out.String() != expected {
		t.Fatalf("expected prompt %q, got %q", expected, out.String())
	}
}

func TestSimpleTextPrompterValidatesInput(t *testing.T) {
	in := bytes.NewBufferString("bad\nvalid\n")
	out := &bytes.Buffer{}

	prompter := SimpleTextPrompter{In: in, Out: out}
	attempts := 0
	value, err := prompter.Prompt(context.Background(), PromptRequest{
		Label: "Code",
		Theme: PromptTheme{Prefix: "? ", Error: "! "},
		Validate: func(v string) error {
			attempts++
			if v != "valid" {
				return errors.New("value must be 'valid'")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if value != "valid" {
		t.Fatalf("expected value 'valid', got %q", value)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 validation attempts, got %d", attempts)
	}

	expected := "? Code: ! value must be 'valid'\n? Code: "
	if out.String() != expected {
		t.Fatalf("expected output %q, got %q", expected, out.String())
	}
}

func TestSimpleTextPrompterAppliesStyles(t *testing.T) {
	in := bytes.NewBufferString("bad\nvalid\n")
	out := &bytes.Buffer{}

	theme := PromptTheme{
		Prefix: "?> ",
		Hint:   "(hint)",
		Error:  "x ",
		PrefixStyle: StyleFunc(func(strs ...string) string {
			return "P:" + strs[0]
		}),
		LabelStyle: StyleFunc(func(strs ...string) string {
			return strings.ToUpper(strs[0])
		}),
		DefaultStyle: StyleFunc(func(strs ...string) string {
			return "D:" + strs[0]
		}),
		HintStyle: StyleFunc(func(strs ...string) string {
			return "H:" + strs[0]
		}),
		ErrorStyle: StyleFunc(func(strs ...string) string {
			return "E:" + strs[0]
		}),
	}

	prompter := SimpleTextPrompter{In: in, Out: out}
	_, err := prompter.Prompt(context.Background(), PromptRequest{
		Label:   "value",
		Default: "fallback",
		Theme:   theme,
		Validate: func(v string) error {
			if v != "valid" {
				return errors.New("value must be 'valid'")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}

	expected := "P:?> VALUE [D:fallback] H:(hint): E:x E:value must be 'valid'\nP:?> VALUE [D:fallback] H:(hint): "
	if out.String() != expected {
		t.Fatalf("expected output %q, got %q", expected, out.String())
	}
}

func TestSimpleTextPrompterRequiresIO(t *testing.T) {
	_, err := SimpleTextPrompter{In: nil, Out: nil}.Prompt(context.Background(), PromptRequest{})
	if err == nil {
		t.Fatal("expected error when IO is missing")
	}
}

func TestSimpleTextPrompterRejectsConfirmPrompts(t *testing.T) {
	prompter := SimpleTextPrompter{In: bytes.NewBufferString(""), Out: &bytes.Buffer{}}
	_, err := prompter.Prompt(context.Background(), PromptRequest{
		Label:   "Continue?",
		Confirm: true,
		Theme:   DefaultPromptTheme,
	})
	if err == nil {
		t.Fatal("expected error for confirm prompt")
	}
	if !strings.Contains(err.Error(), "confirm prompts require the prompt extension") {
		t.Fatalf("expected error about extension, got: %v", err)
	}
}

func TestSimpleTextPrompterRejectsSelectPrompts(t *testing.T) {
	prompter := SimpleTextPrompter{In: bytes.NewBufferString(""), Out: &bytes.Buffer{}}
	_, err := prompter.Prompt(context.Background(), PromptRequest{
		Label: "Choose",
		Theme: DefaultPromptTheme,
		Options: []SelectOption{
			{Label: "Option A", Value: "a"},
		},
	})
	if err == nil {
		t.Fatal("expected error for select prompt")
	}
	if !strings.Contains(err.Error(), "select prompts require the prompt extension") {
		t.Fatalf("expected error about extension, got: %v", err)
	}
}

func TestSimpleTextPrompterRejectsMultiSelectPrompts(t *testing.T) {
	prompter := SimpleTextPrompter{In: bytes.NewBufferString(""), Out: &bytes.Buffer{}}
	_, err := prompter.Prompt(context.Background(), PromptRequest{
		Label:       "Select",
		Theme:       DefaultPromptTheme,
		MultiSelect: true,
		Options: []SelectOption{
			{Label: "Option A", Value: "a"},
		},
	})
	if err == nil {
		t.Fatal("expected error for multi-select prompt")
	}
	if !strings.Contains(err.Error(), "multi-select prompts require the prompt extension") {
		t.Fatalf("expected error about extension, got: %v", err)
	}
}
