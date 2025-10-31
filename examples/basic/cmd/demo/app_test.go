package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"clix"
	demoapp "clix/examples/basic/internal/app"
)

func TestDemoGreetCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := demoapp.New()
	out := &bytes.Buffer{}
	app.Out = out
	app.Err = &bytes.Buffer{}
	app.In = strings.NewReader("")
	app.Prompter = clix.TerminalPrompter{In: app.In, Out: app.Out}

	if err := app.Run(context.Background(), []string{"greet", "Alice"}); err != nil {
		t.Fatalf("app.Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Using project sample-project") {
		t.Fatalf("expected project information in output, got %q", output)
	}
	if !strings.Contains(output, "Hello Alice!") {
		t.Fatalf("expected greeting in output, got %q", output)
	}
	if !strings.Contains(output, "All done!") {
		t.Fatalf("expected post-run message in output, got %q", output)
	}
}
