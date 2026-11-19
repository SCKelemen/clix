package clix_test

import (
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2"
)

// ctx.String on a duration gave "90s" from the default but "1m30s" from
// the command line. Getters now report the parsed value from every source.
func TestGetterReportsParsedValueFromEverySource(t *testing.T) {
	for _, src := range []string{"default", "cli", "env", "config"} {
		t.Run(src, func(t *testing.T) {
			app, _ := newTestApp(t, "myapp")
			var got string
			var bound time.Duration
			cmd := clix.NewCommand("ls", clix.WithCommandRun(func(ctx *clix.Context) error {
				got, _ = ctx.String("timeout")
				return nil
			}))
			def := ""
			if src == "default" {
				def = "90s"
			}
			cmd.Flags.DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "timeout"}, Default: def, Value: &bound})
			app.Root.AddCommand(cmd)
			args := []string{"ls"}
			switch src {
			case "cli":
				args = append(args, "--timeout", "90s")
			case "env":
				t.Setenv("MYAPP_TIMEOUT", "90s")
			case "config":
				writeConfigFile(t, app, "timeout: 90s\n")
			}
			if err := app.Run(bg, args); err != nil {
				t.Fatal(err)
			}
			if got != "1m30s" || bound != 90*time.Second {
				t.Fatalf("ctx.String(timeout) = %q, bound = %v; want 1m30s", got, bound)
			}
		})
	}
}
