package clix_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// "add -5 3" failed with "unknown flag: -5" on a subcommand, though it
// worked on the root.
func TestNegativeNumbersArePositionals(t *testing.T) {
	for _, args := range [][]string{{"add", "-5", "3"}, {"add", "--", "-5", "3"}, {"add", "-5", "--b", "3"}} {
		app, _ := newTestApp(t, "calc")
		var a, b int
		add := clix.NewCommand("add", clix.WithCommandRun(noop))
		add.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "a", Positional: true}, Value: &a})
		add.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "b", Positional: true}, Value: &b})
		app.Root.AddCommand(add)
		if err := app.Run(bg, args); err != nil || a != -5 || b != 3 {
			t.Fatalf("Run(%q) = %v, a=%d b=%d", args, err, a, b)
		}
	}
}

// Short clusters (-xv) and attached values (-fa.tar) were unsupported.
func TestShortClustersAndAttachedValues(t *testing.T) {
	cases := []struct {
		args    []string
		x, v    bool
		file    string
		wantErr bool
	}{
		{args: []string{"-xv", "-fa.tar"}, x: true, v: true, file: "a.tar"},
		{args: []string{"-xvf", "b.tar"}, x: true, v: true, file: "b.tar"},
		{args: []string{"-xvfc.tar"}, x: true, v: true, file: "c.tar"},
		{args: []string{"-vf=d.tar"}, v: true, file: "d.tar"},
		{args: []string{"-x=false", "-v"}, v: true},
		{args: []string{"-xq"}, wantErr: true}, // q is not a flag
		{args: []string{"-xf"}, wantErr: true}, // f needs a value
	}
	for _, c := range cases {
		app, _ := newTestApp(t, "tar")
		var x, v bool
		var file string
		cmd := clix.NewCommand("run", clix.WithCommandRun(noop))
		cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "extract", Short: "x"}, Value: &x})
		cmd.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Short: "v"}, Value: &v})
		cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "file", Short: "f"}, Value: &file})
		app.Root.AddCommand(cmd)
		err := app.Run(bg, append([]string{"run"}, c.args...))
		if c.wantErr {
			if clix.ExitCode(err) != 2 {
				t.Errorf("Run(%q) = %v, want a usage error", c.args, err)
			}
			continue
		}
		if err != nil || x != c.x || v != c.v || file != c.file {
			t.Errorf("Run(%q) = %v x=%v v=%v file=%q", c.args, err, x, v, file)
		}
	}
}

func TestFlagSetParseClustersAndNumbers(t *testing.T) {
	fs := clix.NewFlagSet("x")
	var a, b bool
	var n string
	fs.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "aa", Short: "a"}, Value: &a})
	fs.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "bb", Short: "b"}, Value: &b})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Short: "n"}, Value: &n})
	rest, err := fs.Parse([]string{"-ab", "-nx", "-5", "-1.5", "-"})
	if err != nil || !a || !b || n != "x" || !reflect.DeepEqual(rest, []string{"-5", "-1.5", "-"}) {
		t.Fatalf("Parse = %q, %v; a=%v b=%v n=%q", rest, err, a, b, n)
	}
	if _, err := fs.Parse([]string{"-abz"}); err == nil {
		t.Fatal("unknown letter in a cluster was accepted")
	}
}

// A cluster written before the command, mixing the command's switch with
// a root flag that takes a value, used the command name as the value:
// "myapp -fp deploy" ran deploy with --project=deploy.
func TestClusterBeforeCommandDoesNotEatTheCommand(t *testing.T) {
	for _, args := range [][]string{{"-fp", "deploy"}, {"-fp", "deploy", "prod"}, {"-fr", "deploy"}} {
		app, _ := newTestApp(t, "myapp")
		var project, rootRegion string
		ran := false
		app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Short: "p"}, Value: &project})
		app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "root-region", Short: "r"}, Value: &rootRegion})
		deploy := clix.NewCommand("deploy", clix.WithCommandRun(mark(&ran)))
		deploy.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force", Short: "f"}})
		deploy.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "target", Positional: true}})
		app.Root.AddCommand(deploy)
		err := app.Run(bg, args)
		want := args[0][2:] // the value letter
		if err == nil || clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), "-"+want+" needs a value: write "+args[0]+"=VALUE") || ran {
			t.Errorf("Run(%q) = %v, ran = %v, project = %q, root-region = %q", args, err, ran, project, rootRegion)
		}
	}
}

// A numeric short such as -1 written before the command that defines it
// was a word ("unknown command: m -1"), while -f worked.
func TestNumericShortBeforeCommand(t *testing.T) {
	for _, args := range [][]string{{"-1", "deploy"}, {"deploy", "-1"}, {"-1f", "deploy"}} {
		app, _ := newTestApp(t, "m")
		var one, force bool
		c := clix.NewCommand("deploy", clix.WithCommandRun(noop))
		c.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "one", Short: "1"}, Value: &one})
		c.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force", Short: "f"}, Value: &force})
		app.Root.AddCommand(c)
		if err := app.Run(bg, args); err != nil || !one || force != strings.Contains(args[0], "f") {
			t.Errorf("Run(%q) = %v, one = %v, force = %v", args, err, one, force)
		}
	}
}
