package survey

import (
	"github.com/SCKelemen/clix/v2"
	"context"
	"testing"
)

// mockPrompterForBindings is a prompter that records what key bindings were passed to it
type mockPrompterForBindings struct {
	keyBindings []clix.PromptKeyBinding
	callCount   int
	answers     []string
}

func (m *mockPrompterForBindings) Prompt(ctx context.Context, opts ...clix.PromptOption) (string, error) {
	m.callCount++
	// Extract key bindings from options by building a config
	cfg := &clix.PromptConfig{}
	for _, opt := range opts {
		opt.Apply(cfg)
	}
	// Check if key map is configured (has bindings)
	if len(cfg.KeyMap.Bindings) > 0 {
		m.keyBindings = cfg.KeyMap.Bindings
	}

	// Return next answer
	if len(m.answers) > 0 {
		answer := m.answers[0]
		m.answers = m.answers[1:]
		return answer, nil
	}
	return "", nil
}

func (m *mockPrompterForBindings) Out() interface{} {
	return nil
}

func TestSurveyKeyBindings(t *testing.T) {
	t.Run("undo stack adds Escape and F12 bindings", func(t *testing.T) {
		prompter := &mockPrompterForBindings{
			answers: []string{"Alice"},
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

		// Check that key bindings were added
		if len(prompter.keyBindings) == 0 {
			t.Fatal("expected key bindings to be added, but none were found")
		}

		// Check for Escape binding
		hasEscape := false
		hasF12 := false
		for _, binding := range prompter.keyBindings {
			if binding.Command.Type == clix.PromptCommandEscape {
				hasEscape = true
				if binding.Description != "Back" {
					t.Errorf("expected Escape binding description 'Back', got %q", binding.Description)
				}
			}
			if binding.Command.Type == clix.PromptCommandFunction && binding.Command.FunctionKey == 12 {
				hasF12 = true
				if binding.Description != "Back" {
					t.Errorf("expected F12 binding description 'Back', got %q", binding.Description)
				}
			}
		}

		if !hasEscape {
			t.Error("expected Escape key binding to be added")
		}
		if !hasF12 {
			t.Error("expected F12 key binding to be added")
		}
	})

	t.Run("key bindings only added when undo stack is enabled", func(t *testing.T) {
		prompter := &mockPrompterForBindings{
			answers: []string{"Alice"},
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

		s := NewFromQuestions(ctx, prompter, questions, "name") // No WithUndoStack()
		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		// Check that no undo key bindings were added
		hasUndoBinding := false
		for _, binding := range prompter.keyBindings {
			if binding.Command.Type == clix.PromptCommandEscape ||
				(binding.Command.Type == clix.PromptCommandFunction && binding.Command.FunctionKey == 12) {
				hasUndoBinding = true
				break
			}
		}

		if hasUndoBinding {
			t.Error("expected no undo key bindings when undo stack is not enabled")
		}
	})

	t.Run("key bindings are inactive when canGoBack is false", func(t *testing.T) {
		prompter := &mockPrompterForBindings{
			answers: []string{"Alice"},
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

		// Find Escape binding
		var escapeBinding *clix.PromptKeyBinding
		for i := range prompter.keyBindings {
			if prompter.keyBindings[i].Command.Type == clix.PromptCommandEscape {
				escapeBinding = &prompter.keyBindings[i]
				break
			}
		}

		if escapeBinding == nil {
			t.Fatal("expected Escape binding to be present")
		}

		// On first question, canGoBack should be false (no history)
		if escapeBinding.Active == nil {
			t.Error("expected Active function to be set for Escape binding")
		} else {
			// Test with empty state (first question, no history)
			state := clix.PromptKeyState{}
			active := escapeBinding.Active(state)
			if active {
				t.Error("expected Escape binding to be inactive on first question (no history)")
			}
		}
	})

	t.Run("text prompts get Tab and Enter bindings", func(t *testing.T) {
		prompter := &mockPrompterForBindings{
			answers: []string{"Alice"},
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

		s := NewFromQuestions(ctx, prompter, questions, "name")
		if err := s.Run(); err != nil {
			t.Fatalf("survey failed: %v", err)
		}

		// Check for Tab and Enter bindings
		hasTab := false
		hasEnter := false
		for _, binding := range prompter.keyBindings {
			if binding.Command.Type == clix.PromptCommandTab {
				hasTab = true
				if binding.Description != "Autocomplete" {
					t.Errorf("expected Tab binding description 'Autocomplete', got %q", binding.Description)
				}
			}
			if binding.Command.Type == clix.PromptCommandEnter {
				hasEnter = true
				if binding.Description != "Submit" {
					t.Errorf("expected Enter binding description 'Submit', got %q", binding.Description)
				}
			}
		}

		if !hasTab {
			t.Error("expected Tab key binding to be added for text prompts")
		}
		if !hasEnter {
			t.Error("expected Enter key binding to be added for text prompts")
		}
	})
}
