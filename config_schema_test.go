package clix

import (
	"errors"
	"testing"
)

func TestRegisterSchemaFoldsFunctionalOptions(t *testing.T) {
	m := NewConfigManager("x")
	m.RegisterSchema(WithConfigKey("retries"), WithConfigType(ConfigInt))
	if _, err := m.NormalizeValue("retries", "abc"); err == nil {
		t.Fatal("NormalizeValue accepted abc for a ConfigInt key")
	}
	if v, err := m.NormalizeValue("retries", " 3 "); err != nil || v != "3" {
		t.Fatalf("NormalizeValue(3) = %q, %v", v, err)
	}
}

func TestRegisterSchemaMixedStructAndOptions(t *testing.T) {
	errTooBig := errors.New("too big")
	m := NewConfigManager("x")
	m.RegisterSchema(
		ConfigSchema{Key: "retries"},
		WithConfigType(ConfigInt),
		WithConfigValidate(func(v string) error {
			if v == "99" {
				return errTooBig
			}
			return nil
		}),
	)
	if _, err := m.NormalizeValue("retries", "abc"); err == nil {
		t.Fatal("type option after struct was dropped")
	}
	if _, err := m.NormalizeValue("retries", "99"); !errors.Is(err, errTooBig) {
		t.Fatalf("validator option after struct was dropped: %v", err)
	}
}

func TestRegisterSchemaSeveralSchemas(t *testing.T) {
	m := NewConfigManager("x")
	m.RegisterSchema(
		ConfigSchema{Key: "a", Type: ConfigInt},
		ConfigSchema{Key: "b", Type: ConfigBool},
		WithConfigKey("c"), WithConfigType(ConfigFloat64),
		&ConfigSchema{Key: "d", Type: ConfigInt64},
	)
	for key, bad := range map[string]string{"a": "x", "b": "maybe", "c": "y", "d": "z"} {
		if _, err := m.NormalizeValue(key, bad); err == nil {
			t.Errorf("schema for %q was not registered", key)
		}
	}
}
