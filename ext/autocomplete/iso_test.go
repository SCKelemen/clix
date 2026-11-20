package autocomplete

import (
	"bytes"
	"io"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// A time or an ISO 8601 duration is not a file name: their flags complete
// to nothing, where other flags without a completion function fall back to
// file names.
func TestComplete_ISOFlagsOfferNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(io.Discard))
	cmd := clix.NewCommand("events", clix.WithCommandRun(func(*clix.Context) error { return nil }))
	cmd.Flags.TimeVar(clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Name: "since"}})
	cmd.Flags.ISODurationVar(clix.ISODurationVarOptions{FlagOptions: clix.FlagOptions{Name: "every"}})
	cmd.Flags.TimeVar(clix.TimeVarOptions{FlagOptions: clix.FlagOptions{Name: "day", Positional: true}})
	cmd.Flags.DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "timeout"}})
	app.Root.AddCommand(cmd)
	app.AddExtension(Extension{})

	for _, c := range []struct {
		cur       string
		words     []string
		directive string
	}{
		{"", []string{"events", "--since"}, directiveNoFiles},
		{"20", []string{"events", "--since"}, directiveNoFiles},
		{"--since=", []string{"events"}, directiveNoFiles},
		{"", []string{"events", "--every"}, directiveNoFiles},
		{"P", []string{"events"}, directiveNoFiles}, // the positional time
		{"", []string{"events", "--timeout"}, directiveFiles},
	} {
		lines, directive := runComplete(t, app, out, append([]string{"--cur=" + c.cur, "--", "probe"}, c.words...)...)
		if len(lines) != 0 || directive != c.directive {
			t.Errorf("probe %v %s<TAB>: %q, %s; want no candidates, %s", c.words, c.cur, lines, directive, c.directive)
		}
	}
}
