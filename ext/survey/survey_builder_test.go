package survey

import (
	"bytes"
	"github.com/SCKelemen/clix"
	"context"
	"strings"
	"testing"
)

func TestQuestionBuilder(t *testing.T) {
	t.Run("ThenFunc adds handler branch", func(t *testing.T) {
		handlerCalled := false
		handlerAnswer := ""

		in := bytes.NewBufferString("test\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		questions := []Question{
			{
				ID: "q1",
				Request: clix.PromptRequest{
					Label: "Question 1",
					Theme: clix.DefaultPromptTheme,
				},
				Branches: map[string]Branch{
					"": HandlerBranch{
						Handler: func(answer string, s *Survey) {
							handlerCalled = true
							handlerAnswer = answer
						},
					},
				},
			},
		}

		s := NewFromQuestions(ctx, prompter, questions, "q1")
		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		if !handlerCalled {
			t.Error("expected handler to be called")
		}
		if handlerAnswer != "test" {
			t.Errorf("expected handler to receive 'test', got %q", handlerAnswer)
		}
	})

	t.Run("ThenFunc using builder pattern", func(t *testing.T) {
		handlerCalled := false

		in := bytes.NewBufferString("test\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		s := New(ctx, prompter)
		builder := s.Question("q1", clix.PromptRequest{
			Label: "Question 1",
			Theme: clix.DefaultPromptTheme,
		})
		builder.ThenFunc(func(answer string, s *Survey) {
			handlerCalled = true
		})

		// Start the survey with the first question
		s.Start("q1")

		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		if !handlerCalled {
			t.Error("expected handler to be called")
		}
	})
}

func TestRenderText(t *testing.T) {
	// renderText is a private helper, but we can test it indirectly through renderSummary
	t.Run("renderSummary uses renderText for styling", func(t *testing.T) {
		in := bytes.NewBufferString("Alice\nBob\ny\n")
		out := &bytes.Buffer{}

		prompter := clix.TextPrompter{In: in, Out: out}
		ctx := context.Background()

		theme := clix.DefaultPromptTheme
		theme.LabelStyle = clix.StyleFunc(func(parts ...string) string {
			return "[" + strings.Join(parts, "") + "]"
		})
		theme.DefaultStyle = clix.StyleFunc(func(parts ...string) string {
			return "{" + strings.Join(parts, "") + "}"
		})

		questions := []Question{
			{
				ID: "name1",
				Request: clix.PromptRequest{
					Label: "Name 1",
					Theme: theme,
				},
				Branches: map[string]Branch{
					"": PushQuestion("name2"),
				},
			},
			{
				ID: "name2",
				Request: clix.PromptRequest{
					Label: "Name 2",
					Theme: theme,
				},
				Branches: map[string]Branch{
					"": End(),
				},
			},
		}

		endCardTheme := theme

		s := NewFromQuestions(ctx, prompter, questions, "name1", WithEndCard(), WithEndCardTheme(endCardTheme))
		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		output := out.String()
		// Check that styled text appears in summary (indirect test of renderText)
		if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
			t.Error("expected styled labels in summary output")
		}
	})
}
