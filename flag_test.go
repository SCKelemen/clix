package clix

import (
        "reflect"
        "strings"
        "testing"
)

func TestFlagSetParse(t *testing.T) {
        fs := NewFlagSet("test")

        var name string
        fs.StringVar(&StringVarOptions{
                Name:  "name",
                Short: "n",
                Value: &name,
        })

        var verbose bool
        fs.BoolVar(&BoolVarOptions{
                Name:  "verbose",
                Short: "v",
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

        if got, ok := fs.GetString("name"); !ok || got != "alice" {
                t.Fatalf("GetString returned %q, %v", got, ok)
        }

        if got, ok := fs.GetBool("verbose"); !ok || !got {
                t.Fatalf("GetBool returned %t, %v", got, ok)
        }

        want := []string{"-x", "pos", "--flag"}
        if !reflect.DeepEqual(rest, want) {
                t.Fatalf("unexpected remaining args: want %v, got %v", want, rest)
        }
}

func TestFlagSetParseMissingValue(t *testing.T) {
        fs := NewFlagSet("test")
        fs.StringVar(&StringVarOptions{Name: "config"})

        _, err := fs.Parse([]string{"--config"})
        if err == nil {
                t.Fatalf("expected error for missing value")
        }
        if !strings.Contains(err.Error(), "flag --config requires a value") {
                t.Fatalf("unexpected error message: %v", err)
        }
}
