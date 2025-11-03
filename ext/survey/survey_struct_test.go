package survey

import (
	"bytes"
	"clix"
	"context"
	"testing"
)

func TestStructBasedSurvey(t *testing.T) {
	t.Run("simple struct-based survey", func(t *testing.T) {
		in := bytes.NewBufferString("y\nAlice\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "add-child",
				Request: clix.PromptRequest{
					Label:   "Do you want to add a child?",
					Confirm: true, // Use confirm instead of select for TextPrompter
					Theme:   clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"y":   PushQuestion("child-name"),
					"yes": PushQuestion("child-name"),
					"n":   End(),
					"no":  End(),
				},
			},
			{
				ID: "child-name",
				Request: clix.PromptRequest{
					Label: "Child's name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "add-child")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 2 {
			t.Fatalf("expected 2 answers, got %d: %v", len(answers), answers)
		}
		if (answers[0] != "y" && answers[0] != "yes") || answers[1] != "Alice" {
			t.Fatalf("expected answers ['y'/'yes', 'Alice'], got %v", answers)
		}
	})

	t.Run("struct-based survey with loop", func(t *testing.T) {
		in := bytes.NewBufferString("y\nAlice\ny\nBob\nn\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "add-child",
				Request: clix.PromptRequest{
					Label:   "Do you want to add a child?",
					Confirm: true, // Use confirm instead of select for TextPrompter
					Theme:   clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"y":   PushQuestion("child-name"),
					"yes": PushQuestion("child-name"),
					"n":   End(),
					"no":  End(),
				},
			},
			{
				ID: "child-name",
				Request: clix.PromptRequest{
					Label: "Child's name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": PushQuestion("add-another"),
				},
			},
			{
				ID: "add-another",
				Request: clix.PromptRequest{
					Label:   "Add another child?",
					Confirm: true,
					Theme:   clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"y": PushQuestion("child-name"), // Loop back
					"n": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "add-child")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 5 {
			t.Fatalf("expected 5 answers, got %d: %v", len(answers), answers)
		}
		// Confirm prompts return "y" or "n"
		if (answers[0] != "y" && answers[0] != "yes") || answers[1] != "Alice" {
			t.Fatalf("answers[0-1]: expected ['y'/'yes', 'Alice'], got %q, %q", answers[0], answers[1])
		}
		if (answers[2] != "y" && answers[2] != "yes") || answers[3] != "Bob" {
			t.Fatalf("answers[2-3]: expected ['y'/'yes', 'Bob'], got %q, %q", answers[2], answers[3])
		}
		if answers[4] != "n" && answers[4] != "no" {
			t.Fatalf("answers[4]: expected 'n' or 'no', got %q", answers[4])
		}
	})

	t.Run("struct-based survey with default branch", func(t *testing.T) {
		in := bytes.NewBufferString("Alice\nBob\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "name1",
				Request: clix.PromptRequest{
					Label: "First name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": PushQuestion("name2"), // Always continue
				},
			},
			{
				ID: "name2",
				Request: clix.PromptRequest{
					Label: "Last name",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "name1")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 2 {
			t.Fatalf("expected 2 answers, got %d: %v", len(answers), answers)
		}
	})

	t.Run("struct-based survey with handler", func(t *testing.T) {
		in := bytes.NewBufferString("yes\nAlice\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		var handlerCalled bool

		questions := []Question{
			{
				ID: "q1",
				Request: clix.PromptRequest{
					Label: "Question 1",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"yes": Handler(func(answer string, s *Survey) {
						handlerCalled = true
						s.AddQuestion(Question{
							ID: "q2",
							Request: clix.PromptRequest{
								Label: "Question 2",
								Theme: clix.DefaultPromptTheme,
							},
							Branches: map[string]Branch{
								"": End(),
							},
						})
						s.Start("q2")
					}),
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "q1")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		if !handlerCalled {
			t.Fatal("handler should have been called")
		}

		answers := s.Answers()
		if len(answers) != 2 {
			t.Fatalf("expected 2 answers, got %d: %v", len(answers), answers)
		}
	})

	t.Run("AddQuestion and AddQuestions work", func(t *testing.T) {
		in := bytes.NewBufferString("Alice\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)

		// Add single question
		s.AddQuestion(Question{
			ID: "name",
			Request: clix.PromptRequest{
				Label: "Name",
				Theme: clix.DefaultPromptTheme,
			},
			Branches: map[string]Branch{
				"": End(),
			},
		})

		s.Start("name")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 1 || answers[0] != "Alice" {
			t.Fatalf("expected ['Alice'], got %v", answers)
		}
	})
}
