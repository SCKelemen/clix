package main

import "testing"

func TestNewAppStyles(t *testing.T) {
	app := newApp()

	if app.DefaultTheme.Prefix != "➤ " {
		t.Fatalf("expected custom prompt prefix, got %q", app.DefaultTheme.Prefix)
	}
	if app.Styles.AppTitle == nil {
		t.Fatal("expected app title style to be configured")
	}
}
