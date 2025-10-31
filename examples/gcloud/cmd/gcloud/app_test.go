package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"clix"
)

func TestGcloudProjectsList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := newApp()
	out := &bytes.Buffer{}
	app.Out = out
	app.Err = &bytes.Buffer{}
	app.In = strings.NewReader("")
	app.Prompter = clix.SimpleTextPrompter{In: app.In, Out: app.Out}

	if err := app.Run(context.Background(), []string{"projects", "list"}); err != nil {
		t.Fatalf("app.Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "PROJECT_ID") {
		t.Fatalf("expected header in output, got %q", output)
	}
}
