package clix_test

import (
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// A first word equal to the root name was always skipped, so "echo Echo"
// lost its positional argument.
func TestRootNameIsPositionalWhenRootTakesArguments(t *testing.T) {
	app, _ := newTestApp(t, "echo")
	var text string
	app.Root.Run = noop
	app.Root.Flags.StringVar(clix.StringVarOptions{
		FlagOptions: clix.FlagOptions{Name: "text", Positional: true},
		Value:       &text,
	})
	for _, word := range []string{"Echo", "echo"} {
		if err := app.Run(bg, []string{word}); err != nil || text != word {
			t.Fatalf("Run(%q) = %v, text = %q", word, err, text)
		}
	}
}

// A child named like the app was unreachable with one word.
func TestChildNamedLikeAppIsReachable(t *testing.T) {
	app, _ := newTestApp(t, "kv")
	ran := false
	app.Root.AddCommand(clix.NewCommand("kv", clix.WithCommandRun(mark(&ran))))
	app.Root.AddCommand(clix.NewCommand("get", clix.WithCommandRun(noop)))
	if err := app.Run(bg, []string{"kv"}); err != nil || !ran {
		t.Fatalf("Run(kv) = %v, ran = %v", err, ran)
	}
}

// "app app child" still works when the repeated name cannot mean
// anything else.
func TestRepeatedRootNameIsSkipped(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	ran := false
	app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(mark(&ran))))
	if err := app.Run(bg, []string{"moln", "ls"}); err != nil || !ran {
		t.Fatalf("Run(moln ls) = %v, ran = %v", err, ran)
	}

	// A root with Run but no positional arguments: skipping is the only
	// way the word could be accepted.
	app2, _ := newTestApp(t, "tool")
	ran = false
	app2.Root.Run = mark(&ran)
	if err := app2.Run(bg, []string{"tool"}); err != nil || !ran {
		t.Fatalf("Run(tool) = %v, ran = %v", err, ran)
	}
}
