package config

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func TestConfigCommandsAcceptPositionalArgs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("moln", clix.WithAppOut(out), clix.WithAppErr(&bytes.Buffer{}))
	app.AddExtension(Extension{})

	run := func(args ...string) string {
		t.Helper()
		out.Reset()
		if err := app.Run(context.Background(), args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return strings.TrimSpace(out.String())
	}

	if got := run("config", "set", "core.project", "p1"); got != "core.project = p1" {
		t.Fatalf("config set: %q", got)
	}
	if got := run("config", "get", "core.project"); got != "p1" {
		t.Fatalf("config get: %q", got)
	}
	if got := run("config", "get", "--key", "core.project"); got != "p1" {
		t.Fatalf("config get --key: %q", got)
	}
	if got := run("config", "unset", "core.project"); got != "core.project removed" {
		t.Fatalf("config unset: %q", got)
	}
	if err := app.Run(context.Background(), []string{"config", "get", "core.project"}); err == nil {
		t.Fatal("config get after unset should fail")
	}
}
