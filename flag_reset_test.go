package clix_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// Restoring went back to the value from before the first Run, so a value
// the program assigned between two Runs was discarded.
func TestValuesAssignedBetweenRunsWin(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var name, got string
	var n int
	greet := clix.NewCommand("greet", clix.WithCommandRun(func(*clix.Context) error { got = fmt.Sprint(name, n); return nil }))
	greet.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name"}, Value: &name})
	greet.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "n"}, Value: &n})
	app.Root.AddCommand(greet)

	steps := []struct {
		preset func()
		args   []string
		want   string
	}{
		{func() { name = "bob" }, []string{"greet"}, "bob0"},
		{func() { name = "alice" }, []string{"greet"}, "alice0"},
		{nil, []string{"greet", "--name", "carol", "--n", "3"}, "carol3"},
		{nil, []string{"greet"}, "alice0"}, // carol and 3 were set by the Run, so they are undone
		{func() { n = 7 }, []string{"greet", "--name", "dave"}, "dave7"},
		{func() { name = "erin" }, []string{"greet"}, "erin7"}, // the program's values win over what the Run set
	}
	for i, s := range steps {
		if s.preset != nil {
			s.preset()
		}
		if err := app.Run(bg, s.args); err != nil || got != s.want {
			t.Fatalf("step %d %q: err = %v, got %q, want %q", i, s.args, err, got, s.want)
		}
	}
}

// A Required flag satisfied by a value assigned before the second Run
// failed as missing.
func TestRequiredPresetBeforeSecondRun(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var name string
	mk := clix.NewCommand("mk", clix.WithCommandRun(noop))
	mk.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
	app.Root.AddCommand(mk)
	var missing *clix.MissingFlagsError
	if err := app.Run(bg, []string{"mk"}); !errors.As(err, &missing) {
		t.Fatalf("run 1 = %v", err)
	}
	name = "bob"
	if err := app.Run(bg, []string{"mk"}); err != nil {
		t.Fatalf("run 2 with name preset: %v", err)
	}
}

// resettableList is an accumulating Value with Reset.
type resettableList []string

func (l *resettableList) String() string     { return strings.Join(*l, ",") }
func (l *resettableList) Set(s string) error { *l = append(*l, s); return nil }
func (l *resettableList) Reset()             { *l = nil }

// plainList is the standard library's accumulating Value, without Reset.
type plainList []string

func (l *plainList) String() string     { return strings.Join(*l, ",") }
func (l *plainList) Set(s string) error { *l = append(*l, s); return nil }

// mode is an enum Value whose Set rejects its own zero text "".
type mode struct{ v string }

func (m *mode) String() string { return m.v }
func (m *mode) Set(s string) error {
	if s != "fast" && s != "slow" {
		return fmt.Errorf("must be fast or slow")
	}
	m.v = s
	return nil
}
func (m *mode) Reset() { m.v = "" }

// Restoring passed the saved text back to Set and ignored the error, so
// an accumulating list became ["a" "b" ""] and an enum that rejects ""
// kept the previous Run's value.
func TestCustomValuesWithReset(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var list resettableList
	m := &mode{}
	preset := &mode{v: "slow"}
	var seen string
	c := clix.NewCommand("c", clix.WithCommandRun(func(*clix.Context) error {
		seen = fmt.Sprintf("%q %q %q", []string(list), m.v, preset.v)
		return nil
	}))
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "x"}, Value: &list})
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "mode"}, Value: m})
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "preset"}, Value: preset})
	app.Root.AddCommand(c)
	for _, step := range []struct {
		args []string
		want string
	}{
		{[]string{"c", "--x", "a", "--x", "b", "--mode", "fast", "--preset", "fast"}, `["a" "b"] "fast" "fast"`},
		{[]string{"c"}, `[] "" "slow"`},
		{[]string{"c", "--x", "z"}, `["z"] "" "slow"`},
		{[]string{"c"}, `[] "" "slow"`},
	} {
		if err := app.Run(bg, step.args); err != nil || seen != step.want {
			t.Fatalf("%q: err = %v, seen %s, want %s", step.args, err, seen, step.want)
		}
	}
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

// A custom Value without Reset cannot be put back: Set is not called (so
// it is not corrupted), and the carry-over is recorded for Err and
// Validate.
func TestCustomValueWithoutResetIsReported(t *testing.T) {
	app, _ := newTestApp(t, "app")
	var list plainList
	var seen []string
	c := clix.NewCommand("c", clix.WithCommandRun(func(*clix.Context) error { seen = append([]string(nil), list...); return nil }))
	c.Flags.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "x"}, Value: &list})
	app.Root.AddCommand(c)
	if err := app.Run(bg, []string{"c", "--x", "a", "--x", "b"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Validate(); err != nil {
		t.Fatalf("Validate after one Run = %v", err)
	}
	if err := app.Run(bg, []string{"c"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen, []string{"a", "b"}) {
		t.Fatalf("second run saw %q; want the carried-over [a b], not a corrupted list", seen)
	}
	want := "its value (*clix_test.plainList) has no Reset method"
	if err := c.Flags.Err(); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Err() = %v, want %q", err, want)
	}
	if err := app.Validate(); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate() = %v", err)
	}
}

// A default changed after registration to one the flag rejects is an
// error instead of being ignored.
func TestInvalidDefaultAtRunIsAnError(t *testing.T) {
	app, _ := newTestApp(t, "app")
	c := clix.NewCommand("c", clix.WithCommandRun(noop))
	c.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "port"}, Default: "80"})
	app.Root.AddCommand(c)
	c.Flags.Lookup("port").Default = "eighty"
	if err := app.Run(bg, []string{"c"}); err == nil || !strings.Contains(err.Error(), `invalid default "eighty" for --port`) {
		t.Fatalf("Run = %v", err)
	}
}

// The built-in values have Reset.
func TestBuiltinValuesReset(t *testing.T) {
	fs := clix.NewFlagSet("x")
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "s"}, Default: "v"})
	fs.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "b"}, Default: true})
	fs.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "i"}, Default: "1"})
	fs.Int64Var(clix.Int64VarOptions{FlagOptions: clix.FlagOptions{Name: "j"}, Default: "1"})
	fs.Float64Var(clix.Float64VarOptions{FlagOptions: clix.FlagOptions{Name: "f"}, Default: "1.5"})
	fs.DurationVar(clix.DurationVarOptions{FlagOptions: clix.FlagOptions{Name: "d"}, Default: "1s"})
	fs.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "l"}, Default: []string{"a"}})
	for _, f := range fs.Flags() {
		r, ok := f.Value.(interface{ Reset() })
		if !ok {
			t.Fatalf("--%s: %T has no Reset", f.Name, f.Value)
		}
		r.Reset()
		if got := f.Value.String(); got != map[string]string{"s": "", "b": "false", "i": "0", "j": "0", "f": "0", "d": "0s", "l": ""}[f.Name] {
			t.Errorf("--%s after Reset = %q", f.Name, got)
		}
	}
}
