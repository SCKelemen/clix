package clix

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestExtensionsIntegration(t *testing.T) {
	t.Run("core functionality works without any extensions", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")
		root.Short = "Test command"
		root.Run = func(ctx *Context) error {
			return nil
		}
		app.Root = root

		// Don't add any extensions
		app.AddDefaultCommands()

		// Flag-based help should work
		var output bytes.Buffer
		app.Out = &output

		if err := app.Run(context.Background(), []string{"--help"}); err != nil {
			t.Fatalf("flag-based help failed: %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "TEST") {
			t.Errorf("help output should contain 'TEST', got: %s", outputStr)
		}

		// Commands should work
		if err := app.Run(context.Background(), []string{}); err != nil {
			t.Fatalf("command execution failed: %v", err)
		}
	})

	t.Run("extensions are applied in order", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")
		app.Root = root

		order := []string{}

		ext1 := extensionFunc(func(a *App) error {
			order = append(order, "first")
			return nil
		})

		ext2 := extensionFunc(func(a *App) error {
			order = append(order, "second")
			return nil
		})

		app.AddExtension(ext1)
		app.AddExtension(ext2)

		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("ApplyExtensions failed: %v", err)
		}

		if len(order) != 2 {
			t.Fatalf("expected 2 extensions to run, got %d", len(order))
		}

		if order[0] != "first" || order[1] != "second" {
			t.Errorf("extensions ran in wrong order: %v", order)
		}
	})

	t.Run("extensions can be applied multiple times safely", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")
		app.Root = root

		count := 0
		ext := extensionFunc(func(a *App) error {
			count++
			return nil
		})

		app.AddExtension(ext)

		// Apply twice
		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("first ApplyExtensions failed: %v", err)
		}
		if err := app.ApplyExtensions(); err != nil {
			t.Fatalf("second ApplyExtensions failed: %v", err)
		}

		// Should only run once
		if count != 1 {
			t.Errorf("extension should run only once, ran %d times", count)
		}
	})

	t.Run("extension errors are propagated", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")
		app.Root = root

		testErr := fmt.Errorf("extension error")
		ext := extensionFunc(func(a *App) error {
			return testErr
		})

		app.AddExtension(ext)

		err := app.ApplyExtensions()
		if err != testErr {
			t.Fatalf("expected error %v, got %v", testErr, err)
		}
	})

	t.Run("extensions work with Run()", func(t *testing.T) {
		app := NewApp("test")
		root := NewCommand("test")
		app.Root = root

		extensionApplied := false
		ext := extensionFunc(func(a *App) error {
			extensionApplied = true
			return nil
		})

		app.AddExtension(ext)

		// Run should apply extensions automatically
		if err := app.Run(context.Background(), []string{"--help"}); err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !extensionApplied {
			t.Fatal("extension should have been applied during Run()")
		}
	})
}

