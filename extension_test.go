package clix

import (
	"context"
	"errors"
	"testing"
)

func TestExtensionSystem(t *testing.T) {
	app := NewApp("test")
	root := NewCommand("test")
	app.Root = root

	// Test that extensions are applied
	extensionApplied := false
	app.AddExtension(extensionFunc(func(a *App) error {
		extensionApplied = true
		return nil
	}))

	if err := app.ApplyExtensions(); err != nil {
		t.Fatalf("ApplyExtensions failed: %v", err)
	}

	if !extensionApplied {
		t.Fatal("extension was not applied")
	}

	// Test that extensions are only applied once
	extensionApplied = false
	if err := app.ApplyExtensions(); err != nil {
		t.Fatalf("ApplyExtensions failed: %v", err)
	}

	if extensionApplied {
		t.Fatal("extension was applied twice")
	}
}

func TestExtensionError(t *testing.T) {
	app := NewApp("test")
	app.Root = NewCommand("test")

	testErr := errors.New("extension error")
	app.AddExtension(extensionFunc(func(a *App) error {
		return testErr
	}))

	err := app.ApplyExtensions()
	if err != testErr {
		t.Fatalf("expected error %v, got %v", testErr, err)
	}
}

func TestExtensionsAreAppliedDuringRun(t *testing.T) {
	app := NewApp("test")
	root := NewCommand("test")
	root.Run = func(ctx *Context) error {
		return nil
	}
	app.Root = root

	extensionApplied := false
	app.AddExtension(extensionFunc(func(a *App) error {
		extensionApplied = true
		return nil
	}))

	// Extensions should be applied during Run
	if err := app.Run(context.Background(), []string{}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !extensionApplied {
		t.Fatal("extension was not applied during Run")
	}
}

// extensionFunc is a helper for testing
type extensionFunc func(*App) error

func (f extensionFunc) Extend(app *App) error {
	return f(app)
}
