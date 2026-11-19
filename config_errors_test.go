package clix

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAllowConfigErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(path, []byte("project: [unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := NewApp("probe", WithAppOut(&bytes.Buffer{}))
	app.ConfigPath = path
	ran := 0
	repair := NewCommand("repair", WithCommandRun(func(*Context) error {
		ran++
		return os.Remove(path)
	}))
	other := NewCommand("other", WithCommandRun(func(*Context) error { return nil }))
	AllowConfigErrors(repair)
	app.Root.AddCommand(repair)
	app.Root.AddCommand(other)

	ctx := context.Background()
	if err := app.Run(ctx, []string{"other"}); err == nil {
		t.Fatal("a broken config file must still fail ordinary commands")
	}
	if err := app.Run(ctx, []string{"repair"}); err != nil || ran != 1 {
		t.Fatalf("repair: err=%v ran=%d", err, ran)
	}
	// The file is gone, so the next run loads cleanly.
	if err := app.Run(ctx, []string{"other"}); err != nil {
		t.Fatalf("after repair: %v", err)
	}
}
