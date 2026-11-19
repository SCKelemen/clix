package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func TestGitHubAuthLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := newApp()
	out := &bytes.Buffer{}
	app.Out = out
	app.Err = &bytes.Buffer{}
	app.In = strings.NewReader("")
	app.Prompter = clix.TextPrompter{In: app.In, Out: app.Out}

	if err := app.Run(context.Background(), []string{"auth", "login", "--hostname", "github.com", "--username", "monalisa"}); err != nil {
		t.Fatalf("app.Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Logging into github.com as monalisa") {
		t.Fatalf("expected login message in output, got %q", output)
	}
}

// The example passes App.Validate, as the README recommends checking.
func TestAppValidates(t *testing.T) {
	if err := newApp().Validate(); err != nil {
		t.Fatal(err)
	}
}

// "gh pr list" prints through format.Write: tab-separated values when Out
// is not a terminal, and gh's --json and --template.
func TestPullRequestList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
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
		{[]string{"pr", "ls", "--template", `{{range .}}#{{.number}} {{join "," .labels}}{{"\n"}}{{end}}`}, "#321 enhancement\n#318 bug,ui\n"},
	} {
		app := newApp()
		out := &bytes.Buffer{}
		app.Out = out
		app.Err = &bytes.Buffer{}
		if err := app.Run(context.Background(), tc.args); err != nil {
			t.Fatalf("%q: %v", tc.args, err)
		}
		if out.String() != tc.want {
			t.Errorf("%q:\ngot  %q\nwant %q", tc.args, out.String(), tc.want)
		}
	}
}
