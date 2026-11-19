package autocomplete

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func newQuotingApp(t *testing.T) (*clix.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out, errw := &bytes.Buffer{}, &bytes.Buffer{}
	app := clix.NewApp("moln", clix.WithAppOut(out), clix.WithAppErr(errw))
	app.Root = clix.NewGroup("moln", "root")
	app.AddExtension(Extension{})

	rm := clix.NewCommand("rm", clix.WithCommandRun(func(*clix.Context) error { return nil }))
	rm.Short = `Remove "things" [safely]: it's fine \ really`
	rm.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "yes", Short: "y", Usage: "Don't ask"}})
	app.Root.AddCommand(rm)

	secret := clix.NewCommand("internal-debug", clix.WithCommandRun(func(*clix.Context) error { return nil }))
	secret.Hidden = true
	secret.Flags.BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "secret-flag"}})
	app.Root.AddCommand(secret)
	return app, out, errw
}

// The scripts never contain the command tree: nothing a command is named
// or described as can end up as shell code.
func TestScriptsContainNoCommandTree(t *testing.T) {
	for _, shell := range shells {
		app, out, _ := newQuotingApp(t)
		if err := app.Run(context.Background(), []string{"autocomplete", "--shell", shell}); err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		for _, s := range []string{"internal-debug", "secret-flag", "Remove", "--yes", "eval"} {
			if strings.Contains(out.String(), s) {
				t.Errorf("%s script contains %q:\n%s", shell, s, out.String())
			}
		}
		if !strings.Contains(out.String(), "__complete") {
			t.Errorf("%s script does not call __complete", shell)
		}
	}
}

func TestCompletionOmitsHiddenCommands(t *testing.T) {
	app, out, _ := newQuotingApp(t)
	lines, _ := runComplete(t, app, out, "--cur=", "--", "moln")
	assertSet(t, "moln <TAB>", values(lines), []string{"autocomplete", "rm"})
}

func TestAutocompleteAcceptsPositionalShell(t *testing.T) {
	app, out, _ := newQuotingApp(t)
	if err := app.Run(context.Background(), []string{"autocomplete", "bash"}); err != nil {
		t.Fatalf("autocomplete bash: %v", err)
	}
	if !strings.Contains(out.String(), "complete -F '_moln_completions' 'moln'") {
		t.Fatalf("not a bash completion script:\n%s", out.String())
	}
}

// "app autocomplete bash > file" must give a clean script: only the
// script on standard output, nothing on standard error.
func TestAutocompleteScriptOnlyOnStdout(t *testing.T) {
	for _, shell := range append([]string{"pwsh"}, shells...) {
		app, out, errw := newQuotingApp(t)
		if err := app.Run(context.Background(), []string{"autocomplete", shell}); err != nil {
			t.Fatalf("autocomplete %s: %v", shell, err)
		}
		want, err := script("moln", shell)
		if err != nil {
			t.Fatal(err)
		}
		if out.String() != want {
			t.Errorf("autocomplete %s printed more than the script:\n%s", shell, out.String())
		}
		if errw.Len() != 0 {
			t.Errorf("autocomplete %s wrote to standard error: %q", shell, errw.String())
		}
	}
}

func TestAutocompleteHelpShowsInstallation(t *testing.T) {
	app, out, _ := newQuotingApp(t)
	if err := app.Run(context.Background(), []string{"autocomplete"}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"source <(moln autocomplete bash)", `"${fpath[1]}/_moln"`, "moln autocomplete fish | source", "moln autocomplete powershell"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("autocomplete help does not explain %q:\n%s", s, out.String())
		}
	}
}

// Names that are not identifiers still give valid function names, and
// quotes in the name stay quoted.
func TestScriptForOddAppName(t *testing.T) {
	name := `my app's "x"`
	for _, shell := range shells {
		text, err := script(name, shell)
		if err != nil {
			t.Fatal(err)
		}
		switch shell {
		case "bash":
			if !strings.Contains(text, `complete -F '_my_app_s__x__completions' 'my app'\''s "x"'`) {
				t.Errorf("bash registration:\n%s", text)
			}
		case "zsh":
			if !strings.Contains(text, `compdef _my_app_s__x_ 'my app'\''s "x"'`) || !strings.HasPrefix(text, "#compdef my_app's_\"x\"\n") {
				t.Errorf("zsh registration:\n%s", text)
			}
		case "fish":
			if !strings.Contains(text, `complete -c 'my app\'s "x"' -f -a '(__my_app_s__x__clix_complete)'`) {
				t.Errorf("fish registration:\n%s", text)
			}
		case "powershell":
			if !strings.Contains(text, `-CommandName 'my app''s "x"', 'my app''s "x".exe'`) {
				t.Errorf("powershell registration:\n%s", text)
			}
		}
	}
	// A newline in the name can only appear inside the quoted name, never
	// at the start of a line of code or after a comment.
	for _, shell := range shells {
		text, _ := script("evil\nrm -rf /", shell)
		if n := strings.Count(text, "\nrm -rf /"); n == 0 || n != strings.Count(text, "'evil\nrm -rf /") {
			t.Errorf("%s: a newline in the app name escapes its quoting:\n%s", shell, text)
		}
	}
}
