package clix

import (
	"errors"
	"testing"
	"time"
)

func TestPointerVarOptionsCarryEveryField(t *testing.T) {
	validate := func(string) error { return nil }
	opts := FlagOptions{
		Name: "x", Short: "x", Usage: "u", EnvVar: "E", EnvVars: []string{"A"},
		Required: true, Prompt: "P", Positional: true, Validate: validate,
	}
	check := func(t *testing.T, fs *FlagSet, def string) {
		t.Helper()
		f := fs.lookup("x")
		if f == nil {
			t.Fatal("flag not registered")
		}
		if f.Short != "x" || f.Usage != "u" || f.EnvVar != "E" || len(f.EnvVars) != 1 ||
			!f.Required || f.Prompt != "P" || f.Validate == nil || f.Default != def {
			t.Fatalf("flag lost fields: %+v", f)
		}
	}

	t.Run("string", func(t *testing.T) {
		fs := NewFlagSet("t")
		var v string
		fs.StringVar(&StringVarOptions{FlagOptions: opts, Default: "d", Value: &v})
		check(t, fs, "d")
		if v != "d" {
			t.Fatalf("default not applied to destination: %q", v)
		}
		if !fs.lookup("x").Positional {
			t.Fatal("Positional dropped")
		}
	})
	t.Run("int", func(t *testing.T) {
		fs := NewFlagSet("t")
		var v int
		fs.IntVar(&IntVarOptions{FlagOptions: opts, Default: "7", Value: &v})
		check(t, fs, "7")
		if v != 7 {
			t.Fatalf("default not applied: %d", v)
		}
	})
	t.Run("int64", func(t *testing.T) {
		fs := NewFlagSet("t")
		var v int64
		fs.Int64Var(&Int64VarOptions{FlagOptions: opts, Default: "7", Value: &v})
		check(t, fs, "7")
	})
	t.Run("float64", func(t *testing.T) {
		fs := NewFlagSet("t")
		var v float64
		fs.Float64Var(&Float64VarOptions{FlagOptions: opts, Default: "1.5", Value: &v})
		check(t, fs, "1.5")
	})
	t.Run("duration", func(t *testing.T) {
		fs := NewFlagSet("t")
		var v time.Duration
		fs.DurationVar(&DurationVarOptions{FlagOptions: opts, Default: "1s", Value: &v})
		check(t, fs, "1s")
	})
	t.Run("bool", func(t *testing.T) {
		fs := NewFlagSet("t")
		var v bool
		o := opts
		o.Positional = false // booleans cannot be positional
		fs.BoolVar(&BoolVarOptions{FlagOptions: o, Value: &v})
		check(t, fs, "")
		if _, err := fs.Parse([]string{"--x"}); err != nil || !v {
			t.Fatalf("Parse: err=%v v=%v", err, v)
		}
	})
}

func TestApplyFlagCarriesEveryField(t *testing.T) {
	errBad := errors.New("bad")
	var fo FlagOptions
	IntVarOptions{FlagOptions: FlagOptions{
		Name: "n", Required: true, Prompt: "Count", Positional: true,
		Validate: func(string) error { return errBad },
	}}.ApplyFlag(&fo)
	if fo.Name != "n" || !fo.Required || fo.Prompt != "Count" || !fo.Positional || fo.Validate == nil {
		t.Fatalf("ApplyFlag dropped fields: %+v", fo)
	}

	// A plain FlagOptions is itself a FlagOption.
	fs := NewFlagSet("t")
	var s string
	fs.StringVar(FlagOptions{Name: "s", Required: true}, WithStringValue(&s))
	if f := fs.lookup("s"); f == nil || !f.Required {
		t.Fatalf("FlagOptions as an option: %+v", f)
	}

	// A nil option is ignored instead of panicking.
	fs.StringVar(nil, WithFlagName("t"))
	if fs.lookup("t") == nil {
		t.Fatal("flag after a nil option was not registered")
	}
}
