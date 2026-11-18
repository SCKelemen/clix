package clix

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPromptRepromptsOnInvalidAnswer(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	var replicas int
	var zone string
	cmd := NewCommand("scale", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.IntVar(IntVarOptions{FlagOptions: FlagOptions{Name: "replicas", Required: true}, Value: &replicas})
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{
		Name: "zone", Required: true,
		Validate: func(s string) error {
			if strings.Contains(s, " ") {
				return errors.New("zone must not contain spaces")
			}
			return nil
		},
	}, Value: &zone})
	app.Root.AddCommand(cmd)

	promptOut := &bytes.Buffer{}
	app.Prompter = TextPrompter{In: strings.NewReader("three\n3\nus east\nus-east\n"), Out: promptOut}

	if err := app.Run(context.Background(), []string{"scale"}); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if replicas != 3 || zone != "us-east" {
		t.Fatalf("replicas=%d zone=%q, want 3 and us-east after re-prompting", replicas, zone)
	}
	if !strings.Contains(promptOut.String(), "invalid syntax") || !strings.Contains(promptOut.String(), "must not contain spaces") {
		t.Fatalf("validation errors not shown to the user: %q", promptOut.String())
	}
}

func TestPromptReportsInvalidAnswerFromCustomPrompter(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	var replicas int
	cmd := NewCommand("scale", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.IntVar(IntVarOptions{FlagOptions: FlagOptions{Name: "replicas", Required: true}, Value: &replicas})
	app.Root.AddCommand(cmd)
	app.Prompter = prompterFunc(func(context.Context, ...PromptOption) (string, error) { return "three", nil })

	err := app.Run(context.Background(), []string{"scale"})
	if err == nil || !strings.Contains(err.Error(), "--replicas") {
		t.Fatalf("Run error = %v, want an error naming --replicas", err)
	}
}

func TestPromptReportsExhaustedInput(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	var replicas int
	cmd := NewCommand("scale", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.IntVar(IntVarOptions{FlagOptions: FlagOptions{Name: "replicas", Required: true}, Value: &replicas})
	app.Root.AddCommand(cmd)
	app.Prompter = TextPrompter{In: strings.NewReader("three\n"), Out: &bytes.Buffer{}}

	err := app.Run(context.Background(), []string{"scale"})
	if !errors.Is(err, io.EOF) || !strings.Contains(err.Error(), "--replicas") {
		t.Fatalf("Run error = %v, want EOF naming --replicas", err)
	}
}
