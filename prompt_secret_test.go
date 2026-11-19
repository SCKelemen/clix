package clix_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// A secret is not trimmed, its default is never shown, and a sensitive
// required flag is prompted for as a secret.
func TestSecretPrompts(t *testing.T) {
	out := &bytes.Buffer{}
	tp := clix.TextPrompter{In: strings.NewReader("  s3cret \r\n"), Out: out}
	got, err := tp.Prompt(context.Background(), clix.WithLabel("Passphrase"), clix.WithSecret(), clix.WithDefault("old"))
	if err != nil || got != "  s3cret " {
		t.Fatalf("got %q, %v", got, err)
	}
	if strings.Contains(out.String(), "old") {
		t.Fatalf("secret default shown: %q", out.String())
	}
	tp = clix.TextPrompter{In: strings.NewReader("\n"), Out: io.Discard}
	if got, _ := tp.Prompt(context.Background(), clix.WithLabel("Passphrase"), clix.PromptRequest{Secret: true, Default: " keep "}); got != " keep " {
		t.Fatalf("secret default trimmed: %q", got)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var token string
	app := clix.NewApp("probe", clix.WithAppIn(strings.NewReader(" tok \n")), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppOut(&bytes.Buffer{}))
	app.Interactive = clix.InteractiveAlways
	cmd := clix.NewCommand("login", clix.WithCommandRun(func(*clix.Context) error { return nil }))
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "token", Required: true, Sensitive: true}, Value: &token})
	app.Root.AddCommand(cmd)
	if err := app.Run(context.Background(), []string{"login"}); err != nil || token != " tok " {
		t.Fatalf("token %q, %v", token, err)
	}
	var fo clix.FlagOptions
	clix.WithFlagSensitive().ApplyFlag(&fo)
	if !fo.Sensitive {
		t.Fatal("WithFlagSensitive")
	}
}
