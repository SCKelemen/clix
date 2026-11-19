package clix_test

import (
	"errors"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// A bare --force skipped Validate while --force=true ran it.
func TestBareSwitchRunsValidate(t *testing.T) {
	var seen []string
	for _, args := range [][]string{{"rm", "--force=true"}, {"rm", "--force"}, {"rm", "-f"}} {
		app, _ := newTestApp(t, "moln")
		ran := false
		cmd := clix.NewCommand("rm", clix.WithCommandRun(mark(&ran)))
		cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{
			Name:  "force",
			Short: "f",
			Validate: func(v string) error {
				seen = append(seen, v)
				return errors.New("--force is disabled by policy")
			},
		}})
		app.Root.AddCommand(cmd)
		err := app.Run(bg, args)
		if err == nil || ran || clix.ExitCode(err) != 2 {
			t.Fatalf("Run(%q) = %v (ran=%v), want a usage error", args, err, ran)
		}
	}
	for _, v := range seen {
		if v != "true" {
			t.Fatalf("Validate saw %q, want \"true\"", v)
		}
	}
}
