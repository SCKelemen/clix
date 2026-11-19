package clix

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRequiredWorksForEveryType(t *testing.T) {
	register := map[string]func(fs *FlagSet){
		"int": func(fs *FlagSet) {
			fs.IntVar(IntVarOptions{FlagOptions: FlagOptions{Name: "n", Required: true}, Value: new(int)})
		},
		"int64": func(fs *FlagSet) {
			fs.Int64Var(Int64VarOptions{FlagOptions: FlagOptions{Name: "n", Required: true}, Value: new(int64)})
		},
		"float64": func(fs *FlagSet) {
			fs.Float64Var(Float64VarOptions{FlagOptions: FlagOptions{Name: "n", Required: true}, Value: new(float64)})
		},
		"duration": func(fs *FlagSet) {
			fs.DurationVar(DurationVarOptions{FlagOptions: FlagOptions{Name: "n", Required: true}, Value: new(time.Duration)})
		},
		"bool": func(fs *FlagSet) {
			fs.BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "n", Required: true}, Value: new(bool)})
		},
	}
	for typ, reg := range register {
		t.Run(typ, func(t *testing.T) {
			fs := NewFlagSet("x")
			reg(fs)
			if got := fs.MissingRequired(); len(got) != 1 {
				t.Fatalf("MissingRequired() = %d flags before parsing, want 1", len(got))
			}
			arg := "--n=1"
			if typ == "duration" {
				arg = "--n=1s"
			}
			if _, err := fs.Parse([]string{arg}); err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			if got := fs.MissingRequired(); len(got) != 0 {
				t.Fatalf("MissingRequired() = %d flags after %s, want 0", len(got), arg)
			}
		})
	}
}

func TestRequiredIntIsEnforcedByRun(t *testing.T) {
	app, _, _ := newIsolatedApp(t, "myapp")
	var replicas int
	ran := false
	cmd := NewCommand("scale", WithCommandRun(func(*Context) error { ran = true; return nil }))
	cmd.Flags.IntVar(IntVarOptions{FlagOptions: FlagOptions{Name: "replicas", Required: true}, Value: &replicas})
	cmd.Flags.BoolVar(BoolVarOptions{FlagOptions: FlagOptions{Name: "dry-run"}})
	app.Root.AddCommand(cmd)

	err := app.Run(context.Background(), []string{"scale", "--dry-run"})
	if ran || err == nil || !strings.Contains(err.Error(), "--replicas") {
		t.Fatalf("ran=%v err=%v; want missing --replicas", ran, err)
	}

	t.Setenv("MYAPP_REPLICAS", "3")
	if err := app.Run(context.Background(), []string{"scale", "--dry-run"}); err != nil {
		t.Fatalf("Run error with MYAPP_REPLICAS set: %v", err)
	}
	if !ran || replicas != 3 {
		t.Fatalf("ran=%v replicas=%d, want 3 from the environment", ran, replicas)
	}
}

func TestRequiredWithDefaultIsSatisfied(t *testing.T) {
	fs := NewFlagSet("x")
	fs.IntVar(IntVarOptions{FlagOptions: FlagOptions{Name: "port", Required: true}, Default: "8080", Value: new(int)})
	if got := fs.MissingRequired(); len(got) != 0 {
		t.Fatalf("MissingRequired() = %v, want none (the flag has a default)", got)
	}
}
