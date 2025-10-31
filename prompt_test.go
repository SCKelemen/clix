package clix

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTextPrompterReadsInput(t *testing.T) {
	in := bytes.NewBufferString("custom\n")
	out := &bytes.Buffer{}

	prompter := TextPrompter{In: in, Out: out}
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

func TestTextPrompterUsesDefault(t *testing.T) {
	in := bytes.NewBufferString("\n")
	out := &bytes.Buffer{}

	prompter := TextPrompter{In: in, Out: out}
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

func TestTextPrompterValidatesInput(t *testing.T) {
	in := bytes.NewBufferString("bad\nvalid\n")
	out := &bytes.Buffer{}

	prompter := TextPrompter{In: in, Out: out}
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

func TestTextPrompterAppliesStyles(t *testing.T) {
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

	prompter := TextPrompter{In: in, Out: out}
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

func TestTextPrompterRequiresIO(t *testing.T) {
	_, err := TextPrompter{In: nil, Out: nil}.Prompt(context.Background(), PromptRequest{})
	if err == nil {
		t.Fatal("expected error when IO is missing")
	}
}

func TestTextPrompterSupportsConfirm(t *testing.T) {
	in := bytes.NewBufferString("y\n")
	out := &bytes.Buffer{}

	prompter := TextPrompter{In: in, Out: out}
	value, err := prompter.Prompt(context.Background(), PromptRequest{
		Label:   "Continue?",
		Confirm: true,
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if value != "y" {
		t.Fatalf("expected 'y', got %q", value)
	}

	output := out.String()
	if !strings.Contains(output, "(Y/n)") {
		t.Errorf("output should show default, got: %s", output)
	}
}
