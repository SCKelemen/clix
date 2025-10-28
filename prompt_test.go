package clix

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestTerminalPrompterReadsInput(t *testing.T) {
	in := bytes.NewBufferString("custom\n")
	out := &bytes.Buffer{}

	prompter := TerminalPrompter{In: in, Out: out}
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

func TestTerminalPrompterUsesDefault(t *testing.T) {
	in := bytes.NewBufferString("\n")
	out := &bytes.Buffer{}

	prompter := TerminalPrompter{In: in, Out: out}
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

func TestTerminalPrompterValidatesInput(t *testing.T) {
	in := bytes.NewBufferString("bad\nvalid\n")
	out := &bytes.Buffer{}

	prompter := TerminalPrompter{In: in, Out: out}
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

func TestTerminalPrompterRequiresIO(t *testing.T) {
	_, err := TerminalPrompter{In: nil, Out: nil}.Prompt(context.Background(), PromptRequest{})
	if err == nil {
		t.Fatal("expected error when IO is missing")
	}
}
