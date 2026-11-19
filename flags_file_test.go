package clix_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/format"
	"github.com/SCKelemen/clix/v2/ext/help"
)

// flagsFileApp is an app with --flags-file and this tree:
//
//	myapp [--project] [--verbose/-v] [--target (Ambient)]
//	└── deploy [--region] [--count] [--force (CommandLineOnly)] [--tag]...
//	           [--token (Sensitive)] [--body (ValueFromFile)] [--note] [name]
type flagsFileApp struct {
	app    *clix.App
	deploy *clix.Command
	out    *bytes.Buffer

	project, target, region, token, body, note, name string
	verbose, force                                   bool
	count                                            int
	tags                                             []string
}

func newFlagsFileApp(t *testing.T, opts ...clix.AppOption) *flagsFileApp {
	t.Helper()
	a := &flagsFileApp{}
	a.app, a.out = newTestApp(t, "myapp", opts...)
	a.app.EnableFlagsFile()
	root := a.app.Flags()
	root.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project"}, Value: &a.project})
	root.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "verbose", Short: "v"}, Value: &a.verbose})
	root.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "target", Ambient: true}, Value: &a.target})
	a.deploy = clix.NewCommand("deploy", clix.WithCommandRun(noop))
	fs := a.deploy.Flags
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region"}, Value: &a.region})
	fs.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "count"}, Value: &a.count})
	fs.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force", CommandLineOnly: true}, Value: &a.force})
	fs.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "tag"}, Value: &a.tags})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "token", Sensitive: true, Validate: func(v string) error {
		if !strings.HasPrefix(v, "tok_") {
			return errors.New("token " + v + " does not start with tok_")
		}
		return nil
	}}, Value: &a.token})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "body", ValueFromFile: true}, Value: &a.body})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "note"}, Value: &a.note})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Positional: true}, Value: &a.name})
	a.app.Root.AddCommand(a.deploy)
	return a
}

// state summarizes the values the run set.
func (a *flagsFileApp) state() string {
	return fmt.Sprintf("project=%s verbose=%v target=%s region=%s count=%d force=%v tags=%s token=%s body=%s note=%s name=%s",
		a.project, a.verbose, a.target, a.region, a.count, a.force, strings.Join(a.tags, "|"), a.token, a.body, a.note, a.name)
}

// The values a flags file gives, in YAML and in JSON.
func TestFlagsFileValues(t *testing.T) {
	const zero = "project= verbose=false target= region= count=0 force=false tags= token= body= note= name="
	set := func(changes ...string) string {
		s := zero
		for _, c := range changes {
			key, _, _ := strings.Cut(c, "=")
			start := strings.Index(s, key+"=")
			end := strings.IndexByte(s[start:], ' ')
			if end < 0 {
				end = len(s) - start
			}
			s = s[:start] + c + s[start+end:]
		}
		return s
	}
	tests := []struct {
		name string
		file string
		args []string // the file's path is appended to --flags-file
		want string
	}{
		{"YAML scalars", "--region: us-east1\n--count: 3\n--note: a b  c\n", []string{"deploy"}, set("region=us-east1", "count=3", "note=a b  c")},
		{"keys without dashes", "region: us-east1\ncount: 3\n", []string{"deploy"}, set("region=us-east1", "count=3")},
		{"JSON", `{"--region": "eu", "count": 7, "--force": true, "--tag": ["a", "b,c"]}`, []string{"deploy"}, set("region=eu", "count=7", "force=true", "tags=a|b|c")},
		{"no value turns a switch on", "--force:\n--verbose: null\n", []string{"deploy"}, set("force=true", "verbose=true")},
		{"false turns a switch off", "--force: false\n", []string{"deploy", "--force"}, set("force=false")},
		{"a list repeats the flag", "--tag: [a, \"b,c\", d]\n", []string{"deploy"}, set("tags=a|b|c|d")},
		{"a list of mappings repeats keys", "- --tag: a\n  --region: one\n- --tag: b\n  --region: two\n", []string{"deploy"}, set("tags=a|b", "region=two")},
		{"text as written", "--note: 0755\n--region: 1.10\n--count: 0755\n", []string{"deploy"}, set("note=0755", "region=1.10", "count=755")},
		{"quoted values", "--note: \"line\\nbreak\"\n--region: '#not a comment'\n", []string{"deploy"}, set("note=line\nbreak", "region=#not a comment")},
		{"a value that looks like a flag", "--note: --force\n--region: -5\n", []string{"deploy"}, set("note=--force", "region=-5")},
		{"the empty string", "--note: \"\"\n", []string{"deploy", "--note", "x"}, set("note=")},
		{"an empty file", "", []string{"deploy"}, zero},
		{"only comments", "# nothing here\n", []string{"deploy"}, zero},
		{"null document", "~\n", []string{"deploy"}, zero},
		{"a positional flag by name", "--name: web\n", []string{"deploy"}, set("name=web")},
		{"root and command flags", "--project: p1\n--region: r1\n", []string{"deploy"}, set("project=p1", "region=r1")},
		{"CommandLineOnly flags may be set", "--force: true\n", []string{"deploy"}, set("force=true")},
		{"Windows paths as plain scalars", "--note: C:\\certs\\a.pem\n--region: 'D:\\x\\'\n", []string{"deploy"}, set(`note=C:\certs\a.pem`, `region=D:\x\`)},
		{"aliases", "- &m {--tag: a, --region: r}\n- *m\n", []string{"deploy"}, set("tags=a|a", "region=r")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newFlagsFileApp(t)
			path := writeTestFile(t, t.TempDir(), "flags.yaml", tt.file)
			args := append(append([]string(nil), tt.args...), "--flags-file", path)
			if err := a.app.Run(context.Background(), args); err != nil {
				t.Fatal(err)
			}
			if got := a.state(); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

// Each --flags-file is replaced by its flags where it is written, so the
// last occurrence of a flag wins, as gcloud documents; flags-file values
// are command-line values, so they beat the environment and the config
// file, and they report the command line as their source.
func TestFlagsFilePrecedence(t *testing.T) {
	dir := t.TempDir()
	one := writeTestFile(t, dir, "flags-1.yaml", "--region: us-east2-a\n")
	two := writeTestFile(t, dir, "flags-2.yaml", "--verbose:\n--region: us-central1-a\n")
	tags := writeTestFile(t, dir, "tags.yaml", "--tag: [f1, f2]\n")

	tests := []struct {
		name       string
		args       []string
		env        string // MYAPP_REGION
		config     string // region in the config file
		wantRegion string
		wantSource clix.Source
		wantTags   string
	}{
		{"gcloud's example", []string{"deploy", "--flags-file=" + one, "my-instance", "--flags-file=" + two}, "", "", "us-central1-a", clix.SourceCommandFlag, ""},
		{"a flag after the file wins", []string{"deploy", "--flags-file", one, "--region", "cli"}, "", "", "cli", clix.SourceCommandFlag, ""},
		{"the file after a flag wins", []string{"deploy", "--region", "cli", "--flags-file", one}, "", "", "us-east2-a", clix.SourceCommandFlag, ""},
		{"the file beats the environment", []string{"deploy", "--flags-file", one}, "env", "", "us-east2-a", clix.SourceCommandFlag, ""},
		{"the file beats the config file", []string{"deploy", "--flags-file", one}, "", "config", "us-east2-a", clix.SourceCommandFlag, ""},
		{"the environment fills what the file leaves", []string{"deploy", "--flags-file", tags}, "env", "config", "env", clix.SourceEnvVar, "f1|f2"},
		{"the config file fills what the file leaves", []string{"deploy", "--flags-file", tags}, "", "config", "config", clix.SourceConfigFile, "f1|f2"},
		{"slices append in order", []string{"deploy", "--tag", "a", "--flags-file", tags, "--tag", "b"}, "", "", "", clix.SourceDefault, "a|f1|f2|b"},
		{"before the command name", []string{"--flags-file", one, "deploy"}, "", "", "us-east2-a", clix.SourceCommandFlag, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newFlagsFileApp(t)
			if tt.env != "" {
				t.Setenv("MYAPP_REGION", tt.env)
			}
			if tt.config != "" {
				a.app.Config.Set("region", tt.config)
			}
			var source clix.Source
			a.deploy.Run = func(ctx *clix.Context) error {
				var found bool
				if _, source, found = ctx.EffectiveString("region"); !found {
					source = clix.SourceDefault
				}
				return nil
			}
			if err := a.app.Run(context.Background(), tt.args); err != nil {
				t.Fatal(err)
			}
			if a.region != tt.wantRegion || source != tt.wantSource || strings.Join(a.tags, "|") != tt.wantTags {
				t.Errorf("region = %q (%v), tags = %v; want %q (%v), %q", a.region, source, a.tags, tt.wantRegion, tt.wantSource, tt.wantTags)
			}
			if wantChanged := tt.wantSource == clix.SourceCommandFlag; a.deploy.Flags.Changed("region") != wantChanged {
				t.Errorf("Changed(region) = %v, want %v", !wantChanged, wantChanged)
			}
		})
	}
}

// A root flag set from a flags file while another command runs is an app
// flag, as on the command line, and --flags-file itself records the files
// named on the command line.
func TestFlagsFileSources(t *testing.T) {
	path := writeTestFile(t, t.TempDir(), "f.yaml", "--project: p1\n")
	a := newFlagsFileApp(t)
	var source clix.Source
	a.deploy.Run = func(ctx *clix.Context) error {
		_, source, _ = ctx.EffectiveString("project")
		return nil
	}
	if err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", path}); err != nil {
		t.Fatal(err)
	}
	if source != clix.SourceAppFlag || !a.app.Flags().Changed("project") {
		t.Errorf("source = %v, Changed = %v", source, a.app.Flags().Changed("project"))
	}
	if got, _ := a.app.Flags().String("flags-file"); got != path || !a.app.Flags().Changed("flags-file") {
		t.Errorf("--flags-file = %q, Changed = %v", got, a.app.Flags().Changed("flags-file"))
	}
	// The next Run starts over.
	if err := a.app.Run(context.Background(), []string{"deploy"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.app.Flags().String("flags-file"); got != "" || a.project != "" {
		t.Errorf("after a second Run: --flags-file = %q, project = %q", got, a.project)
	}
}

// Flags files may name more flags files, relative to their own directory;
// a file may be read twice, but not inside itself.
func TestFlagsFileNested(t *testing.T) {
	t.Run("relative to the naming file", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "conf/common.yaml", "--project: shared\n--flags-file: more/region.yaml\n")
		writeTestFile(t, dir, "conf/more/region.yaml", "--region: nested\n--body: '@body.txt'\n")
		writeTestFile(t, dir, "conf/more/body.txt", "from beside region.yaml\n")
		top := writeTestFile(t, dir, "conf/deploy.yaml", "--flags-file: common.yaml\n--count: 2\n")
		a := newFlagsFileApp(t)
		if err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", top}); err != nil {
			t.Fatal(err)
		}
		if a.project != "shared" || a.region != "nested" || a.count != 2 || a.body != "from beside region.yaml" {
			t.Errorf("state %s", a.state())
		}
	})
	t.Run("in place, in order", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "a.yaml", "--region: from-a\n--tag: a\n")
		writeTestFile(t, dir, "b.yaml", "--region: from-b\n--tag: b\n")
		top := writeTestFile(t, dir, "top.yaml", "- --region: top-first\n- --flags-file: [a.yaml, b.yaml]\n- --tag: top\n")
		a := newFlagsFileApp(t)
		if err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", top}); err != nil {
			t.Fatal(err)
		}
		if a.region != "from-b" || strings.Join(a.tags, "|") != "a|b|top" {
			t.Errorf("state %s", a.state())
		}
	})
	t.Run("a diamond is not a cycle", func(t *testing.T) {
		dir := t.TempDir()
		writeTestFile(t, dir, "d.yaml", "--tag: d\n")
		writeTestFile(t, dir, "b.yaml", "--flags-file: d.yaml\n")
		writeTestFile(t, dir, "c.yaml", "--flags-file: d.yaml\n")
		top := writeTestFile(t, dir, "a.yaml", "--flags-file: [b.yaml, c.yaml]\n")
		a := newFlagsFileApp(t)
		if err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", top}); err != nil {
			t.Fatal(err)
		}
		if strings.Join(a.tags, "|") != "d|d" {
			t.Errorf("tags = %v", a.tags)
		}
	})

	cycles := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"itself", map[string]string{"a.yaml": "--flags-file: a.yaml\n"}, "a.yaml names itself"},
		{"through another", map[string]string{"a.yaml": "--flags-file: b.yaml\n", "b.yaml": "--region: x\n--flags-file: a.yaml\n"}, "a.yaml -> "},
		{"through a different spelling", map[string]string{"a.yaml": "--flags-file: ./sub/../a.yaml\n", "sub/keep": ""}, "names itself"},
		{"too deep", deepFlagsFiles(20), "nested more than 16 deep"},
	}
	for _, tt := range cycles {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				writeTestFile(t, dir, name, content)
			}
			a := newFlagsFileApp(t)
			err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", filepath.Join(dir, "a.yaml")})
			if err == nil || !strings.Contains(err.Error(), tt.want) || clix.ExitCode(err) != 2 {
				t.Fatalf("err = %v, want a usage error containing %q", err, tt.want)
			}
		})
	}
}

// deepFlagsFiles returns n flags files, a.yaml naming f1.yaml naming
// f2.yaml and so on.
func deepFlagsFiles(n int) map[string]string {
	files := map[string]string{"a.yaml": "--flags-file: f1.yaml\n"}
	for i := 1; i < n; i++ {
		files[fmt.Sprintf("f%d.yaml", i)] = fmt.Sprintf("--flags-file: f%d.yaml\n", i+1)
	}
	files[fmt.Sprintf("f%d.yaml", n)] = "--region: deep\n"
	return files
}

// Mistakes in a flags file are usage errors that name the file and line.
func TestFlagsFileErrors(t *testing.T) {
	tests := []struct {
		name string
		file string
		args []string
		want []string
	}{
		{"an unknown key", "--region: a\n--zone: b\n", []string{"deploy"}, []string{"flags.yaml, line 2: unknown flag --zone"}},
		{"an unknown key on the non-strict root", "--bogus: 1\n", nil, []string{"line 1: unknown flag --bogus"}},
		{"a command's flag before the command name is looked up there", "--region: a\n", []string{"--flags-file", "PATH", "deploy"}, nil},
		{"a command's flag for the root", "--region: a\n", []string{}, []string{"unknown flag --region"}},
		{"a duplicate key", "--region: a\n--count: 1\n--region: b\n", []string{"deploy"}, []string{"line 3: --region is already given on line 1", "use a list"}},
		{"a duplicate key with and without dashes", "region: a\n--region: b\n", []string{"deploy"}, []string{"line 2: --region is already given on line 1"}},
		{"a mapping value", "--region:\n  a: b\n", []string{"deploy"}, []string{"line 2: --region: a mapping is not a flag value"}},
		{"a list of lists", "--tag: [[a]]\n", []string{"deploy"}, []string{"--tag: each item of a list is one value"}},
		{"a short name", "-v: true\n", []string{"deploy"}, []string{`"-v" is not a long flag name`}},
		{"a key with =", "--region=x: y\n", []string{"deploy"}, []string{"is not a long flag name"}},
		{"a scalar document", "just text\n", []string{"deploy"}, []string{"line 1: a flags file is a mapping of flags to values"}},
		{"a list of scalars", "- a\n- b\n", []string{"deploy"}, []string{"each item of the list is a mapping"}},
		{"two documents", "--region: a\n---\n--count: 1\n", []string{"deploy"}, []string{"holds one YAML document"}},
		{"a merge key", "- &b {--region: a}\n- <<: *b\n  --count: 1\n", []string{"deploy"}, []string{"merge keys (<<) are not supported"}},
		{"bad YAML", "--region: [a\n", []string{"deploy"}, []string{"flags file", "yaml:"}},
		{"a value the flag rejects", "--region: a\n--count: many\n", []string{"deploy"}, []string{"line 2: invalid value for count", `"many"`}},
		{"no value for a flag that needs one", "--region:\n", []string{"deploy"}, []string{"line 1: flag --region requires a value"}},
		{"a nested file with no name", "--flags-file:\n", []string{"deploy"}, []string{"--flags-file needs a file name"}},
		{"a missing nested file", "--flags-file: nope.yaml\n", []string{"deploy"}, []string{"--flags-file:", "nope.yaml"}},
		{"an alias to an unknown key", "x: &r us-west1\n--region: *r\n", []string{"deploy"}, []string{"unknown flag --x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newFlagsFileApp(t)
			path := writeTestFile(t, t.TempDir(), "flags.yaml", tt.file)
			args := append([]string(nil), tt.args...)
			replaced := false
			for i, arg := range args {
				if arg == "PATH" {
					args[i], replaced = path, true
				}
			}
			if !replaced {
				args = append(args, "--flags-file", path)
			}
			err := a.app.Run(context.Background(), args)
			if tt.want == nil {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || clix.ExitCode(err) != 2 {
				t.Fatalf("err = %v, want a usage error", err)
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}

	t.Run("a missing file", func(t *testing.T) {
		a := newFlagsFileApp(t)
		err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", filepath.Join(t.TempDir(), "nope.yaml")})
		if err == nil || !strings.Contains(err.Error(), "--flags-file:") || clix.ExitCode(err) != 2 {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("no file name", func(t *testing.T) {
		for _, args := range [][]string{{"deploy", "--flags-file"}, {"deploy", "--flags-file="}} {
			a := newFlagsFileApp(t)
			if err := a.app.Run(context.Background(), args); err == nil || clix.ExitCode(err) != 2 {
				t.Errorf("%v: err = %v", args, err)
			}
		}
	})
	t.Run("not enabled", func(t *testing.T) {
		app, _ := newTestApp(t, "myapp")
		app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandRun(noop)))
		err := app.Run(context.Background(), []string{"deploy", "--flags-file", "x.yaml"})
		if err == nil || !strings.Contains(err.Error(), "unknown flag: --flags-file") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("after --", func(t *testing.T) {
		a := newFlagsFileApp(t)
		var rest []string
		a.deploy.Flags.ArgsVar(clix.ArgsVarOptions{Name: "rest", Value: &rest})
		if err := a.app.Run(context.Background(), []string{"deploy", "web", "--", "--flags-file", "x.yaml"}); err != nil {
			t.Fatal(err)
		}
		if strings.Join(rest, " ") != "--flags-file x.yaml" {
			t.Errorf("rest = %v", rest)
		}
	})
	t.Run("over the size limit", func(t *testing.T) {
		a := newFlagsFileApp(t)
		a.app.MaxFileSize = 8
		path := writeTestFile(t, t.TempDir(), "f.yaml", "--region: too-long\n")
		err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", path})
		if err == nil || !strings.Contains(err.Error(), "larger than 8 bytes") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("aliases cannot multiply values", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("- &m\n")
		for i := range 200 {
			fmt.Fprintf(&b, "  --tag%d: x\n", i)
		}
		for range 100 {
			b.WriteString("- *m\n")
		}
		a := newFlagsFileApp(t)
		path := writeTestFile(t, t.TempDir(), "f.yaml", b.String())
		err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", path})
		if err == nil || !strings.Contains(err.Error(), "more than 10000 flag values") {
			t.Fatalf("err = %v", err)
		}
	})
}

// A Sensitive flag's value from a flags file never appears in an error.
func TestFlagsFileSensitive(t *testing.T) {
	a := newFlagsFileApp(t)
	path := writeTestFile(t, t.TempDir(), "f.yaml", "--token: hunter2-SECRET\n")
	err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", path})
	if err == nil || strings.Contains(err.Error(), "SECRET") || !strings.Contains(err.Error(), "does not start with tok_") {
		t.Fatalf("err = %v", err)
	}
}

// Standard input may hold the flags, once per Run.
func TestFlagsFileStdin(t *testing.T) {
	a := newFlagsFileApp(t, clix.WithAppIn(strings.NewReader("--region: piped\n--body: '@@inline'\n")))
	if err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", "-"}); err != nil {
		t.Fatal(err)
	}
	if a.region != "piped" || a.body != "@inline" {
		t.Errorf("state %s", a.state())
	}

	a = newFlagsFileApp(t, clix.WithAppIn(strings.NewReader("--region: piped\n")))
	err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", "-", "--body=@-"})
	if err == nil || !strings.Contains(err.Error(), "standard input was already read by --flags-file") {
		t.Fatalf("err = %v", err)
	}
}

// Flags files decide prompting like the command line does: a flag from
// the file counts as passed unless it is Ambient, and --flags-file
// itself is Ambient.
func TestFlagsFilePrompting(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		wantPrompt bool
	}{
		{"only an Ambient flag", "--target: staging\n", true},
		{"an empty file", "", true},
		{"a flag that is not Ambient", "--note: hi\n", false},
		{"both", "--target: staging\n--note: hi\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newFlagsFileApp(t, clix.WithAppIn(strings.NewReader("prod\n")), clix.WithAppInteractive(clix.InteractiveAlways))
			var env string
			a.deploy.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "env", Required: true}, Value: &env})
			path := writeTestFile(t, t.TempDir(), "f.yaml", tt.file)
			err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", path})
			var missing *clix.MissingFlagsError
			switch {
			case tt.wantPrompt && (err != nil || env != "prod"):
				t.Errorf("err = %v, env = %q; want a prompt", err, env)
			case !tt.wantPrompt && !errors.As(err, &missing):
				t.Errorf("err = %v; want a *MissingFlagsError", err)
			}
		})
	}
}

// Flags that extensions register can be set from a flags file, and help
// lists --flags-file with the root's flags.
func TestFlagsFileExtensions(t *testing.T) {
	a := newFlagsFileApp(t)
	a.app.AddExtension(format.Extension{})
	a.app.AddExtension(help.Extension{})
	path := writeTestFile(t, t.TempDir(), "f.yaml", "--format: json\n")
	var got string
	a.deploy.Run = func(ctx *clix.Context) error {
		got = format.OutputFormat(ctx.App)
		return nil
	}
	if err := a.app.Run(context.Background(), []string{"deploy", "--flags-file", path}); err != nil {
		t.Fatal(err)
	}
	if got != "json" {
		t.Errorf("format = %q", got)
	}

	a.out.Reset()
	if err := a.app.Run(context.Background(), []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.out.String(), "--flags-file") {
		t.Errorf("help does not list --flags-file:\n%s", a.out)
	}
}

// An app flag named --flags-file wins over EnableFlagsFile's, and is an
// ordinary flag.
func TestFlagsFileAppFlagWins(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	var mine string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "flags-file"}, Value: &mine})
	app.EnableFlagsFile()
	app.EnableFlagsFile() // again: nothing changes
	app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandRun(noop)))
	if err := app.Run(context.Background(), []string{"deploy", "--flags-file", "not-read.yaml"}); err != nil {
		t.Fatal(err)
	}
	if mine != "not-read.yaml" || !app.FlagsFileEnabled() {
		t.Errorf("mine = %q", mine)
	}
}

// FlagSet.Parse inserts a flags file's flags too.
func TestFlagsFileParse(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	app.EnableFlagsFile()
	var region string
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "region"}, Value: &region})
	path := writeTestFile(t, t.TempDir(), "f.yaml", "--region: parsed\n")
	if _, err := app.Flags().Parse([]string{"--flags-file", path}); err != nil {
		t.Fatal(err)
	}
	if region != "parsed" {
		t.Errorf("region = %q", region)
	}
}
