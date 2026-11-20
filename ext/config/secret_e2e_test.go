package config

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// scriptedPrompter answers prompts in order and records each request.
type scriptedPrompter struct {
	answers  []string
	requests []clix.PromptConfig
}

func (p *scriptedPrompter) Prompt(_ context.Context, opts ...clix.PromptOption) (string, error) {
	var cfg clix.PromptConfig
	for _, o := range opts {
		o.Apply(&cfg)
	}
	p.requests = append(p.requests, cfg)
	if len(p.answers) == 0 {
		return "", io.EOF
	}
	a := p.answers[0]
	p.answers = p.answers[1:]
	return a, nil
}

// secretApp has a Sensitive root --token flag and a Sensitive --pin int.
func secretApp(t *testing.T, mode clix.InteractiveMode, prompter clix.Prompter) (*clix.App, *bytes.Buffer, string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PROBE_TOKEN", "")
	t.Setenv("PROBE_PIN", "")
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}), clix.WithAppInteractive(mode))
	app.Prompter = prompter
	path := filepath.Join(t.TempDir(), "config.yaml")
	app.ConfigPath = path
	app.AddExtension(Extension{})
	app.AddExtension(format.Extension{})
	var token string
	var pin int
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "token", Usage: "API token", Sensitive: true}, Value: &token})
	app.Flags().IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "pin", Sensitive: true}, Value: &pin})
	return app, out, path
}

// A secret stored with "config set" is never echoed: set prints "token
// set", list shows [redacted], and get refuses without --reveal.
func TestConfigE2E_SecretsAreNotPrinted(t *testing.T) {
	app, out, path := secretApp(t, clix.InteractiveNever, nil)
	if got := run(t, app, out, "config", "set", "token", "s3cr3t"); got != "token set\n" {
		t.Errorf("config set token printed %q", got)
	}
	run(t, app, out, "config", "set", "name", "visible")
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "s3cr3t") {
		t.Fatalf("the token was not stored:\n%s", data)
	}
	if got := run(t, app, out, "config", "list"); got != "name: visible\ntoken: '[redacted]'\n" {
		t.Errorf("config list:\n%s", got)
	}
	if got := run(t, app, out, "config", "list", "--format", "json"); strings.Contains(got, "s3cr3t") || !strings.Contains(got, `"token": "[redacted]"`) {
		t.Errorf("config list --format json:\n%s", got)
	}
	got, err := tryRun(app, out, "config", "get", "token")
	// Without --reveal the call is incomplete: a usage error.
	if err == nil || got != "" || strings.Contains(err.Error(), "s3cr3t") || !strings.Contains(err.Error(), "--reveal") || clix.ExitCode(err) != 2 {
		t.Errorf("config get token: out %q, err %v", got, err)
	}
	if got := run(t, app, out, "config", "get", "token", "--reveal"); got != "s3cr3t\n" {
		t.Errorf("config get token --reveal: %q", got)
	}
	if got := run(t, app, out, "config", "get", "name"); got != "visible\n" {
		t.Errorf("config get name: %q", got)
	}
	// An invalid secret is refused without showing it.
	_, err = tryRun(app, out, "config", "set", "pin", "abc123")
	if err == nil || strings.Contains(err.Error(), "abc123") || !strings.Contains(err.Error(), "--pin") {
		t.Errorf("config set pin abc123: %v", err)
	}
	// PROBE_REVEAL must not reveal anything.
	t.Setenv("PROBE_REVEAL", "true")
	if _, err := tryRun(app, out, "config", "get", "token"); err == nil {
		t.Error("PROBE_REVEAL=true revealed the token")
	}
}

// Interactive "config set" asks for the value with echo off when the key
// holds a secret.
func TestConfigE2E_SetPromptsForSecrets(t *testing.T) {
	for _, tc := range []struct {
		key    string
		secret bool
		want   string
	}{
		{"token", true, "token set\n"},
		{"region", false, "region = eu\n"},
	} {
		p := &scriptedPrompter{answers: []string{tc.key, "eu"}}
		app, out, _ := secretApp(t, clix.InteractiveAlways, p)
		if got := run(t, app, out, "config", "set"); got != tc.want {
			t.Errorf("config set (%s): printed %q", tc.key, got)
		}
		if len(p.requests) != 2 || p.requests[1].Secret != tc.secret || p.requests[0].Secret {
			t.Errorf("config set (%s): prompts %+v", tc.key, p.requests)
		}
	}

	// A scripted "config set key" without a value is an error, not a prompt.
	p := &scriptedPrompter{}
	app, out, _ := secretApp(t, clix.InteractiveAlways, p)
	_, err := tryRun(app, out, "config", "set", "token")
	if err == nil || clix.ExitCode(err) != 2 || len(p.requests) != 0 {
		t.Errorf("config set token: err %v, prompts %d", err, len(p.requests))
	}
	app, out, _ = secretApp(t, clix.InteractiveNever, nil)
	if _, err := tryRun(app, out, "config", "set"); err == nil || clix.ExitCode(err) != 2 {
		t.Errorf("non-interactive config set: %v", err)
	}
}
