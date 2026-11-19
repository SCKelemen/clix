package autocomplete

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Regression tests for the flat completion scripts, which listed every
// command and flag of the whole tree at every position. They were probes
// that failed before completion was computed from the live tree.

// The bash script offered the app's own name, and every command and flag
// of the tree at every position.
func TestBug_BashCompletionIsFlatAndOffersAppName(t *testing.T) {
	bash := anyBash(t)
	path := writeScript(t, "bash", "probe.bash")
	top := bashComplete(t, bash, path, "probe ", "", "")
	if len(top) == 0 {
		t.Fatal("probe <TAB> offers nothing")
	}
	for _, w := range top {
		if w == "probe" {
			t.Errorf("`probe <TAB>` offers the app name itself: %q", top)
		}
		if w == "get" || w == "--watch" || w == "--force" {
			t.Errorf("`probe <TAB>` offers %q, which only exists under a subcommand: %q", w, top)
		}
	}
	for _, w := range bashComplete(t, bash, path, "probe -", "", "") {
		if w == "--watch" || w == "--force" {
			t.Errorf("`probe -<TAB>` offers the subcommand flag %q", w)
		}
	}
	nested := bashComplete(t, bash, path, "probe widgets ", "", "")
	assertSet(t, "probe widgets <TAB>", nested, []string{"get", "list", "ls"})
	flags := bashComplete(t, bash, path, "probe widgets get --", "", "")
	for _, w := range flags {
		if w == "--force" {
			t.Errorf("`probe widgets get --<TAB>` offers rm's --force: %q", flags)
		}
	}
}

// Command names were interpolated unquoted into compgen -W "...": a double
// quote broke the script, and $(...) ran at completion time.
func TestSharp_BashCompletionUnquotedTokens(t *testing.T) {
	bash := anyBash(t)
	path := writeScript(t, "bash", "probe.bash")
	if out, err := exec.Command(bash.path, "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("bash -n rejects the script: %s", out)
	}
	text, _ := os.ReadFile(path)
	for _, name := range weirdNames {
		if strings.Contains(string(text), name) {
			t.Errorf("the script contains the command name %q", name)
		}
	}
	got := bashComplete(t, bash, path, "probe weird ", "", "")
	assertSet(t, "probe weird <TAB>", got, []string{`say\"hi`, `it\'s`, `\$\(touch\ pwned\)`, `two\ words`, "x:y", `back\\slash`})
	assertNotPwned(t)

	// Each candidate reads back as exactly the command name.
	for i, quoted := range got {
		out, err := exec.Command(bash.path, "--norc", "--noprofile", "-c", "printf '%s' "+quoted).Output()
		if err != nil {
			t.Fatalf("bash reading %s: %v", quoted, err)
		}
		if !contains(weirdNames, string(out)) {
			t.Errorf("candidate %d %s reads back as %q, which is no command", i, quoted, out)
		}
	}
	assertNotPwned(t)
}

// zsh: `_arguments '*: :->cmds'` always set state=cmds, so the case arm
// holding the flags was unreachable and flags never completed.
func TestBug_ZshFlagsBranchUnreachable(t *testing.T) {
	text, err := script("probe", "zsh")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "->cmds") {
		t.Fatalf("the zsh script still routes everything to one state:\n%s", text)
	}
	// The flags of the command at the cursor are what zsh gets.
	app, out := testApp(t)
	lines, _ := runComplete(t, app, out, "--shell", "zsh", "--cur=--", "--", "probe", "widgets", "get")
	for _, want := range []string{"--watch:Keep watching for changes", "--output:Output format", "--all:Include archived widgets"} {
		if !contains(lines, want) {
			t.Errorf("zsh probe widgets get --<TAB> lacks %q: %q", want, lines)
		}
	}
	// And a real zsh lists them (see TestShell_Zsh_* for the full matrix).
	zshListsFlags(t)
}

// fish: every completion was guarded by __fish_use_subcommand, so nothing
// completed after the first subcommand.
func TestBug_FishCompletionOnlyTopLevel(t *testing.T) {
	text, err := script("probe", "fish")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "__fish_use_subcommand") {
		t.Fatalf("the fish script only completes before the first subcommand:\n%s", text)
	}
	app, out := testApp(t)
	lines, _ := runComplete(t, app, out, "--shell", "fish", "--cur=", "--", "probe", "widgets")
	assertSet(t, "fish probe widgets <TAB>", values(lines), []string{"get", "list", "ls"})
	lines, _ = runComplete(t, app, out, "--shell", "fish", "--cur=--w", "--", "probe", "widgets", "get")
	assertSet(t, "fish probe widgets get --w<TAB>", values(lines), []string{"--watch"})

	fish, err := exec.LookPath("fish")
	if err != nil {
		t.Log("fish is not installed; TestShell_Fish covers a real fish in CI")
		return
	}
	_, _, work := shellEnv(t)
	path := filepath.Join(t.TempDir(), "probe.fish")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(fish, "--no-config", "-c", `source $CLIX_SCRIPT; and complete --do-complete='probe widgets '`)
	cmd.Env = append(shellEnviron(t), "CLIX_SCRIPT="+path)
	cmd.Dir = work
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fish: %v\n%s", err, got)
	}
	if !strings.Contains(string(got), "get\t") {
		t.Errorf("fish does not complete after a subcommand:\n%s", got)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
