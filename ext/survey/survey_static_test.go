package survey

import (
	"bytes"
	"github.com/SCKelemen/clix/v2"
	"context"
	"testing"
)

func TestStaticSurvey(t *testing.T) {
	t.Run("static survey with simple branches", func(t *testing.T) {
		in := bytes.NewBufferString("yes\nAlice\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		s.Question("add-child", clix.PromptRequest{
			Label: "Do you want to add a child?",
			Theme: clix.DefaultPromptTheme,
		}).
			If("yes", QuestionBranch{QuestionID: "child-name"}).
			If("no", EndBranch{})

		s.Question("child-name", clix.PromptRequest{
			Label: "Child's name",
			Theme: clix.DefaultPromptTheme,
		}).
			End()

		s.Start("add-child")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 2 {
			t.Fatalf("expected 2 answers, got %d: %v", len(answers), answers)
		}
		if answers[0] != "yes" || answers[1] != "Alice" {
			t.Fatalf("expected answers ['yes', 'Alice'], got %v", answers)
		}
	})

	t.Run("static survey with loop", func(t *testing.T) {
		in := bytes.NewBufferString("yes\nAlice\nyes\nBob\nno\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)

		// Define questions statically
		s.Question("add-child", clix.PromptRequest{
			Label: "Do you want to add a child?",
			Theme: clix.DefaultPromptTheme,
		}).
			If("yes", QuestionBranch{QuestionID: "child-name"}).
			If("no", EndBranch{})

		s.Question("child-name", clix.PromptRequest{
			Label: "Child's name",
			Theme: clix.DefaultPromptTheme,
		}).
			Then("add-another")

		s.Question("add-another", clix.PromptRequest{
			Label: "Do you want to add another child?",
			Theme: clix.DefaultPromptTheme,
		}).
			If("yes", QuestionBranch{QuestionID: "child-name"}). // Loop back to child-name
			If("no", EndBranch{})

		s.Start("add-child")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		// Should have: yes, Alice, yes, Bob, no
		if len(answers) != 5 {
			t.Fatalf("expected 5 answers, got %d: %v", len(answers), answers)
		}
		expected := []string{"yes", "Alice", "yes", "Bob", "no"}
		for i, exp := range expected {
			if answers[i] != exp {
				t.Fatalf("answers[%d]: expected %q, got %q", i, exp, answers[i])
			}
		}
	})

	t.Run("static survey with default branch", func(t *testing.T) {
		in := bytes.NewBufferString("Alice\nBob\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)

		s.Question("name1", clix.PromptRequest{
			Label: "First name",
			Theme: clix.DefaultPromptTheme,
		}).
			Then("name2") // Always continue to name2

		s.Question("name2", clix.PromptRequest{
			Label: "Last name",
			Theme: clix.DefaultPromptTheme,
		}).
			End()

		s.Start("name1")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 2 {
			t.Fatalf("expected 2 answers, got %d: %v", len(answers), answers)
		}
	})

	t.Run("static survey with mixed branches", func(t *testing.T) {
		in := bytes.NewBufferString("option1\nAlice\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)

		s.Question("choose", clix.PromptRequest{
			Label: "Choose option",
			Theme: clix.DefaultPromptTheme,
		}).
			If("option1", QuestionBranch{QuestionID: "option1-detail"}).
			If("option2", QuestionBranch{QuestionID: "option2-detail"}).
			End()

		s.Question("option1-detail", clix.PromptRequest{
			Label: "Option 1 detail",
			Theme: clix.DefaultPromptTheme,
		}).
			End()

		s.Question("option2-detail", clix.PromptRequest{
			Label: "Option 2 detail",
			Theme: clix.DefaultPromptTheme,
		}).
			End()

		s.Start("choose")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		answers := s.Answers()
		if len(answers) != 2 {
			t.Fatalf("expected 2 answers, got %d: %v", len(answers), answers)
		}
		if answers[0] != "option1" || answers[1] != "Alice" {
			t.Fatalf("expected answers ['option1', 'Alice'], got %v", answers)
		}
	})

	t.Run("static survey with handler branches", func(t *testing.T) {
		in := bytes.NewBufferString("yes\nAlice\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		var handlerCalled bool
		s := New(ctx, prompter)

		s.Question("q1", clix.PromptRequest{
			Label: "Question 1",
			Theme: clix.DefaultPromptTheme,
		}).
			If("yes", HandlerBranch{
				Handler: func(answer string, s *Survey) {
					handlerCalled = true
					s.Question("q2", clix.PromptRequest{
						Label: "Question 2",
						Theme: clix.DefaultPromptTheme,
					}).End()
					s.Start("q2")
				},
			})

		s.Start("q1")

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
}

