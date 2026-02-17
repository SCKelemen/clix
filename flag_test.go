package clix

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestFlagSetParse(t *testing.T) {
	fs := NewFlagSet("test")
	// Disable strict mode for this test to allow unknown flags as positionals
	fs.SetStrict(false)

	var name string
	fs.StringVar(StringVarOptions{
		FlagOptions: FlagOptions{
			Name:  "name",
			Short: "n",
		},
		Value: &name,
	})

	var verbose bool
	fs.BoolVar(BoolVarOptions{
		FlagOptions: FlagOptions{
			Name:  "verbose",
			Short: "v",
		},
		Value: &verbose,
	})

	args := []string{"-v", "--name=alice", "-x", "pos", "--", "--flag"}
	rest, err := fs.Parse(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !verbose {
		t.Fatalf("expected verbose to be true")
	}

	if name != "alice" {
		t.Fatalf("expected name to be %q, got %q", "alice", name)
	}

	if got, ok := fs.String("name"); !ok || got != "alice" {
		t.Fatalf("String returned %q, %v", got, ok)
	}

	if got, ok := fs.Bool("verbose"); !ok || !got {
		t.Fatalf("Bool returned %t, %v", got, ok)
	}

	want := []string{"-x", "pos", "--flag"}
	if !reflect.DeepEqual(rest, want) {
		t.Fatalf("unexpected remaining args: want %v, got %v", want, rest)
	}
}

func TestFlagSetParseValidate(t *testing.T) {
	t.Run("validate rejects bad named flag value", func(t *testing.T) {
		fs := NewFlagSet("test")
		var name string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{
				Name: "name",
				Validate: func(s string) error {
					if len(s) < 3 {
						return fmt.Errorf("name must be at least 3 characters")
					}
					return nil
				},
			},
			Value: &name,
		})

		_, err := fs.Parse([]string{"--name", "ab"})
		if err == nil {
			t.Fatal("expected validation error")
		}
		if !strings.Contains(err.Error(), "name must be at least 3 characters") {
			t.Errorf("expected validation message, got: %v", err)
		}
	})

	t.Run("validate passes for good named flag value", func(t *testing.T) {
		fs := NewFlagSet("test")
		var name string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{
				Name: "name",
				Validate: func(s string) error {
					if len(s) < 3 {
						return fmt.Errorf("name must be at least 3 characters")
					}
					return nil
				},
			},
			Value: &name,
		})

		_, err := fs.Parse([]string{"--name", "alice"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "alice" {
			t.Errorf("expected name = alice, got %q", name)
		}
	})

	t.Run("validate works with equals syntax", func(t *testing.T) {
		fs := NewFlagSet("test")
		var name string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{
				Name: "name",
				Validate: func(s string) error {
					if len(s) < 3 {
						return fmt.Errorf("name must be at least 3 characters")
					}
					return nil
				},
			},
			Value: &name,
		})

		_, err := fs.Parse([]string{"--name=ab"})
		if err == nil {
			t.Fatal("expected validation error for equals syntax")
		}
		if !strings.Contains(err.Error(), "name must be at least 3 characters") {
			t.Errorf("expected validation message, got: %v", err)
		}
	})

	t.Run("nil validate is a no-op", func(t *testing.T) {
		fs := NewFlagSet("test")
		var name string
		fs.StringVar(StringVarOptions{
			FlagOptions: FlagOptions{Name: "name"},
			Value:       &name,
		})

		_, err := fs.Parse([]string{"--name", "anything"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "anything" {
			t.Errorf("expected name = anything, got %q", name)
		}
	})
}

func TestFlagSetParseMissingValue(t *testing.T) {
	fs := NewFlagSet("test")
	fs.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "config"}})

	_, err := fs.Parse([]string{"--config"})
	if err == nil {
		t.Fatalf("expected error for missing value")
	}
	if !strings.Contains(err.Error(), "flag --config requires a value") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
