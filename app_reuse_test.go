package clix_test

import (
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
)

// Values parsed by one Run leaked into the next: "rm --force" followed by
// "rm" still had force=true.
func TestRunRestoresValuesBetweenRuns(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	var force bool
	var target, preset string
	var timeout time.Duration
	var n int
	preset = "kept"
	rm := clix.NewCommand("rm", clix.WithCommandRun(noop))
	rm.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force"}, Value: &force})
	rm.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "target", Positional: true}, Value: &target})
	rm.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "preset"}, Value: &preset})
	rm.Flags.DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "timeout"}, Default: "5s", Value: &timeout})
	rm.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "n"}, Value: &n})
	app.Root.AddCommand(rm)
	app.Root.AddCommand(clix.NewCommand("ls", clix.WithCommandRun(noop)))

	if err := app.Run(bg, []string{"rm", "--force", "prod", "--preset", "x", "--timeout", "1m", "--n", "3"}); err != nil {
		t.Fatal(err)
	}
	if !force || target != "prod" || preset != "x" || timeout != time.Minute || n != 3 {
		t.Fatalf("first run: force=%v target=%q preset=%q timeout=%v n=%d", force, target, preset, timeout, n)
	}
	if err := app.Run(bg, []string{"rm"}); err != nil {
		t.Fatal(err)
	}
	if force || target != "" || preset != "kept" || timeout != 5*time.Second || n != 0 {
		t.Fatalf("second run: force=%v target=%q preset=%q timeout=%v n=%d", force, target, preset, timeout, n)
	}

	// Running another command resets rm's flags too.
	if err := app.Run(bg, []string{"rm", "--force"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(bg, []string{"ls"}); err != nil {
		t.Fatal(err)
	}
	if force {
		t.Fatal("force still set after running ls")
	}
}

// A value from the environment does not outlive the variable.
func TestRunRestoresEnvValues(t *testing.T) {
	app, _ := newTestApp(t, "moln")
	var region string
	cmd := clix.NewCommand("ls", clix.WithCommandRun(noop))
	cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region"}, Value: &region})
	app.Root.AddCommand(cmd)

	t.Setenv("MOLN_REGION", "eu")
	if err := app.Run(bg, []string{"ls"}); err != nil || region != "eu" {
		t.Fatalf("Run = %v, region = %q", err, region)
	}
	t.Setenv("MOLN_REGION", "")
	if err := app.Run(bg, []string{"ls"}); err != nil || region != "" {
		t.Fatalf("Run = %v, region = %q, want \"\"", err, region)
	}
}
