//go:build darwin || linux

package clix_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/ptytest"
)

// On a real terminal in the default mode, an Ambient member of a group
// still lets Run prompt for missing required flags, while a broken group
// is reported before anything is asked.
func TestFlagGroupsPromptOnTerminal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("MYAPP_DISABLE_PROMPTS", "")
	term := ptytest.New(t)
	app := clix.NewApp("myapp", clix.WithAppIn(term.TTY), clix.WithAppErr(term.TTY), clix.WithAppOut(&bytes.Buffer{}))
	var asJSON, asYAML bool
	var name, file, url string
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "json", Ambient: true}, Value: &asJSON})
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "yaml", Ambient: true}, Value: &asYAML})
	app.Flags().MarkMutuallyExclusive("json", "yaml")
	create := clix.NewCommand("create", clix.WithCommandRun(noop))
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "file", Required: true}, Value: &file})
	create.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "url"}, Value: &url})
	create.Flags.MarkOneRequired("file", "url")
	app.Root.AddCommand(create)

	done := make(chan error, 1)
	go func() { done <- app.Run(bg, []string{"--json", "create"}) }()
	term.WaitOutput(t, "Name", 5*time.Second)
	term.Type(t, "w1\n")
	term.WaitOutput(t, "File", 5*time.Second)
	term.Type(t, "spec.yaml\n")
	select {
	case err := <-done:
		if err != nil || !asJSON || name != "w1" || file != "spec.yaml" {
			t.Fatalf("err = %v, json %v, name %q, file %q", err, asJSON, name, file)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the answers")
	}

	term.ResetOutput()
	err := app.Run(bg, []string{"--json", "--yaml", "create"})
	var group *clix.FlagGroupError
	if !errors.As(err, &group) || group.Kind != clix.FlagsMutuallyExclusive || clix.ExitCode(err) != 2 {
		t.Fatalf("--json --yaml: err = %v, want a mutually exclusive *FlagGroupError", err)
	}
	time.Sleep(50 * time.Millisecond) // let the drain goroutine catch up
	if out := term.Output(); out != "" {
		t.Fatalf("prompted for a broken group: %q", out)
	}
}
