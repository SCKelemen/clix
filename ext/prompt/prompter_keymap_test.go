package prompt

import (
	"testing"

	"clix"
)

func TestRenderHintLine_TextBindings(t *testing.T) {
	cfg := &clix.PromptConfig{
		Theme: clix.PromptTheme{
			Buttons: clix.PromptButtonStyles{
				Active: clix.StyleFunc(func(parts ...string) string {
					return "A(" + parts[0] + ")"
				}),
				Inactive: clix.StyleFunc(func(parts ...string) string {
					return "I(" + parts[0] + ")"
				}),
			},
		},
		KeyMap: clix.PromptKeyMap{
			Bindings: []clix.PromptKeyBinding{
				{
					Command:     clix.PromptCommand{Type: clix.PromptCommandTab},
					Description: "Autocomplete",
					Active: func(state clix.PromptKeyState) bool {
						return state.Default != "" && state.Suggestion != ""
					},
				},
				{
					Command:     clix.PromptCommand{Type: clix.PromptCommandEnter},
					Description: "Submit",
				},
			},
		},
	}

	got := renderHintLine(cfg, clix.PromptKeyState{Default: "value", Suggestion: "value"})
	want := "A([ Tab ] Autocomplete)    A([ Enter ] Submit)"
	if got != want {
		t.Fatalf("renderHintLine() = %q, want %q", got, want)
	}

	got = renderHintLine(cfg, clix.PromptKeyState{Default: "", Suggestion: ""})
	want = "I([ Tab ] Autocomplete)    A([ Enter ] Submit)"
	if got != want {
		t.Fatalf("renderHintLine() inactive = %q, want %q", got, want)
	}
}

func TestDispatchCommandUsesBinding(t *testing.T) {
	cfg := &clix.PromptConfig{
		KeyMap: clix.PromptKeyMap{
			Bindings: []clix.PromptKeyBinding{
				{
					Command: clix.PromptCommand{Type: clix.PromptCommandEscape},
					Handler: func(ctx clix.PromptCommandContext) clix.PromptCommandAction {
						if ctx.Command.Type != clix.PromptCommandEscape {
							t.Fatalf("unexpected command: %+v", ctx.Command)
						}
						ctx.SetInput("updated")
						return clix.PromptCommandAction{Handled: true}
					},
				},
			},
		},
	}

	input := "start"
	action := dispatchCommand(cfg, clix.PromptKeyState{Command: clix.PromptCommand{Type: clix.PromptCommandEscape}}, func(v string) {
		input = v
	})

	if !action.Handled {
		t.Fatalf("expected handled action")
	}
	if input != "updated" {
		t.Fatalf("expected input to be updated, got %q", input)
	}
}

func TestDispatchCommandInactiveBinding(t *testing.T) {
	called := false
	cfg := &clix.PromptConfig{
		KeyMap: clix.PromptKeyMap{
			Bindings: []clix.PromptKeyBinding{
				{
					Command: clix.PromptCommand{Type: clix.PromptCommandEscape},
					Handler: func(ctx clix.PromptCommandContext) clix.PromptCommandAction {
						called = true
						return clix.PromptCommandAction{Handled: true}
					},
					Active: func(clix.PromptKeyState) bool { return false },
				},
			},
		},
	}

	action := dispatchCommand(cfg, clix.PromptKeyState{Command: clix.PromptCommand{Type: clix.PromptCommandEscape}}, nil)
	if !action.Handled {
		t.Fatalf("expected inactive binding to consume event")
	}
	if called {
		t.Fatalf("inactive binding handler should not be called")
	}
}
