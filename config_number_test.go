package clix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigManagerLoadNumbers(t *testing.T) {
	// Test that ConfigManager can load YAML with numbers (both quoted and unquoted)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := strings.Join([]string{
		"port: 8080",       // unquoted number
		"count: \"1000\"",  // quoted number string
		"ratio: 3.14",      // unquoted float
		"price: \"99.99\"", // quoted float string
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	mgr := NewConfigManager("test")
	if err := mgr.Load(path); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	tests := map[string]string{
		"port":  "8080",
		"count": "1000",
		"ratio": "3.14",
		"price": "99.99",
	}

	for key, want := range tests {
		got, ok := mgr.Get(key)
		if !ok {
			t.Fatalf("expected key %q to be present", key)
		}
		if got != want {
			t.Errorf("value mismatch for %q: want %q, got %q", key, want, got)
		}
	}
}

func TestConfigManagerSaveNumbers(t *testing.T) {
	// Test that numbers saved to config can be loaded and used
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	mgr := NewConfigManager("test")
	mgr.Set("port", "8080")
	mgr.Set("ratio", "3.14159")

	if err := mgr.Save(path); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Reload and verify
	reload := NewConfigManager("test")
	if err := reload.Load(path); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	if val, ok := reload.Get("port"); !ok || val != "8080" {
		t.Errorf("port round-trip failed: got %q", val)
	}
	if val, ok := reload.Get("ratio"); !ok || val != "3.14159" {
		t.Errorf("ratio round-trip failed: got %q", val)
	}
}
