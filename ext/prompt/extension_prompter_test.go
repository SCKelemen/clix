package prompt

import (
	"bytes"
	"context"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

type fixedPrompter string

func (p fixedPrompter) Prompt(context.Context, ...clix.PromptOption) (string, error) {
	return string(p), nil
}

func TestExtensionKeepsUserPrompter(t *testing.T) {
	app := clix.NewApp("test", clix.WithAppPrompter(fixedPrompter("mine")))
	if err := (Extension{}).Extend(app); err != nil {
		t.Fatal(err)
	}
	if _, ok := app.Prompter.(fixedPrompter); !ok {
		t.Fatalf("Prompter = %T, want the user's prompter to be kept", app.Prompter)
	}
}

func TestExtensionPromptsToErr(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app := clix.NewApp("test", clix.WithAppOut(out), clix.WithAppErr(errOut), clix.WithAppIn(bytes.NewBufferString("v\n")))
	if err := (Extension{}).Extend(app); err != nil {
		t.Fatal(err)
	}
	tp, ok := app.Prompter.(TerminalPrompter)
	if !ok {
		t.Fatalf("Prompter = %T, want TerminalPrompter", app.Prompter)
	}
	if tp.Out != errOut {
		t.Fatal("TerminalPrompter does not write to App.Err")
	}
	if _, err := tp.Prompt(context.Background(), clix.WithLabel("Name")); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || errOut.Len() == 0 {
		t.Fatalf("stdout=%q stderr=%q; prompts belong on stderr", out.String(), errOut.String())
	}
}
