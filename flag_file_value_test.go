package clix_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// writeTestFile writes content to name in dir and returns its path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// fileValueApp returns an app whose "post" command has --body (-b,
// ValueFromFile), --plain (without it), --count (int, ValueFromFile),
// --tag (slice, ValueFromFile) and a positional "title" (ValueFromFile).
type fileValueApp struct {
	app   *clix.App
	cmd   *clix.Command
	body  string
	plain string
	count int
	tags  []string
	title string
}

func newFileValueApp(t *testing.T, stdin string) *fileValueApp {
	t.Helper()
	a := &fileValueApp{}
	a.app, _ = newTestApp(t, "myapp", clix.WithAppIn(strings.NewReader(stdin)))
	a.cmd = clix.NewCommand("post", clix.WithCommandRun(noop))
	a.cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "body", Short: "b", ValueFromFile: true}, Value: &a.body})
	a.cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "plain"}, Value: &a.plain})
	a.cmd.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "count"}, Value: &a.count}, clix.WithFlagValueFromFile())
	a.cmd.Flags.StringSliceVar(clix.StringSliceVarOptions{FlagOptions: clix.FlagOptions{Name: "tag", ValueFromFile: true}, Value: &a.tags})
	a.cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "title", Positional: true, ValueFromFile: true}, Value: &a.title})
	a.app.Root.AddCommand(a.cmd)
	return a
}

// --name=@path reads the file, @- reads standard input, @@ escapes a
// leading @, and one trailing line ending is removed.
func TestValueFromFile(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"body.txt":   "hello\n",
		"crlf.txt":   "line one\r\nline two\r\n",
		"two.txt":    "kept\n\n",
		"none.txt":   "no newline",
		"empty.txt":  "",
		"lone.txt":   "\n",
		"count.txt":  "42\n",
		"tags.txt":   "a, \"b,c\"\n",
		"title.txt":  "A title\n",
		"spaces.txt": "  padded  \n",
	}
	for name, content := range files {
		writeTestFile(t, dir, name, content)
	}
	at := func(name string) string { return "@" + filepath.Join(dir, name) }

	tests := []struct {
		name  string
		args  []string
		stdin string
		check func(*fileValueApp) string // returns what was set, to compare with want
		want  string
	}{
		{"long with =", []string{"post", "--body=" + at("body.txt")}, "", bodyOf, "hello"},
		{"long with space", []string{"post", "--body", at("body.txt")}, "", bodyOf, "hello"},
		{"short attached", []string{"post", "-b" + at("body.txt")}, "", bodyOf, "hello"},
		{"short with space", []string{"post", "-b", at("body.txt")}, "", bodyOf, "hello"},
		{"CRLF: only the last line ending goes", []string{"post", "--body=" + at("crlf.txt")}, "", bodyOf, "line one\r\nline two"},
		{"two line endings keep one", []string{"post", "--body=" + at("two.txt")}, "", bodyOf, "kept\n"},
		{"no line ending", []string{"post", "--body=" + at("none.txt")}, "", bodyOf, "no newline"},
		{"empty file", []string{"post", "--body=" + at("empty.txt")}, "", bodyOf, ""},
		{"a lone newline is empty", []string{"post", "--body=" + at("lone.txt")}, "", bodyOf, ""},
		{"spaces are kept", []string{"post", "--body=" + at("spaces.txt")}, "", bodyOf, "  padded  "},
		{"standard input", []string{"post", "--body=@-"}, "from stdin\n", bodyOf, "from stdin"},
		{"standard input with CRLF", []string{"post", "--body=@-"}, "piped\r\n", bodyOf, "piped"},
		{"@@ is one @", []string{"post", "--body=@@team"}, "", bodyOf, "@team"},
		{"@@@ is two", []string{"post", "--body=@@@x"}, "", bodyOf, "@@x"},
		{"@@- is the text @-", []string{"post", "--body=@@-"}, "unread", bodyOf, "@-"},
		{"only a leading @ is special", []string{"post", "--body=a@b"}, "", bodyOf, "a@b"},
		{"a flag without ValueFromFile", []string{"post", "--plain=" + at("body.txt")}, "", func(a *fileValueApp) string { return a.plain }, at("body.txt")},
		{"an int flag", []string{"post", "--count=" + at("count.txt")}, "", func(a *fileValueApp) string { return strconv.Itoa(a.count) }, "42"},
		{"a slice flag splits the contents", []string{"post", "--tag=" + at("tags.txt"), "--tag", "d"}, "", func(a *fileValueApp) string { return strings.Join(a.tags, "|") }, "a|b,c|d"},
		{"a positional argument", []string{"post", at("title.txt")}, "", func(a *fileValueApp) string { return a.title }, "A title"},
		{"a positional @@", []string{"post", "@@me"}, "", func(a *fileValueApp) string { return a.title }, "@me"},
		{"the value after --", []string{"post", "--", at("title.txt")}, "", func(a *fileValueApp) string { return a.title }, "A title"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newFileValueApp(t, tt.stdin)
			if err := a.app.Run(context.Background(), tt.args); err != nil {
				t.Fatal(err)
			}
			if got := tt.check(a); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func bodyOf(a *fileValueApp) string { return a.body }

// Bad @file values are usage errors that say what went wrong.
func TestValueFromFileErrors(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "word.txt", "twelve\n")
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  []string
	}{
		{"@ alone", []string{"post", "--body=@"}, "", []string{"--body: name a file after @", "write @@"}},
		{"a missing file", []string{"post", "--body=@" + filepath.Join(dir, "missing.txt")}, "", []string{"--body:", "missing.txt"}},
		{"a directory", []string{"post", "--body=@" + dir}, "", []string{"is a directory"}},
		{"standard input twice", []string{"post", "--body=@-", "--count=@-"}, "1\n", []string{"--count:", "standard input was already read by --body"}},
		{"a value the flag rejects names the file", []string{"post", "--count=@" + filepath.Join(dir, "word.txt")}, "", []string{"invalid value for count (read from " + filepath.Join(dir, "word.txt") + ")", `"twelve"`}},
		{"a positional @ alone", []string{"post", "--count", "1", "@"}, "", []string{"title: name a file after @"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newFileValueApp(t, tt.stdin)
			err := a.app.Run(context.Background(), tt.args)
			if err == nil {
				t.Fatal("expected an error")
			}
			if clix.ExitCode(err) != 2 {
				t.Errorf("exit code %d, want 2 (usage): %v", clix.ExitCode(err), err)
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

// Values from the environment, the config file and defaults are used as
// written: they never read a file.
func TestValueFromFileOnlyOnTheCommandLine(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "secret.txt", "file contents\n")

	t.Run("environment", func(t *testing.T) {
		a := newFileValueApp(t, "")
		t.Setenv("MYAPP_BODY", "@"+path)
		if err := a.app.Run(context.Background(), []string{"post"}); err != nil {
			t.Fatal(err)
		}
		if a.body != "@"+path {
			t.Errorf("body = %q, want the variable as written", a.body)
		}
	})
	t.Run("config file", func(t *testing.T) {
		a := newFileValueApp(t, "")
		a.app.Config.Set("body", "@"+path)
		if err := a.app.Run(context.Background(), []string{"post"}); err != nil {
			t.Fatal(err)
		}
		if a.body != "@"+path {
			t.Errorf("body = %q, want the config value as written", a.body)
		}
	})
	t.Run("default", func(t *testing.T) {
		app, _ := newTestApp(t, "myapp")
		var body string
		cmd := clix.NewCommand("post", clix.WithCommandRun(noop))
		cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "body", ValueFromFile: true}, Default: "@" + path, Value: &body})
		app.Root.AddCommand(cmd)
		if err := app.Run(context.Background(), []string{"post"}); err != nil {
			t.Fatal(err)
		}
		if body != "@"+path {
			t.Errorf("body = %q, want the default as written", body)
		}
	})
	t.Run("prompt", func(t *testing.T) {
		app, _ := newTestApp(t, "myapp", clix.WithAppIn(strings.NewReader("@"+path+"\n")), clix.WithAppInteractive(clix.InteractiveAlways))
		var body string
		cmd := clix.NewCommand("post", clix.WithCommandRun(noop))
		cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "body", Required: true, ValueFromFile: true}, Value: &body})
		app.Root.AddCommand(cmd)
		if err := app.Run(context.Background(), []string{"post"}); err != nil {
			t.Fatal(err)
		}
		if body != "@"+path {
			t.Errorf("body = %q, want the answer as typed", body)
		}
	})
}

// A value read from a file is a command-line value: it beats the
// environment and the config file, counts for Changed and Source, and
// counts as a flag passed, so Run does not prompt, unless the flag is
// Ambient.
func TestValueFromFileIsACommandLineValue(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "body.txt", "from file\n")

	a := newFileValueApp(t, "")
	t.Setenv("MYAPP_BODY", "from env")
	a.app.Config.Set("body", "from config")
	var source clix.Source
	a.cmd.Run = func(ctx *clix.Context) error {
		_, source, _ = ctx.EffectiveString("body")
		return nil
	}
	if err := a.app.Run(context.Background(), []string{"post", "--body=@" + path}); err != nil {
		t.Fatal(err)
	}
	if a.body != "from file" || !a.cmd.Flags.Changed("body") || source != clix.SourceCommandFlag {
		t.Errorf("body = %q, Changed = %v, source = %v", a.body, a.cmd.Flags.Changed("body"), source)
	}

	for _, ambient := range []bool{false, true} {
		app, _ := newTestApp(t, "myapp", clix.WithAppIn(strings.NewReader("answer\n")), clix.WithAppInteractive(clix.InteractiveAlways))
		var body, name string
		cmd := clix.NewCommand("post", clix.WithCommandRun(noop))
		cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "body", ValueFromFile: true, Ambient: ambient}, Value: &body})
		cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "name", Required: true}, Value: &name})
		app.Root.AddCommand(cmd)
		err := app.Run(context.Background(), []string{"post", "--body=@" + path})
		var missing *clix.MissingFlagsError
		switch {
		case ambient && (err != nil || name != "answer"):
			t.Errorf("Ambient: err = %v, name = %q; want a prompt", err, name)
		case !ambient && !errors.As(err, &missing):
			t.Errorf("not Ambient: err = %v; want a *MissingFlagsError", err)
		}
	}
}

// Files and standard input are read up to App.MaxFileSize.
func TestValueFromFileSizeLimit(t *testing.T) {
	dir := t.TempDir()
	exact := writeTestFile(t, dir, "exact.txt", "1234")
	over := writeTestFile(t, dir, "over.txt", "12345")

	tests := []struct {
		name    string
		limit   int64
		args    []string
		stdin   io.Reader
		wantErr string
	}{
		{"at the limit", 4, []string{"post", "--body=@" + exact}, nil, ""},
		{"over the limit", 4, []string{"post", "--body=@" + over}, nil, over + " is larger than 4 bytes"},
		{"standard input over the limit", 4, []string{"post", "--body=@-"}, strings.NewReader("12345"), "standard input is larger than 4 bytes"},
		{"endless standard input", 1024, []string{"post", "--body=@-"}, endless{}, "standard input is larger than 1024 bytes"},
		{"the default limit", 0, []string{"post", "--body=@-"}, endless{}, "standard input is larger than 1048576 bytes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newFileValueApp(t, "")
			if tt.stdin != nil {
				a.app.In = tt.stdin
			}
			a.app.MaxFileSize = tt.limit
			err := a.app.Run(context.Background(), tt.args)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) || clix.ExitCode(err) != 2 {
				t.Fatalf("err = %v, want a usage error containing %q", err, tt.wantErr)
			}
		})
	}
}

// endless is a reader that never ends, like /dev/zero.
type endless struct{}

func (endless) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

// An error about a Sensitive value read from a file never shows the
// file's contents: not from Value.Set, not from Validate, not one line of
// a multi-line secret.
func TestValueFromFileSensitiveRedaction(t *testing.T) {
	dir := t.TempDir()
	pin := writeTestFile(t, dir, "pin.txt", "hunter2-SECRET\n")
	token := writeTestFile(t, dir, "token.txt", "bad-SECRET-token\n")
	multi := writeTestFile(t, dir, "multi.txt", "-----BEGIN KEY-----\nAAAASECRETLINE\n-----END KEY-----\n")

	tests := []struct {
		name string
		args []string
		want string // must appear, to show the error still says what is wrong
	}{
		{"Set error", []string{"login", "--pin=@" + pin}, "not a valid integer"},
		{"Validate error echoing the value", []string{"login", "--token=@" + token}, "does not start with tok_"},
		{"Validate error echoing one line", []string{"login", "--token=@" + multi}, "does not start with tok_"},
		{"standard input", []string{"login", "--pin=@-"}, "not a valid integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _ := newTestApp(t, "myapp", clix.WithAppIn(strings.NewReader("12-SECRET\n")))
			cmd := clix.NewCommand("login", clix.WithCommandRun(noop))
			cmd.Flags.IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "pin", Sensitive: true, ValueFromFile: true}})
			cmd.Flags.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "token", Sensitive: true, ValueFromFile: true, Validate: func(v string) error {
				if !strings.HasPrefix(v, "tok_") {
					lines := strings.Split(v, "\n")
					return errors.New("token " + v + " (second line " + lines[len(lines)/2] + ") does not start with tok_")
				}
				return nil
			}}})
			app.Root.AddCommand(cmd)
			err := app.Run(context.Background(), tt.args)
			if err == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(err.Error(), "SECRET") || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q: want %q and no SECRET", err, tt.want)
			}
		})
	}
}

// FlagSet.Parse and MapPositionals read @file values too.
func TestValueFromFileOutsideRun(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "v.txt", "parsed\n")
	fs := clix.NewFlagSet("test")
	var body, title string
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "body", ValueFromFile: true}, Value: &body})
	fs.StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "title", Positional: true, ValueFromFile: true}, Value: &title})
	rest, err := fs.Parse([]string{"--body", "@" + path, "@" + path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.MapPositionals(rest); err != nil {
		t.Fatal(err)
	}
	if body != "parsed" || title != "parsed" {
		t.Errorf("body = %q, title = %q", body, title)
	}
}

// Paths are used as written. On Windows that includes drive letters and
// backslashes; elsewhere a backslash is an ordinary character in a file
// name, so the same text names a file in the current directory.
func TestValueFromFileWindowsPaths(t *testing.T) {
	dir := t.TempDir()
	var path string
	if filepath.Separator == '\\' {
		path = writeTestFile(t, dir, `sub\body.txt`, "windows\r\n")
	} else {
		t.Chdir(dir)
		path = `C:\Users\me\body.txt`
		if err := os.WriteFile(path, []byte("windows\r\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	a := newFileValueApp(t, "")
	if err := a.app.Run(context.Background(), []string{"post", "--body=@" + path}); err != nil {
		t.Fatal(err)
	}
	if a.body != "windows" {
		t.Errorf("body = %q, want %q", a.body, "windows")
	}
}

// A second Run does not keep what @file set in the first.
func TestValueFromFileRunTwice(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "v.txt", "first\n")
	a := newFileValueApp(t, "")
	if err := a.app.Run(context.Background(), []string{"post", "--body=@" + path}); err != nil {
		t.Fatal(err)
	}
	if err := a.app.Run(context.Background(), []string{"post"}); err != nil {
		t.Fatal(err)
	}
	if a.body != "" || a.cmd.Flags.Changed("body") {
		t.Errorf("body = %q, Changed = %v after a second Run", a.body, a.cmd.Flags.Changed("body"))
	}
}
