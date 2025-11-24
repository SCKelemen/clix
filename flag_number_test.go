package clix

import (
	"context"
	"os"
	"testing"
)

func TestIntVar(t *testing.T) {
	var port int
	fs := NewFlagSet("test")
	fs.IntVar(IntVarOptions{
		FlagOptions: FlagOptions{Name: "port"},
		Default:     "8080",
		Value:       &port,
	})

	// Test default value
	if port != 8080 {
		t.Errorf("expected default port 8080, got %d", port)
	}

	// Test parsing from command line
	_, err := fs.Parse([]string{"--port", "9090"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if port != 9090 {
		t.Errorf("expected port 9090, got %d", port)
	}

	// Test Integer
	if val, ok := fs.Integer("port"); !ok || val != 9090 {
		t.Errorf("Integer returned %d, %v, expected 9090, true", val, ok)
	}
}

func TestInt64Var(t *testing.T) {
	var count int64
	fs := NewFlagSet("test")
	fs.Int64Var(Int64VarOptions{
		FlagOptions: FlagOptions{Name: "count"},
		Default:      "1000000000",
		Value:        &count,
	})

	if count != 1000000000 {
		t.Errorf("expected default count 1000000000, got %d", count)
	}

	_, err := fs.Parse([]string{"--count", "2000000000"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if count != 2000000000 {
		t.Errorf("expected count 2000000000, got %d", count)
	}

	if val, ok := fs.Int64("count"); !ok || val != 2000000000 {
		t.Errorf("Int64 returned %d, %v, expected 2000000000, true", val, ok)
	}
}

func TestFloat64Var(t *testing.T) {
	var ratio float64
	fs := NewFlagSet("test")
	fs.Float64Var(Float64VarOptions{
		FlagOptions: FlagOptions{Name: "ratio"},
		Default:      "3.14159",
		Value:         &ratio,
	})

	if ratio != 3.14159 {
		t.Errorf("expected default ratio 3.14159, got %f", ratio)
	}

	_, err := fs.Parse([]string{"--ratio", "2.71828"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if ratio != 2.71828 {
		t.Errorf("expected ratio 2.71828, got %f", ratio)
	}

	if val, ok := fs.Float64("ratio"); !ok || val != 2.71828 {
		t.Errorf("Float64 returned %f, %v, expected 2.71828, true", val, ok)
	}
}

func TestNumberVarWithConfigAndEnv(t *testing.T) {
	// Use a temporary home directory for config
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	defer os.Unsetenv("HOME")

	app := NewApp("test")
	var port int
	root := NewCommand("test")
	root.Flags.IntVar(IntVarOptions{
		FlagOptions: FlagOptions{Name: "port"},
		Default:      "3000",
		Value:        &port,
	})
	root.Run = func(ctx *Context) error {
		return nil
	}
	app.Root = root

	// Set config value
	app.Config.Set("port", "8080")
	app.SaveConfig()
	app.configLoaded = false

	// Test that config value is used
	if err := app.Run(context.Background(), []string{}); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if port != 8080 {
		t.Errorf("expected port from config 8080, got %d", port)
	}

	// Test that env var overrides config
	t.Setenv("TEST_PORT", "9090")
	app.configLoaded = false
	if err := app.Run(context.Background(), []string{}); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if port != 9090 {
		t.Errorf("expected port from env 9090, got %d", port)
	}

	// Test that flag overrides everything
	app.configLoaded = false
	if err := app.Run(context.Background(), []string{"--port", "5000"}); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if port != 5000 {
		t.Errorf("expected port from flag 5000, got %d", port)
	}
}
