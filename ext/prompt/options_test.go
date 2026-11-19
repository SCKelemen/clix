package prompt_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/prompt"
)

// Select(nil) is an empty select, not a free-text prompt, and Select after
// MultiSelect is a single select.
func TestSelectOptionApply(t *testing.T) {
	for _, opt := range []clix.PromptOption{prompt.Select(nil), prompt.MultiSelect(nil)} {
		tp := prompt.TerminalPrompter{In: strings.NewReader("anything\n"), Out: &bytes.Buffer{}}
		if _, err := tp.Prompt(context.Background(), clix.WithLabel("Region"), opt); !errors.Is(err, clix.ErrNoOptions) {
			t.Errorf("err = %v, want ErrNoOptions", err)
		}
	}
	out := &bytes.Buffer{}
	tp := prompt.TerminalPrompter{In: strings.NewReader("2\n"), Out: out}
	got, err := tp.Prompt(context.Background(), clix.WithLabel("Env"), prompt.MultiSelect(envs), prompt.Select(envs))
	if err != nil || got != "production" || strings.Contains(out.String(), "[ ]") {
		t.Fatalf("got %q, %v, output %q", got, err, out.String())
	}
	var p clix.Prompter = &prompt.TerminalPrompter{}
	if prompt.AsTerminalPrompter(p) == nil {
		t.Fatal("AsTerminalPrompter(*TerminalPrompter) = nil")
	}
}
