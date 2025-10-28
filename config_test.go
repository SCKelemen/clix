package clix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigManagerLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := strings.Join([]string{
		"# comment should be ignored",
		"colour: blue",
		"quoted: \"with:colon\"",
		"spaced: ' value '",
		"invalid line without separator",
		"key: value",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	mgr := NewConfigManager("demo")
	if err := mgr.Load(path); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	tests := map[string]string{
		"colour": "blue",
		"quoted": "with:colon",
		"spaced": " value ",
		"key":    "value",
	}

	for key, want := range tests {
		got, ok := mgr.Get(key)
		if !ok {
			t.Fatalf("expected key %q to be present", key)
		}
		if got != want {
			t.Fatalf("value mismatch for %q: want %q, got %q", key, want, got)
		}
	}

	if _, ok := mgr.Get("invalid line without separator"); ok {
		t.Fatalf("unexpected key parsed from invalid line")
	}
}

func TestConfigManagerSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	mgr := NewConfigManager("demo")
	mgr.Set("token", "abc:def")
	mgr.Set("colour", "blue")
	mgr.Set("spaced", " value ")

	if err := mgr.Save(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{
		"colour: blue",
		"spaced: \" value \"",
		"token: \"abc:def\"",
	}

	if len(lines) != len(want) {
		t.Fatalf("unexpected number of lines: want %d, got %d", len(want), len(lines))
	}

	for i, line := range lines {
		if line != want[i] {
			t.Fatalf("unexpected line %d: want %q, got %q", i, want[i], line)
		}
	}

	reload := NewConfigManager("demo")
	if err := reload.Load(path); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	for key, want := range map[string]string{"token": "abc:def", "colour": "blue", "spaced": " value "} {
		got, ok := reload.Get(key)
		if !ok || got != want {
			t.Fatalf("round-trip mismatch for %q: want %q, got %q", key, want, got)
		}
	}
}
