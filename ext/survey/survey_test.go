package survey

import (
	"bytes"
	"github.com/SCKelemen/clix"
	"context"
	"strings"
	"testing"
)

func TestSurveyDepthFirstTraversal(t *testing.T) {
	t.Run("simple linear survey", func(t *testing.T) {
		// Depth-first: last added is processed first
		// We add "First name" then "Last name", so "Last name" is processed first
		in := bytes.NewBufferString("Doe\nJohn\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		s.Ask(clix.PromptRequest{
			Label: "First name",
			Theme: clix.DefaultPromptTheme,
		}, func(answer string, s *Survey) {
			// No nested questions
		})
		s.Ask(clix.PromptRequest{
			Label: "Last name",
			Theme: clix.DefaultPromptTheme,
		}, func(answer string, s *Survey) {
			// No nested questions
		})

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 2 {
			t.Fatalf("expected 2 answers, got %d", len(answers))
		}
		// Depth-first: Last name answered first (was added last)
		if answers[0] != "Doe" {
			t.Fatalf("expected first answer 'Doe' (last added), got %q", answers[0])
		}
		if answers[1] != "John" {
			t.Fatalf("expected second answer 'John', got %q", answers[1])
		}
	})

	t.Run("depth-first: nested questions processed first", func(t *testing.T) {
		in := bytes.NewBufferString("yes\nAlice\nyes\nBob\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		var order []string

		s := New(ctx, prompter)
		s.Ask(clix.PromptRequest{
			Label: "Do you want to add a child?",
			Theme: clix.DefaultPromptTheme,
		}, func(answer string, s *Survey) {
			order = append(order, "ask-child-question")
			if answer == "yes" {
				s.Ask(clix.PromptRequest{Label: "Child name", Theme: clix.DefaultPromptTheme}, func(childName string, s *Survey) {
					order = append(order, "ask-child-name-answer: "+childName)
					// Add another nested question
					s.Ask(clix.PromptRequest{Label: "Do you want to add another child?", Theme: clix.DefaultPromptTheme}, func(answer2 string, s *Survey) {
						order = append(order, "ask-another-child: "+answer2)
						if answer2 == "yes" {
							s.Ask(clix.PromptRequest{Label: "Second child name", Theme: clix.DefaultPromptTheme}, func(childName2 string, s *Survey) {
								order = append(order, "ask-second-child-name-answer: "+childName2)
							})
						}
					})
				})
			}
		})

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		// Verify depth-first order: nested questions are processed before returning to parent
		// Flow:
		// 1. "Do you want to add a child?" → yes → handler adds "Child name"
		// 2. "Child name" → Alice → handler adds "Do you want to add another child?"
		// 3. "Do you want to add another child?" → yes → handler adds "Second child name"
		// 4. "Second child name" → Bob → handler completes
		// All done - depth-first means we complete the nested branch fully
		expected := []string{
			"ask-child-question",
			"ask-child-name-answer: Alice",
			"ask-another-child: yes",
			"ask-second-child-name-answer: Bob",
		}

		if len(order) != len(expected) {
			t.Fatalf("expected %d order entries, got %d: %v", len(expected), len(order), order)
		}

		for i, exp := range expected {
			if order[i] != exp {
				t.Fatalf("order[%d]: expected %q, got %q", i, exp, order[i])
			}
		}

		answers := s.Answers()
		if len(answers) != 4 {
			t.Fatalf("expected 4 answers, got %d: %v", len(answers), answers)
		}
		// Verify answers: yes, Alice, yes, Bob
		if answers[0] != "yes" || answers[1] != "Alice" || answers[2] != "yes" || answers[3] != "Bob" {
			t.Fatalf("expected answers ['yes', 'Alice', 'yes', 'Bob'], got %v", answers)
		}
	})

	t.Run("no handler adds no nested questions", func(t *testing.T) {
		in := bytes.NewBufferString("yes\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		s.Ask(clix.PromptRequest{Label: "Question?", Theme: clix.DefaultPromptTheme}, nil) // No handler

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 1 {
			t.Fatalf("expected 1 answer, got %d", len(answers))
		}
	})

	t.Run("handler can add multiple questions", func(t *testing.T) {
		// Handler adds "First child" then "Second child"
		// Depth-first: "Second child" is processed first (added last)
		in := bytes.NewBufferString("yes\nBob\nAlice\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		s.Ask(clix.PromptRequest{Label: "Do you have children?", Theme: clix.DefaultPromptTheme}, func(answer string, s *Survey) {
			if answer == "yes" {
				s.Ask(clix.PromptRequest{Label: "First child name", Theme: clix.DefaultPromptTheme}, nil)
				s.Ask(clix.PromptRequest{Label: "Second child name", Theme: clix.DefaultPromptTheme}, nil)
			}
		})

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 3 {
			t.Fatalf("expected 3 answers, got %d: %v", len(answers), answers)
		}
		// Depth-first: Second child answered first (added last), then First child
		if answers[1] != "Bob" || answers[2] != "Alice" {
			t.Fatalf("expected answers ['yes', 'Bob', 'Alice'] (depth-first), got %v", answers)
		}
	})

	t.Run("complex nested structure", func(t *testing.T) {
		// Handler adds A then B, so B is processed first (depth-first)
		in := bytes.NewBufferString("yes\nb\na\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		var path []string

		s := New(ctx, prompter)
		s.Ask(clix.PromptRequest{Label: "Start?", Theme: clix.DefaultPromptTheme}, func(answer string, s *Survey) {
			path = append(path, "start: "+answer)
			if answer == "yes" {
				s.Ask(clix.PromptRequest{Label: "A", Theme: clix.DefaultPromptTheme}, func(answerA string, s *Survey) {
					path = append(path, "a: "+answerA)
				})
				s.Ask(clix.PromptRequest{Label: "B", Theme: clix.DefaultPromptTheme}, func(answerB string, s *Survey) {
					path = append(path, "b: "+answerB)
				})
			}
		})

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		// Depth-first: B is added last, so processed first
		expectedPath := []string{"start: yes", "b: b", "a: a"}
		if len(path) != len(expectedPath) {
			t.Fatalf("expected path length %d, got %d: %v", len(expectedPath), len(path), path)
		}
		for i, exp := range expectedPath {
			parts := strings.Split(exp, ":")
			if len(parts) != 2 {
				continue
			}
			if !strings.Contains(path[i], strings.TrimSpace(parts[1])) {
				t.Errorf("path[%d]: expected to contain %q, got %q", i, strings.TrimSpace(parts[1]), path[i])
			}
		}
	})
}

func TestSurveyAnswers(t *testing.T) {
	t.Run("answers collected in order", func(t *testing.T) {
		// Depth-first: Q3 processed first (added last)
		in := bytes.NewBufferString("3\n2\n1\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		s.Ask(clix.PromptRequest{Label: "Q1", Theme: clix.DefaultPromptTheme}, nil)
		s.Ask(clix.PromptRequest{Label: "Q2", Theme: clix.DefaultPromptTheme}, nil)
		s.Ask(clix.PromptRequest{Label: "Q3", Theme: clix.DefaultPromptTheme}, nil)

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 3 {
			t.Fatalf("expected 3 answers, got %d", len(answers))
		}
		// Depth-first order: last added is processed first
		if answers[0] != "3" || answers[1] != "2" || answers[2] != "1" {
			t.Fatalf("expected answers ['3', '2', '1'] (depth-first), got %v", answers)
		}
	})
}

func TestSurveyClear(t *testing.T) {
	t.Run("clear removes remaining questions", func(t *testing.T) {
		in := bytes.NewBufferString("test\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		s.Ask(clix.PromptRequest{Label: "Q1", Theme: clix.DefaultPromptTheme}, nil)
		s.Ask(clix.PromptRequest{Label: "Q2", Theme: clix.DefaultPromptTheme}, nil)

		s.Clear()
		if len(s.stack) != 0 {
			t.Fatalf("expected empty stack after clear, got %d questions", len(s.stack))
		}

		// Run should complete immediately
		if err := s.Run(); err != nil {
			t.Fatalf("survey failed after clear: %v", err)
		}

		if len(s.Answers()) != 0 {
			t.Fatalf("expected no answers after clearing and running, got %d", len(s.Answers()))
		}
	})
}
