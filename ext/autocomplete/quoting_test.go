package autocomplete

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

func newQuotingApp(t *testing.T) (*clix.App, *bytes.Buffer) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("moln", clix.WithAppOut(out))
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
	return app, out
}

func TestCompletionOmitsHiddenCommands(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		app, out := newQuotingApp(t)
		if err := app.Run(context.Background(), []string{"autocomplete", "--shell", shell}); err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		if strings.Contains(out.String(), "internal-debug") || strings.Contains(out.String(), "secret-flag") {
			t.Errorf("%s completion lists a hidden command:\n%s", shell, out.String())
		}
		if !strings.Contains(out.String(), "rm") {
			t.Errorf("%s completion is missing the rm command", shell)
		}
	}
}

func TestCompletionScriptsAreValidShell(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		t.Run(shell, func(t *testing.T) {
			bin, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s not installed", shell)
			}
			app, out := newQuotingApp(t)
			if err := app.Run(context.Background(), []string{"autocomplete", "--shell", shell}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "completion."+shell)
			if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			if msg, err := exec.Command(bin, "-n", path).CombinedOutput(); err != nil {
				t.Fatalf("%s -n rejected the script: %v\n%s\n--- script ---\n%s", shell, err, msg, out.String())
			}
		})
	}
}

func TestShellQuoting(t *testing.T) {
	if got := shellSingleQuote(`it's`); got != `'it'\''s'` {
		t.Errorf("shellSingleQuote = %s", got)
	}
	if got := fishQuote(`it's \ ok`); got != `'it\'s \\ ok'` {
		t.Errorf("fishQuote = %s", got)
	}
	if got := zshValue(completionEntry{Token: "rm", Help: "a [b]: c"}); got != `'rm[a \[b\]\: c]'` {
		t.Errorf("zshValue = %s", got)
	}
}

func TestAutocompleteAcceptsPositionalShell(t *testing.T) {
	app, out := newQuotingApp(t)
	if err := app.Run(context.Background(), []string{"autocomplete", "bash"}); err != nil {
		t.Fatalf("autocomplete bash: %v", err)
	}
	if !strings.Contains(out.String(), "complete -F _moln_completions moln") {
		t.Fatalf("not a bash completion script:\n%s", out.String())
	}
}
