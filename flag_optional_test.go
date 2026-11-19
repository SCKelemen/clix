package clix_test

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// optValue is a Value whose value may be left out (OptionalValue).
type optValue struct {
	set  bool
	vals []string
}

func (v *optValue) Set(s string) error {
	v.set = true
	v.vals = append(v.vals, s)
	return nil
}
func (v *optValue) String() string      { return "" }
func (v *optValue) OptionalValue() bool { return true }
func (v *optValue) Reset()              { *v = optValue{} }

// A flag whose Value has OptionalValue() == true takes the next argument
// as its value unless it is missing or is a flag; then it is set to "".
func TestOptionalValueFlag(t *testing.T) {
	for _, tc := range []struct {
		args      []string
		vals      []string // what Set received; nil when not set
		verbose   bool
		rest      []string // positional arguments the command got
		usesShort bool
	}{
		{args: []string{"run", "--opt"}, vals: []string{""}},
		{args: []string{"run", "--opt", "a,b"}, vals: []string{"a,b"}},
		{args: []string{"run", "--opt=a"}, vals: []string{"a"}},
		{args: []string{"run", "--opt="}, vals: []string{""}},
		{args: []string{"run", "--opt=-x"}, vals: []string{"-x"}},
		{args: []string{"run", "--opt", "--verbose"}, vals: []string{""}, verbose: true},
		{args: []string{"run", "--opt", "-v"}, vals: []string{""}, verbose: true},
		{args: []string{"run", "--opt", "-"}, vals: []string{"-"}},
		{args: []string{"run", "--opt", "--", "x"}, vals: []string{""}, rest: []string{"x"}},
		{args: []string{"--opt", "run"}, vals: []string{"run"}}, // like any flag that takes a value
		{args: []string{"--opt", "--verbose", "run"}, vals: []string{""}, verbose: true},
		{args: []string{"run", "-o"}, vals: []string{""}},
		{args: []string{"run", "-vo"}, vals: []string{""}, verbose: true},
		{args: []string{"run", "-vo", "x"}, vals: []string{"x"}, verbose: true},
		{args: []string{"run", "-vo", "-v"}, vals: []string{""}, verbose: true},
		{args: []string{"run", "-ox"}, vals: []string{"x"}},
		{args: []string{"run", "--opt", "a", "--opt"}, vals: []string{"a", ""}},
		{args: []string{"run"}},
	} {
		var opt optValue
		var verbose bool
		var rest []string
		app := clix.NewApp("myapp", clix.WithAppOut(&bytes.Buffer{}), clix.WithAppInteractive(clix.InteractiveNever))
		app.Flags().Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "opt", Short: "o", CommandLineOnly: true}, Value: &opt})
		app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Short: "v"}, Value: &verbose})
		run := clix.NewCommand("run", clix.WithCommandRun(func(*clix.Context) error { return nil }))
		run.Flags.ArgsVar(clix.ArgsVarOptions{Name: "rest", Value: &rest})
		app.Root.AddCommand(run)
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		err := app.Run(context.Background(), tc.args)
		if tc.args[0] == "--opt" && tc.vals[0] == "run" {
			// "run" was the value, so the root ran (and has no handler).
			if !reflect.DeepEqual(opt.vals, tc.vals) {
				t.Errorf("%q: Set got %q, want %q", tc.args, opt.vals, tc.vals)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", tc.args, err)
			continue
		}
		if !reflect.DeepEqual(opt.vals, tc.vals) || verbose != tc.verbose || !reflect.DeepEqual(rest, tc.rest) {
			t.Errorf("%q: Set got %q, verbose %v, rest %q; want %q, %v, %q", tc.args, opt.vals, verbose, rest, tc.vals, tc.verbose, tc.rest)
		}
		if changed := app.Flags().Changed("opt"); changed != (tc.vals != nil) {
			t.Errorf("%q: Changed = %v", tc.args, changed)
		}
	}
}

// FlagSet.Parse follows the same rule.
func TestOptionalValueFlagSetParse(t *testing.T) {
	var opt optValue
	fs := clix.NewFlagSet("test")
	fs.Var(clix.VarOptions{FlagOptions: clix.FlagOptions{Name: "opt"}, Value: &opt})
	fs.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "x"}})
	rest, err := fs.Parse([]string{"--opt", "--x", "pos", "--opt", "v", "--opt"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"", "v", ""}; !reflect.DeepEqual(opt.vals, want) || !reflect.DeepEqual(rest, []string{"pos"}) {
		t.Errorf("Set got %q, rest %q; want %q, [pos]", opt.vals, rest, want)
	}
}

// A flag without OptionalValue still requires its value.
func TestRequiredValueUnchanged(t *testing.T) {
	var s string
	fs := clix.NewFlagSet("test")
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name"}, Value: &s})
	fs.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "x"}})
	if _, err := fs.Parse([]string{"--name"}); err == nil || clix.ExitCode(err) != 2 {
		t.Errorf("--name alone: err = %v", err)
	}
	if _, err := fs.Parse([]string{"--name", "--x"}); err != nil || s != "--x" {
		t.Errorf("--name --x: s = %q, err = %v (a string flag takes the next token)", s, err)
	}
}
