package autocomplete

import (
	"bytes"
	"io"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// fieldsValue is a Value whose value may be left out, like the format
// extension's --json (see clix.VarOptions.Value).
type fieldsValue struct{ s string }

func (v *fieldsValue) Set(s string) error  { v.s = s; return nil }
func (v *fieldsValue) String() string      { return v.s }
func (v *fieldsValue) OptionalValue() bool { return true }

// Completion reads a flag whose value is optional as the parser does: a
// flag after it is not its value.
func TestComplete_OptionalValueFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("probe", clix.WithAppOut(out), clix.WithAppErr(io.Discard))
	app.Flags().Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "json"}, Value: &fieldsValue{}})
	list := clix.NewCommand("list", clix.WithCommandRun(func(*clix.Context) error { return nil }))
	list.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "all"}})
	app.Root.AddCommand(list)
	app.AddExtension(Extension{})

	// "--json --all": --all is a flag, so it is given, and list's other
	// flags are offered.
	lines, _ := runComplete(t, app, out, "--cur=--", "--", "probe", "list", "--json", "--all")
	assertSet(t, "probe list --json --all --<TAB>", values(lines), []string{"--help"})
	// "--json name": name is the value, not a stray word.
	lines, _ = runComplete(t, app, out, "--cur=--a", "--", "probe", "list", "--json", "name")
	assertSet(t, "probe list --json name --a<TAB>", values(lines), []string{"--all"})
}
