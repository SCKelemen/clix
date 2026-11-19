package clix

import (
	"bytes"
	"context"
	"reflect"
	"testing"
)

func TestArgsVarCollectsRemainingArguments(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var first string
	var rest []string
	app := NewApp("probe", WithAppOut(&bytes.Buffer{}))
	cmd := NewCommand("add", WithCommandRun(func(*Context) error { return nil }))
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "target", Positional: true}, Value: &first})
	cmd.Flags.ArgsVar(ArgsVarOptions{Name: "path", Usage: "Files to add", Value: &rest})
	app.Root.AddCommand(cmd)

	ctx := context.Background()
	if err := app.Run(ctx, []string{"add", "t", "a", "b", "--", "-c"}); err != nil {
		t.Fatal(err)
	}
	if first != "t" || !reflect.DeepEqual(rest, []string{"a", "b", "-c"}) {
		t.Fatalf("target=%q path=%q", first, rest)
	}

	// A second run starts empty.
	if err := app.Run(ctx, []string{"add", "t"}); err != nil {
		t.Fatal(err)
	}
	if len(rest) != 0 {
		t.Fatalf("second run kept %q", rest)
	}

	// --path appends too.
	if err := app.Run(ctx, []string{"add", "t", "--path", "x", "y"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rest, []string{"x", "y"}) {
		t.Fatalf("path=%q", rest)
	}

	var f *Flag
	for _, fl := range cmd.Flags.Flags() {
		if fl.Name == "path" {
			f = fl
		}
	}
	if f == nil || !f.IsVariadic() {
		t.Fatal("path should be variadic")
	}
}

func TestArgsVarOnlyWhenLast(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	app := NewApp("probe", WithAppOut(&bytes.Buffer{}))
	cmd := NewCommand("one", WithCommandRun(func(*Context) error { return nil }))
	var v string
	cmd.Flags.StringVar(StringVarOptions{FlagOptions: FlagOptions{Name: "x", Positional: true}, Value: &v})
	app.Root.AddCommand(cmd)
	if err := app.Run(context.Background(), []string{"one", "a", "b"}); err == nil {
		t.Fatal("extra arguments must still be an error without ArgsVar")
	}
}
