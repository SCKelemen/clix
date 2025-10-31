package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"clix"
)

func TestGitHubAuthLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := newApp()
	out := &bytes.Buffer{}
	app.Out = out
	app.Err = &bytes.Buffer{}
	app.In = strings.NewReader("")
	app.Prompter = clix.TerminalPrompter{In: app.In, Out: app.Out}

	if err := app.Run(context.Background(), []string{"auth", "login", "github.com", "monalisa"}); err != nil {
		t.Fatalf("app.Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Logging into github.com as monalisa") {
		t.Fatalf("expected login message in output, got %q", output)
	}
}
