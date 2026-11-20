package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// run runs a fresh app with args, and returns what it wrote to stdout.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	app := newApp(clix.WithAppIn(strings.NewReader("")), clix.WithAppOut(&out), clix.WithAppErr(&bytes.Buffer{}))
	err := app.Run(context.Background(), args)
	return out.String(), err
}

// isolate gives the test its own config directory, where aliases live.
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GH_FORMAT", "")
}

// The example passes App.Validate, as docs/testing.md recommends checking.
func TestAppValidates(t *testing.T) {
	if err := newApp().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAuthLogin(t *testing.T) {
	isolate(t)
	out, err := run(t, "auth", "login", "--hostname", "github.com", "--username", "monalisa")
	if err != nil || !strings.Contains(out, "Logging into github.com as monalisa") {
		t.Fatalf("auth login: %q, %v", out, err)
	}
}

// "gh pr list" prints through format.Write: tab-separated values when Out
// is not a terminal, and gh's --json, --jq and --template.
func TestPullRequestList(t *testing.T) {
	isolate(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"pr", "list"}, "321\tAdd --json, --jq and --template\toutput-engine\tOPEN\t2026-11-18T09:30:00Z\n" +
			"318\tFix tables on narrow terminals\tnarrow-tables\tMERGED\t2026-11-12T16:05:00Z\n"},
		{[]string{"pr", "list", "--json", "number,author.login"}, `[
  {
    "number": 321,
    "author": {
      "login": "monalisa"
    }
  },
  {
    "number": 318,
    "author": {
      "login": "hubot"
    }
  }
]
`},
		{[]string{"pr", "list", "--jq", `.[] | select(.state == "OPEN") | .title`}, "Add --json, --jq and --template\n"},
		{[]string{"pr", "ls", "--template", `{{range .}}#{{.number}} {{join "," .labels}}{{"\n"}}{{end}}`}, "#321 enhancement\n#318 bug,ui\n"},
		{[]string{"pr", "list", "--template", `{{range .}}{{tablerow (printf "#%v" .number) .title .author.login}}{{end}}`},
			"#321  Add --json, --jq and --template  monalisa\n#318  Fix tables on narrow terminals   hubot\n"},
	} {
		out, err := run(t, tc.args...)
		if err != nil {
			t.Fatalf("%q: %v", tc.args, err)
		}
		if out != tc.want {
			t.Errorf("%q:\ngot  %q\nwant %q", tc.args, out, tc.want)
		}
	}
	if _, err := run(t, "pr", "list", "--json"); clix.ExitCode(err) != 2 || !strings.Contains(err.Error(), "headRefName") {
		t.Errorf("--json without fields: %v, want a usage error listing the fields", err)
	}
}

// Aliases are stored in the config file and expand into commands.
func TestAliases(t *testing.T) {
	isolate(t)
	for _, args := range [][]string{
		{"alias", "set", "co", "pr checkout"},
		{"alias", "set", "titles", `pr list --jq .[].title`},
	} {
		if _, err := run(t, args...); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
	}
	if out, err := run(t, "co", "321"); err != nil || out != "Checking out PR #321\n" {
		t.Errorf("co 321: %q, %v", out, err)
	}
	if out, err := run(t, "titles"); err != nil || out != "Add --json, --jq and --template\nFix tables on narrow terminals\n" {
		t.Errorf("titles: %q, %v", out, err)
	}
	if out, err := run(t, "alias", "list"); err != nil || !strings.Contains(strings.Join(strings.Fields(out), " "), "co: pr checkout titles: pr list --jq .[].title") {
		t.Errorf("alias list: %q, %v", out, err)
	}
	// An alias never shadows a command, and never runs a shell.
	if _, err := run(t, "alias", "set", "pr", "repo clone"); err == nil {
		t.Error("alias set pr: an alias named like a command was accepted")
	}
	if _, err := run(t, "alias", "set", "x", "!rm -rf /"); err == nil {
		t.Error("alias set x '!...': a shell alias was accepted")
	}
}

// plugins/gh-hello runs as "gh hello" when its directory is on PATH.
func TestPlugin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("on Windows, only .exe and .com files run as plugins")
	}
	isolate(t)
	src, err := os.ReadFile(filepath.Join("..", "..", "plugins", "gh-hello"))
	if err != nil {
		t.Fatal(err)
	}
	// A copy with a known mode: the checkout's mode depends on the umask.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh-hello"), src, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	out, err := run(t, "hello", "world", "--loud")
	if err != nil || out != "Hello from hello, a plugin of gh: world --loud\n" {
		t.Errorf("hello world --loud: %q, %v", out, err)
	}
	out, err = run(t, "plugin", "list")
	if err != nil || !strings.Contains(strings.Join(strings.Fields(out), " "), "hello "+filepath.Join(dir, "gh-hello")) {
		t.Errorf("plugin list: %q, %v", out, err)
	}
	// A command wins over a plugin of the same name.
	if err := os.WriteFile(filepath.Join(dir, "gh-pr"), src, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, "pr", "list", "--jq", ".[0].number"); err != nil || out != "321\n" {
		t.Errorf("pr list with a gh-pr plugin on PATH: %q, %v", out, err)
	}
}
