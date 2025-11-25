package survey

import (
	"bytes"
	"github.com/SCKelemen/clix"
	"context"
	"io"
	"testing"
)

// mockPrompterWithEscape simulates a prompter that returns ErrGoBack when requested
type mockPrompterWithEscape struct {
	callCount int
	answers   []string
	escapeAt  int       // Call number at which to return ErrGoBack (0 = never)
	out       io.Writer // Output writer for end card
}

func (m *mockPrompterWithEscape) Prompt(ctx context.Context, opts ...clix.PromptOption) (string, error) {
	m.callCount++
	// Return ErrGoBack if we're at the escape point
	if m.escapeAt > 0 && m.callCount == m.escapeAt {
		return "", ErrGoBack
	}
	// Otherwise return next answer
	if len(m.answers) > 0 {
		answer := m.answers[0]
		m.answers = m.answers[1:]
		return answer, nil
	}
	return "", nil
}

// Out returns the output writer (for compatibility with getOut)
func (m *mockPrompterWithEscape) Out() io.Writer {
	return m.out
}

func TestSurveyUndo(t *testing.T) {
	t.Run("undo allows going back to previous question", func(t *testing.T) {
		// Flow: Two questions, answer "Alice" to first, answer "Bob" to second
		// Since EndBranch clears the stack immediately after "Bob", the survey ends
		// We test undo from the end card instead: answer "Alice", "Bob", then escape from end card, then "Charlie"
		// Expected: After escape from end card, second question is re-asked, answer "Charlie" replaces "Bob"
		// So final answers should be ["Alice", "Charlie"]
		out := &bytes.Buffer{}
		prompter := &mockPrompterWithEscape{
			answers:  []string{"Alice", "Bob", "Charlie", "y"}, // y to confirm after end card
			escapeAt: 3,                                        // Return ErrGoBack on third call (from end card prompt, should undo "Bob")
			out:      out,
		}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "first",
				Request: clix.PromptRequest{
					Label: "First name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": PushQuestion("second"),
				},
			},
			{
				ID: "second",
				Request: clix.PromptRequest{
					Label: "Last name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "first", WithUndoStack(), WithEndCard())

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		// After undo of "Bob" from end card, we should have "Alice" and "Charlie"
		if len(answers) != 2 {
			t.Fatalf("expected 2 answers, got %d: %v", len(answers), answers)
		}
		if answers[0] != "Alice" {
			t.Fatalf("expected first answer 'Alice', got %q", answers[0])
		}
		if answers[1] != "Charlie" {
			t.Fatalf("expected second answer 'Charlie' (after undo of Bob), got %q. Full answers: %v", answers[1], answers)
		}
	})

	t.Run("undo with multiple questions", func(t *testing.T) {
		// Flow: first question -> answer "Alice" -> last question -> answer "Bob" -> end card -> Escape -> answer "Charlie"
		// Expected: ["Alice", "Charlie"] (Bob was undone from end card)
		out := &bytes.Buffer{}
		prompter := &mockPrompterWithEscape{
			answers:  []string{"Alice", "Bob", "Charlie", "y"}, // y to confirm after end card
			escapeAt: 3,                                        // Return ErrGoBack on third call (from end card, should undo "Bob")
			out:      out,
		}
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

		s := NewFromQuestions(ctx, prompter, questions, "first", WithUndoStack(), WithEndCard())

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 2 {
			t.Fatalf("expected 2 answers, got %d: %v", len(answers), answers)
		}
		if answers[0] != "Alice" {
			t.Fatalf("expected first answer 'Alice', got %q", answers[0])
		}
		if answers[1] != "Charlie" {
			t.Fatalf("expected second answer 'Charlie' (after undo of Bob), got %q. Full answers: %v", answers[1], answers)
		}
	})

	t.Run("undo at first question has no effect", func(t *testing.T) {
		// At first question, Escape with no history should just ask again
		// Input: first prompt shows, user presses Escape, question asked again, user types "Alice"
		prompter := &mockPrompterWithEscape{
			answers:  []string{"Alice"},
			escapeAt: 1, // Return ErrGoBack on first call (no history yet)
		}
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
		// Escape with no history should just re-ask, so we should only have "Alice"
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
		// This test requires TextPrompter for the end card, so we use a different approach
		// We'll test that "no" in the end card triggers go back
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
