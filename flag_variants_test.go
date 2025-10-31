package clix

import (
	"testing"
)

func TestFlagVariants(t *testing.T) {
	t.Run("--flag=value format", func(t *testing.T) {
		fs := NewFlagSet("test")
		var name string
		fs.StringVar(&StringVarOptions{Name: "name", Value: &name})

		_, err := fs.Parse([]string{"--name=alice"})
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if name != "alice" {
			t.Errorf("expected name='alice', got %q", name)
		}
	})

	t.Run("--flag value format", func(t *testing.T) {
		fs := NewFlagSet("test")
		var name string
		fs.StringVar(&StringVarOptions{Name: "name", Value: &name})

		_, err := fs.Parse([]string{"--name", "bob"})
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if name != "bob" {
			t.Errorf("expected name='bob', got %q", name)
		}
	})

	t.Run("-f=value format", func(t *testing.T) {
		fs := NewFlagSet("test")
		var name string
		fs.StringVar(&StringVarOptions{Name: "name", Short: "n", Value: &name})

		_, err := fs.Parse([]string{"-n=charlie"})
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if name != "charlie" {
			t.Errorf("expected name='charlie', got %q", name)
		}
	})

	t.Run("-f value format", func(t *testing.T) {
		fs := NewFlagSet("test")
		var name string
		fs.StringVar(&StringVarOptions{Name: "name", Short: "n", Value: &name})

		_, err := fs.Parse([]string{"-n", "david"})
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if name != "david" {
			t.Errorf("expected name='david', got %q", name)
		}
	})

	t.Run("boolean flags", func(t *testing.T) {
		fs := NewFlagSet("test")
		var verbose bool
		fs.BoolVar(&BoolVarOptions{Name: "verbose", Short: "v", Value: &verbose})

		_, err := fs.Parse([]string{"--verbose"})
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if !verbose {
			t.Error("expected verbose to be true")
		}

		verbose = false
		_, err = fs.Parse([]string{"-v"})
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if !verbose {
			t.Error("expected verbose to be true")
		}
	})

	t.Run("mixed flag formats", func(t *testing.T) {
		fs := NewFlagSet("test")
		var name string
		var count int
		var verbose bool
		fs.StringVar(&StringVarOptions{Name: "name", Short: "n", Value: &name})
		fs.IntVar(&IntVarOptions{Name: "count", Short: "c", Value: &count})
		fs.BoolVar(&BoolVarOptions{Name: "verbose", Short: "v", Value: &verbose})

		_, err := fs.Parse([]string{"--name=alice", "-c", "42", "-v"})
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if name != "alice" {
			t.Errorf("expected name='alice', got %q", name)
		}
		if count != 42 {
			t.Errorf("expected count=42, got %d", count)
		}
		if !verbose {
			t.Error("expected verbose to be true")
		}
	})
}

