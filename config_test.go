package clix

import (
	"errors"
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

	// Verify the file can be reloaded correctly (round-trip test)
	// We don't check exact formatting since YAML encoders may vary in quoting style
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

func TestConfigManagerTypedAccessors(t *testing.T) {
	mgr := NewConfigManager("demo")
	mgr.Set("feature.enabled", "true")
	mgr.Set("project.retries", "3")
	mgr.Set("timeout", "1500")
	mgr.Set("latency", "12.5")

	if v, ok := mgr.Bool("feature.enabled"); !ok || !v {
		t.Fatalf("expected feature.enabled to be true, got %v %v", v, ok)
	}
	if v, ok := mgr.Int("project.retries"); !ok || v != 3 {
		t.Fatalf("expected project.retries to be 3, got %d %v", v, ok)
	}
	if v, ok := mgr.Int64("timeout"); !ok || v != 1500 {
		t.Fatalf("expected timeout to be 1500, got %d %v", v, ok)
	}
	if v, ok := mgr.Float64("latency"); !ok || v != 12.5 {
		t.Fatalf("expected latency to be 12.5, got %f %v", v, ok)
	}
}

func TestConfigManagerSchemaNormalization(t *testing.T) {
	mgr := NewConfigManager("demo")
	var validated bool
	mgr.RegisterSchema(ConfigSchema{
		Key:  "service.retries",
		Type: ConfigInt,
		Validate: func(val string) error {
			validated = true
			if val == "0" {
				return errors.New("must be positive")
			}
			return nil
		},
	})

	value, err := mgr.NormalizeValue("service.retries", " 5 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "5" {
		t.Fatalf("expected canonical integer string, got %q", value)
	}
	if !validated {
		t.Fatalf("expected validator to be called")
	}

	if _, err := mgr.NormalizeValue("service.retries", "0"); err == nil {
		t.Fatalf("expected validator failure for zero")
	}
	if _, err := mgr.NormalizeValue("service.retries", "abc"); err == nil {
		t.Fatalf("expected parse failure for non-integer input")
	}

	// Keys without schema pass through untouched.
	value, err = mgr.NormalizeValue("project.default", "dev")
	if err != nil {
		t.Fatalf("unexpected error for key without schema: %v", err)
	}
	if value != "dev" {
		t.Fatalf("expected passthrough value for key without schema, got %q", value)
	}
}
