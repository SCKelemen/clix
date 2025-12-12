package clix

import (
	"testing"
)

// TestAppFunctionalOptions tests App functional options
func TestAppFunctionalOptions(t *testing.T) {
	app := NewApp("test",
		WithAppDescription("Test app"),
		WithAppVersion("1.0.0"),
		WithAppEnvPrefix("TEST"),
	)

	if app.Description != "Test app" {
		t.Errorf("expected Description to be 'Test app', got %q", app.Description)
	}
	if app.Version != "1.0.0" {
		t.Errorf("expected Version to be '1.0.0', got %q", app.Version)
	}
	if app.EnvPrefix != "TEST" {
		t.Errorf("expected EnvPrefix to be 'TEST', got %q", app.EnvPrefix)
	}
}

// TestStylesFunctionalOptions tests Styles functional options
func TestStylesFunctionalOptions(t *testing.T) {
	style1 := StyleFunc(func(strs ...string) string { return "style1" })

	styles := Styles{}
	opt1 := WithAppTitle(style1)
	opt1.ApplyStyle(&styles)

	if styles.AppTitle == nil {
		t.Error("expected AppTitle to be set")
	}
}

