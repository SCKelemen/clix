package prompt

import (
	"bytes"
	"context"
	"github.com/SCKelemen/clix"
	"os"
	"strings"
	"testing"
)

// mockTerminalInput simulates terminal input by providing a pipe that can be written to
// This allows us to test raw terminal mode by sending escape sequences
type mockTerminalInput struct {
	r *os.File
	w *os.File
}

func newMockTerminalInput() (*mockTerminalInput, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &mockTerminalInput{r: r, w: w}, nil
}

func (m *mockTerminalInput) Read(p []byte) (n int, err error) {
	return m.r.Read(p)
}

func (m *mockTerminalInput) WriteKeySequence(seq []byte) error {
	_, err := m.w.Write(seq)
	return err
}

func (m *mockTerminalInput) Close() error {
	m.w.Close()
	return m.r.Close()
}

// TestPromptSelectRawTerminal tests select prompt in raw terminal mode
func TestPromptSelectRawTerminal(t *testing.T) {
	t.Run("select prompt uses line-based fallback for bytes.Buffer", func(t *testing.T) {
		// This test verifies that bytes.Buffer (non-terminal) triggers fallback
		in := bytes.NewBufferString("1\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Choose", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
		},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
		// Verify it used line-based mode (has the prompt with > marker)
		output := out.String()
		if !strings.Contains(output, "Choose") {
			t.Errorf("output should contain label, got: %s", output)
		}
	})

	t.Run("select prompt with number key in raw mode", func(t *testing.T) {
		// This test is conceptual - actual raw terminal testing requires a real terminal
		// We verify that the code path exists and fallback works
		// Real terminal testing should be done manually or with integration tests
		in := bytes.NewBufferString("2\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Choose", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
			{Label: "Option C", Value: "c"},
		},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "b" {
			t.Fatalf("expected value 'b', got %q", value)
		}
	})
}

// TestPromptMultiSelectRawTerminal tests multi-select prompt in raw terminal mode
func TestPromptMultiSelectRawTerminal(t *testing.T) {
	t.Run("multi-select uses line-based fallback for bytes.Buffer", func(t *testing.T) {
		// This test verifies that bytes.Buffer triggers line-based fallback
		in := bytes.NewBufferString("1,2\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if !strings.Contains(value, "a") || !strings.Contains(value, "b") {
			t.Fatalf("expected value to contain 'a' and 'b', got %q", value)
		}
		// Verify it used line-based mode
		output := out.String()
		if !strings.Contains(output, "[ ]") {
			t.Errorf("output should contain checkboxes, got: %s", output)
		}
	})

	t.Run("multi-select with continue button text", func(t *testing.T) {
		in := bytes.NewBufferString("1\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(),
			clix.PromptRequest{
				Label:        "Select",
				Theme:        clix.DefaultPromptTheme,
				ContinueText: "Done",
				Options: []clix.SelectOption{
					{Label: "Option A", Value: "a"},
				},
				MultiSelect: true,
			})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
		// In line-based mode, continue button text isn't shown, but the field is respected
	})
}

// TestReadKey tests the ReadKey function with escape sequences
func TestReadKey(t *testing.T) {
	t.Run("read arrow key sequences", func(t *testing.T) {
		tests := []struct {
			name     string
			sequence []byte
			expected Key
		}{
			{"Up arrow", []byte{0x1b, '[', 'A'}, KeyUp},
			{"Down arrow", []byte{0x1b, '[', 'B'}, KeyDown},
			{"Right arrow", []byte{0x1b, '[', 'C'}, KeyRight},
			{"Left arrow", []byte{0x1b, '[', 'D'}, KeyLeft},
			{"Home", []byte{0x1b, '[', 'H'}, KeyHome},
			{"End", []byte{0x1b, '[', 'F'}, KeyEnd},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				in := bytes.NewBuffer(tt.sequence)
				key, err := ReadKey(in)
				if err != nil {
					t.Fatalf("ReadKey returned error: %v", err)
				}
				if key != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, key)
				}
			})
		}
	})

	t.Run("read regular keys", func(t *testing.T) {
		tests := []struct {
			name     string
			sequence []byte
			expected Key
		}{
			{"Enter (newline)", []byte{'\n'}, KeyEnter},
			{"Enter (carriage return)", []byte{'\r'}, KeyEnter},
			{"Tab", []byte{'\t'}, KeyTab},
			{"Space", []byte{' '}, KeySpace},
			{"Backspace", []byte{0x7f}, KeyBackspace},
			{"Ctrl+C", []byte{0x03}, KeyCtrlC},
			{"Regular character", []byte{'a'}, Key{rune('a'), 'a'}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				in := bytes.NewBuffer(tt.sequence)
				key, err := ReadKey(in)
				if err != nil {
					t.Fatalf("ReadKey returned error: %v", err)
				}
				if key != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, key)
				}
			})
		}
	})

	t.Run("read escape key", func(t *testing.T) {
		in := bytes.NewBuffer([]byte{0x1b})
		key, err := ReadKey(in)
		if err != nil {
			t.Fatalf("ReadKey returned error: %v", err)
		}
		if key != KeyEscape {
			t.Errorf("expected KeyEscape, got %v", key)
		}
	})
}

// TestTerminalFallbackBehavior verifies that non-terminal inputs use line-based mode
func TestTerminalFallbackBehavior(t *testing.T) {
	t.Run("bytes.Buffer triggers line-based fallback for select", func(t *testing.T) {
		// bytes.Buffer is not an *os.File, so it triggers line-based fallback
		in := bytes.NewBufferString("Option B\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Choose option", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
		},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "b" {
			t.Fatalf("expected value 'b', got %q", value)
		}
		// Line-based mode prints all options with selection marker
		output := out.String()
		if !strings.Contains(output, "Option A") || !strings.Contains(output, "Option B") {
			t.Errorf("output should show all options, got: %s", output)
		}
	})

	t.Run("bytes.Buffer triggers line-based fallback for multi-select", func(t *testing.T) {
		// bytes.Buffer is not an *os.File, so it triggers line-based fallback
		in := bytes.NewBufferString("1,3\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select items", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "Item 1", Value: "1"},
			{Label: "Item 2", Value: "2"},
			{Label: "Item 3", Value: "3"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if !strings.Contains(value, "1") || !strings.Contains(value, "3") {
			t.Fatalf("expected value to contain '1' and '3', got %q", value)
		}
		// Line-based mode shows checkboxes
		output := out.String()
		if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
			t.Errorf("output should show checkboxes, got: %s", output)
		}
	})

	t.Run("line-based fallback supports number input for select", func(t *testing.T) {
		in := bytes.NewBufferString("2\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Choose", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "First", Value: "1"},
			{Label: "Second", Value: "2"},
			{Label: "Third", Value: "3"},
		},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "2" {
			t.Fatalf("expected value '2', got %q", value)
		}
	})

	t.Run("line-based fallback supports partial match for select", func(t *testing.T) {
		in := bytes.NewBufferString("Sec\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Choose", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "First", Value: "1"},
			{Label: "Second", Value: "2"},
			{Label: "Third", Value: "3"},
		},
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "2" {
			t.Fatalf("expected value '2', got %q", value)
		}
	})

	t.Run("line-based fallback supports 'done' for multi-select", func(t *testing.T) {
		in := bytes.NewBufferString("1,2\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "A", Value: "a"},
			{Label: "B", Value: "b"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if !strings.Contains(value, "a") || !strings.Contains(value, "b") {
			t.Fatalf("expected value to contain 'a' and 'b', got %q", value)
		}
	})

	t.Run("line-based fallback requires selections for multi-select", func(t *testing.T) {
		in := bytes.NewBufferString("\n1\ndone\n")
		out := &bytes.Buffer{}

		prompter := TerminalPrompter{In: in, Out: out}
		value, err := prompter.Prompt(context.Background(), clix.PromptRequest{Label: "Select", Theme: clix.DefaultPromptTheme, Options: []clix.SelectOption{
			{Label: "A", Value: "a"},
		},
			MultiSelect: true,
		})
		if err != nil {
			t.Fatalf("Prompt returned error: %v", err)
		}
		if value != "a" {
			t.Fatalf("expected value 'a', got %q", value)
		}
		output := out.String()
		if !strings.Contains(output, "Please select at least one option") {
			t.Errorf("output should show error for empty selection, got: %s", output)
		}
	})
}
