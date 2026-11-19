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

// onOff is a switch (IsBoolFlag) whose Set accepts only "on" and "off".
type onOff struct{ on bool }

func (o *onOff) String() string   { return map[bool]string{true: "on", false: "off"}[o.on] }
func (o *onOff) IsBoolFlag() bool { return true }
func (o *onOff) Reset()           { o.on = false }
func (o *onOff) Set(s string) error {
	switch s {
	case "on", "off":
		o.on = s == "on"
		return nil
	}
	return errors.New("must be on or off")
}

// A bare switch's Set error came back unwrapped with exit status 1, while
// --light=maybe was "invalid value for light: ..." with exit status 2.
func TestBareSwitchSetErrorIsAUsageError(t *testing.T) {
	for _, args := range [][]string{{"c", "--light"}, {"c", "--light=maybe"}} {
		app, _ := newTestApp(t, "app")
		c := clix.NewCommand("c", clix.WithCommandRun(noop))
		c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "light"}, Value: &onOff{}})
		app.Root.AddCommand(c)
		err := app.Run(bg, args)
		if err == nil || err.Error() != "invalid value for light: must be on or off" || clix.ExitCode(err) != 2 {
			t.Errorf("Run(%q) = %v (exit %d)", args, err, clix.ExitCode(err))
		}
	}
}
