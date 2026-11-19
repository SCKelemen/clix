package autocomplete

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// aliasEnv returns environment for the probe app with a config file that
// defines two aliases (see clix.App.ExpandAliases).
func aliasEnv(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "probe"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := "aliases:\n  zz-get: widgets get\n  zz-out: widgets get --output $1\n  widgets: rm\n"
	if err := os.WriteFile(filepath.Join(dir, "probe", "config.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	return []string{"XDG_CONFIG_HOME=" + dir}
}

// In-process: aliases are offered as the first word, and after one,
// completion continues as for its expansion.
func TestComplete_UserAliases(t *testing.T) {
	app, out := testApp(t)
	t.Setenv("XDG_CONFIG_HOME", strings.TrimPrefix(aliasEnv(t)[0], "XDG_CONFIG_HOME="))
	lines, _ := runComplete(t, app, out, "--cur=zz", "--", "probe")
	assertSet(t, "probe zz<TAB>", lines, []string{"zz-get\tAlias for \"widgets get\"", "zz-out\tAlias for \"widgets get --output $1\""})
	lines, _ = runComplete(t, app, out, "--cur=", "--", "probe", "zz-get")
	assertSet(t, "probe zz-get <TAB>", values(lines), []string{"alpha", "beta", "gamma ray"})
	lines, _ = runComplete(t, app, out, "--cur=", "--", "probe", "zz-out", "json")
	assertSet(t, "probe zz-out json <TAB>", values(lines), []string{"alpha", "beta", "gamma ray"})
	lines, directive := runComplete(t, app, out, "--cur=", "--", "probe", "zz-out")
	if len(lines) != 0 || directive != directiveNoFiles {
		t.Errorf("probe zz-out <TAB> (needs $1) = %q %s", lines, directive)
	}
	// The alias named like a command is never offered, and aliases are
	// only the first word.
	lines, _ = runComplete(t, app, out, "--cur=", "--", "probe")
	assertSet(t, "probe <TAB>", values(lines), append(append([]string(nil), topLevel...), "zz-get", "zz-out"))
	lines, _ = runComplete(t, app, out, "--cur=zz", "--", "probe", "-v")
	if len(lines) != 0 {
		t.Errorf("probe -v zz<TAB> = %q", lines)
	}
}
