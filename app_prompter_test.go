package clix

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDefaultPrompterUsesAppStreamsFromOptions(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	in := strings.NewReader("x\n")
	out := &bytes.Buffer{}
	app := NewApp("moln", WithAppIn(in), WithAppOut(out))

	tp, ok := app.Prompter.(TextPrompter)
	if !ok {
		t.Fatalf("default prompter is %T, want TextPrompter", app.Prompter)
	}
	if tp.In != in {
		t.Fatal("default prompter ignores WithAppIn")
	}
	if tp.Out != app.Err {
		t.Fatal("default prompter does not write prompts to the app's error stream")
	}
}

func TestWithAppPrompterIsKept(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	custom := prompterFunc(func(context.Context, ...PromptOption) (string, error) { return "", nil })
	app := NewApp("moln", WithAppPrompter(custom), WithAppIn(strings.NewReader("")))
	if _, ok := app.Prompter.(prompterFunc); !ok {
		t.Fatalf("Prompter = %T, want the prompter passed to WithAppPrompter", app.Prompter)
	}
}
