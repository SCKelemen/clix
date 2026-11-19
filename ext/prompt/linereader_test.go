package prompt

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func TestLineBasedPromptsShareInput(t *testing.T) {
	in := strings.NewReader("alice\n2\ny\n1,2\ndone\nleftover")
	p := TerminalPrompter{In: in, Out: &bytes.Buffer{}}
	ctx := context.Background()
	opts := []clix.SelectOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}

	if got, err := p.Prompt(ctx, clix.WithLabel("Name")); err != nil || got != "alice" {
		t.Fatalf("text = %q, %v", got, err)
	}
	if got, err := p.Prompt(ctx, clix.WithLabel("Pick"), Select(opts)); err != nil || got != "b" {
		t.Fatalf("select = %q, %v", got, err)
	}
	if got, err := p.Prompt(ctx, clix.WithLabel("Sure?"), Confirm()); err != nil || got != "y" {
		t.Fatalf("confirm = %q, %v", got, err)
	}
	if got, err := p.Prompt(ctx, clix.WithLabel("Some"), MultiSelect(opts)); err != nil || got != "a,b" {
		t.Fatalf("multi-select = %q, %v", got, err)
	}
	rest, _ := io.ReadAll(in)
	if string(rest) != "leftover" {
		t.Fatalf("prompts consumed input meant for later: rest = %q", rest)
	}
}
