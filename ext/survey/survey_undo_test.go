package survey

import (
	"bytes"
	"clix"
	"context"
	"testing"
)

func TestSurveyUndo(t *testing.T) {
	t.Run("undo allows going back to previous question", func(t *testing.T) {
		in := bytes.NewBufferString("Alice\nback\nBob\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "name",
				Request: clix.PromptRequest{
					Label: "Name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "name", WithUndoStack())

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 1 || answers[0] != "Bob" {
			t.Fatalf("expected ['Bob'] (after undo and re-answer), got %v", answers)
		}
	})

	t.Run("undo with multiple questions", func(t *testing.T) {
		// Flow: first question -> answer "Alice" -> last question -> answer "Bob" -> back -> answer "Charlie"
		// Expected: ["Alice", "Charlie"] (Bob was undone)
		in := bytes.NewBufferString("Alice\nBob\nback\nCharlie\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "first",
				Request: clix.PromptRequest{
					Label: "First name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": PushQuestion("last"),
				},
			},
			{
				ID: "last",
				Request: clix.PromptRequest{
					Label: "Last name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "first", WithUndoStack())

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 2 {
			t.Fatalf("expected 2 answers, got %d: %v", len(answers), answers)
		}
		// The undo should have removed "Bob" and replaced it with "Charlie"
		// But the current implementation might keep "Bob" and add "Charlie"
		// Let's check what actually happens and adjust the test
		if answers[0] != "Alice" {
			t.Fatalf("expected first answer 'Alice', got %q", answers[0])
		}
		// After undo and re-answer, we should have Charlie, not Bob
		if answers[1] != "Charlie" {
			t.Fatalf("expected second answer 'Charlie' (after undo of Bob), got %q. Full answers: %v", answers[1], answers)
		}
	})

	t.Run("undo at first question has no effect", func(t *testing.T) {
		// At first question, typing "back" with no history should just ask again
		// Input: first prompt shows, user types "back", question asked again, user types "Alice"
		in := bytes.NewBufferString("back\nAlice\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "name",
				Request: clix.PromptRequest{
					Label: "Name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "name", WithUndoStack())

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		// "back" is not saved as an answer, so we should only have "Alice"
		if len(answers) != 1 || answers[0] != "Alice" {
			t.Fatalf("expected ['Alice'], got %v", answers)
		}
	})
}

func TestSurveyEndCard(t *testing.T) {
	t.Run("end card appears after survey completion", func(t *testing.T) {
		in := bytes.NewBufferString("Alice\ny\n") // y to confirm after end card
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "name",
				Request: clix.PromptRequest{
					Label: "Name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "name", WithEndCard())

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		output := out.String()
		if !contains(output, "Survey complete") {
			t.Fatalf("expected end card message, got: %s", output)
		}
	})

	t.Run("end card with undo allows going back", func(t *testing.T) {
		in := bytes.NewBufferString("Alice\nback\nBob\ny\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "name",
				Request: clix.PromptRequest{
					Label: "Name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "name", WithEndCard(), WithUndoStack())

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 1 || answers[0] != "Bob" {
			t.Fatalf("expected ['Bob'] (after going back from end card), got %v", answers)
		}
	})

	t.Run("custom end card text", func(t *testing.T) {
		in := bytes.NewBufferString("Alice\ny\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "name",
				Request: clix.PromptRequest{
					Label: "Name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "name", WithEndCardText("Done!"))

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		output := out.String()
		if !contains(output, "Done!") {
			t.Fatalf("expected custom end card text, got: %s", output)
		}
	})

	t.Run("end card no allows going back if undo enabled", func(t *testing.T) {
		in := bytes.NewBufferString("Alice\nno\nBob\ny\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "name",
				Request: clix.PromptRequest{
					Label: "Name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "name", WithEndCard(), WithUndoStack())

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 1 || answers[0] != "Bob" {
			t.Fatalf("expected ['Bob'] (after saying no and going back), got %v", answers)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
