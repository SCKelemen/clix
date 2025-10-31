package survey

import (
	"bytes"
	"clix"
	"clix/ext/prompt"
	"context"
	"testing"
)

func TestSurveyWithTextPrompter(t *testing.T) {
	t.Run("text prompt works", func(t *testing.T) {
		in := bytes.NewBufferString("Alice\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		s.Question("name", clix.PromptRequest{
			Label: "Name",
			Theme: clix.DefaultPromptTheme,
		}).End()

		s.Start("name")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 1 || answers[0] != "Alice" {
			t.Fatalf("expected ['Alice'], got %v", answers)
		}
	})

	t.Run("confirm prompt works", func(t *testing.T) {
		in := bytes.NewBufferString("y\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		s.Question("confirm", clix.PromptRequest{
			Label:   "Proceed?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		}).End()

		s.Start("confirm")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 1 || answers[0] != "y" {
			t.Fatalf("expected ['y'], got %v", answers)
		}
	})
}

func TestSurveyWithTerminalPrompter(t *testing.T) {
	t.Run("select prompt works", func(t *testing.T) {
		in := bytes.NewBufferString("1\n")
		out := &bytes.Buffer{}

		prompter := prompt.TerminalPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		s.Question("choose", clix.PromptRequest{
			Label: "Choose option",
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
			},
			Theme: clix.DefaultPromptTheme,
		}).End()

		s.Start("choose")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 1 || answers[0] != "a" {
			t.Fatalf("expected ['a'], got %v", answers)
		}
	})

	t.Run("multi-select prompt works", func(t *testing.T) {
		in := bytes.NewBufferString("1\ndone\n")
		out := &bytes.Buffer{}

		prompter := prompt.TerminalPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		s.Question("select", clix.PromptRequest{
			Label: "Select options",
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
				{Label: "Option B", Value: "b"},
			},
			MultiSelect: true,
			Theme:       clix.DefaultPromptTheme,
		}).End()

		s.Start("select")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 1 {
			t.Fatalf("expected 1 answer, got %d: %v", len(answers), answers)
		}
		// Multi-select returns comma-separated values
		if answers[0] != "a" {
			t.Fatalf("expected ['a'], got %v", answers)
		}
	})

	t.Run("mixed prompt types in survey", func(t *testing.T) {
		// Text input, then select, then confirm
		// For line-based fallback (bytes.Buffer):
		// - Select prompts accept option numbers (1-based) or option labels
		// - Each prompt creates its own bufio.Reader, so input must be available sequentially
		in := bytes.NewBufferString("Alice\n1\ny\n")
		out := &bytes.Buffer{}

		prompter := prompt.TerminalPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)

		s.Question("name", clix.PromptRequest{
			Label: "Name",
			Theme: clix.DefaultPromptTheme,
		}).Then("choose")

		s.Question("choose", clix.PromptRequest{
			Label: "Choose option",
			Options: []clix.SelectOption{
				{Label: "Option A", Value: "a"},
			},
			Theme: clix.DefaultPromptTheme,
		}).Then("confirm")

		s.Question("confirm", clix.PromptRequest{
			Label:   "Proceed?",
			Confirm: true,
			Theme:   clix.DefaultPromptTheme,
		}).End()

		s.Start("name")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 3 {
			t.Fatalf("expected 3 answers, got %d: %v", len(answers), answers)
		}
		if answers[0] != "Alice" {
			t.Fatalf("answers[0]: expected 'Alice', got %q", answers[0])
		}
		if answers[1] != "a" {
			t.Fatalf("answers[1]: expected 'a', got %q", answers[1])
		}
		// Confirm returns "y" or "n"
		if answers[2] != "y" && answers[2] != "n" {
			t.Fatalf("answers[2]: expected 'y' or 'n', got %q", answers[2])
		}
	})
}
