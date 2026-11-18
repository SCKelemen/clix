package clix

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, app string, contents string) {
	t.Helper()
	dir := os.Getenv("XDG_CONFIG_HOME")
	if err := os.MkdirAll(filepath.Join(dir, app), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, app, "config.yaml"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyConfigFileIsIgnored(t *testing.T) {
	for _, contents := range []string{"", "# only a comment\n", "\n\n"} {
		app, out, _ := newIsolatedApp(t, "moln")
		writeConfig(t, "moln", contents)
		ran := false
		app.Root.AddCommand(NewCommand("ls", WithCommandRun(func(*Context) error { ran = true; return nil })))

		if err := app.Run(context.Background(), []string{"--help"}); err != nil {
			t.Fatalf("--help with config %q: %v", contents, err)
		}
		if !strings.Contains(out.String(), "USAGE") {
			t.Fatalf("no help printed for config %q", contents)
		}
		if err := app.Run(context.Background(), []string{"ls"}); err != nil || !ran {
			t.Fatalf("ls with config %q: ran=%v err=%v", contents, ran, err)
		}
	}
}

func TestNullConfigValueIsUnset(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "moln")
	writeConfig(t, "moln", "project:\nzone: z1\nlist: [a, null, b]\n")
	var project string
	cmd := NewCommand("ls", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "project", Required: true}, Value: &project})
	cmd.Flags.BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "quiet"}})
	app.Root.AddCommand(cmd)

	err := app.Run(context.Background(), []string{"ls", "--quiet"})
	if err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("Run error = %v, want --project reported missing", err)
	}
	if project == "<nil>" {
		t.Fatal(`null config value became "<nil>"`)
	}
	if _, ok := app.Config.Get("project"); ok {
		t.Fatal("null value stored in config")
	}
	if v, _ := app.Config.Get("list"); v != "a,,b" {
		t.Fatalf("list = %q, want a,,b", v)
	}
}

func TestMalformedConfigDoesNotBlockHelp(t *testing.T) {
	app, out, _ := newIsolatedApp(t, "moln")
	writeConfig(t, "moln", "project: [unclosed\n")
	app.Root.AddCommand(NewCommand("ls", WithCommandRun(func(*Context) error { return nil })))

	if err := app.Run(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("--help with malformed config: %v", err)
	}
	if !strings.Contains(out.String(), "USAGE") {
		t.Fatal("no help printed")
	}
	if err := app.Run(context.Background(), []string{"ls"}); err == nil || !strings.Contains(err.Error(), "YAML") {
		t.Fatalf("ls with malformed config: err=%v, want a YAML parse error", err)
	}
}
