package clix

import (
	"testing"
)

func TestPositionalFlags(t *testing.T) {
	t.Run("returns positional flags in registration order", func(t *testing.T) {
		fs := NewFlagSet("test")
		var a, b, c string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "alpha", Positional: true},
			Value:       &a,
		})
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "beta"},
			Value:       &b,
		})
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "gamma", Positional: true},
			Value:       &c,
		})

		pos := fs.PositionalFlags()
		if len(pos) != 2 {
			t.Fatalf("expected 2 positional flags, got %d", len(pos))
		}
		if pos[0].Name != "alpha" {
			t.Errorf("expected first positional to be alpha, got %s", pos[0].Name)
		}
		if pos[1].Name != "gamma" {
			t.Errorf("expected second positional to be gamma, got %s", pos[1].Name)
		}
	})

	t.Run("returns empty when no positional flags", func(t *testing.T) {
		fs := NewFlagSet("test")
		var a string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "alpha"},
			Value:       &a,
		})

		pos := fs.PositionalFlags()
		if len(pos) != 0 {
			t.Fatalf("expected 0 positional flags, got %d", len(pos))
		}
	})
}

func TestMapPositionals(t *testing.T) {
	t.Run("basic mapping", func(t *testing.T) {
		fs := NewFlagSet("test")
		var a, b string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "first", Positional: true},
			Value:       &a,
		})
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "second", Positional: true},
			Value:       &b,
		})

		excess, err := fs.MapPositionals([]string{"hello", "world"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(excess) != 0 {
			t.Errorf("expected no excess, got %v", excess)
		}
		if a != "hello" {
			t.Errorf("expected first = hello, got %q", a)
		}
		if b != "world" {
			t.Errorf("expected second = world, got %q", b)
		}
	})

	t.Run("skip already set via CLI", func(t *testing.T) {
		fs := NewFlagSet("test")
		var a, b string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "first", Positional: true},
			Value:       &a,
		})
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "second", Positional: true},
			Value:       &b,
		})

		// Simulate --first being set via CLI parsing
		f := fs.lookup("first")
		f.Value.Set("cli-value")
		f.cliSet = true
		f.set = true

		excess, err := fs.MapPositionals([]string{"positional-value"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(excess) != 0 {
			t.Errorf("expected no excess, got %v", excess)
		}
		if a != "cli-value" {
			t.Errorf("first should remain cli-value, got %q", a)
		}
		if b != "positional-value" {
			t.Errorf("second should be positional-value, got %q", b)
		}
	})

	t.Run("excess args returned", func(t *testing.T) {
		fs := NewFlagSet("test")
		var a string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "first", Positional: true},
			Value:       &a,
		})

		excess, err := fs.MapPositionals([]string{"hello", "extra1", "extra2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(excess) != 2 {
			t.Fatalf("expected 2 excess args, got %d", len(excess))
		}
		if excess[0] != "extra1" || excess[1] != "extra2" {
			t.Errorf("unexpected excess: %v", excess)
		}
	})

	t.Run("no positional flags returns all args as excess", func(t *testing.T) {
		fs := NewFlagSet("test")
		var a string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "first"},
			Value:       &a,
		})

		excess, err := fs.MapPositionals([]string{"hello"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(excess) != 1 {
			t.Fatalf("expected 1 excess arg, got %d", len(excess))
		}
	})

	t.Run("sets cliSet on mapped flags", func(t *testing.T) {
		fs := NewFlagSet("test")
		var a string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "first", Positional: true},
			Value:       &a,
		})

		_, err := fs.MapPositionals([]string{"hello"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		f := fs.lookup("first")
		if !f.cliSet {
			t.Error("expected cliSet to be true after positional mapping")
		}
		if !f.set {
			t.Error("expected set to be true after positional mapping")
		}
	})

	t.Run("empty args is a no-op", func(t *testing.T) {
		fs := NewFlagSet("test")
		var a string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "first", Positional: true},
			Value:       &a,
		})

		excess, err := fs.MapPositionals([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(excess) != 0 {
			t.Errorf("expected no excess, got %v", excess)
		}
		if a != "" {
			t.Errorf("expected empty value, got %q", a)
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		fs := NewFlagSet("test")
		var n int
		fs.IntVar(IntVarOptions{
			FlagOptions: FlagOptions{Name: "count", Positional: true},
			Value:       &n,
		})

		_, err := fs.MapPositionals([]string{"not-a-number"})
		if err == nil {
			t.Fatal("expected error for invalid int value")
		}
	})
}

func TestBooleanPositionalPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for boolean positional flag")
		}
	}()

	fs := NewFlagSet("test")
	var b bool
	fs.BoolVar(BoolVarOptions{
		FlagOptions: FlagOptions{Name: "verbose", Positional: true},
		Value:       &b,
	})
}
