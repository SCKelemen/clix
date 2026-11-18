package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func TestGcloudProjectsList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := newApp()
	out := &bytes.Buffer{}
	app.Out = out
	app.Err = &bytes.Buffer{}
	app.In = strings.NewReader("")
	app.Prompter = clix.TextPrompter{In: app.In, Out: app.Out}

	if err := app.Run(context.Background(), []string{"projects", "list"}); err != nil {
		t.Fatalf("app.Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "PROJECT_ID") {
		t.Fatalf("expected header in output, got %q", output)
	}
}

func TestGcloudProjectFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := newApp()
	app.Out = &bytes.Buffer{}
	app.Err = &bytes.Buffer{}

	// --project is registered on the installed root, so it is accepted.
	if err := app.Run(context.Background(), []string{"--project", "my-project", "projects", "list"}); err != nil {
		t.Fatalf("app.Run returned error: %v", err)
	}
}
