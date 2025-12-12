package clix

import (
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
