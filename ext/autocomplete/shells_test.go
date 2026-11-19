package autocomplete

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// requireEnv lists the shells (comma-separated: bash32, bash5,
// bash-completion, zsh, fish, pwsh) whose absence fails the tests instead
// of skipping them. CI sets it so a shell that failed to install cannot
// make the tests pass silently.
const requireEnv = "CLIX_COMPLETION_SHELLS"

// skipShell skips the test because shell is missing, or fails it when
// requireEnv asks for that shell.
func skipShell(t *testing.T, shell, why string) {
	t.Helper()
	for _, s := range strings.Split(os.Getenv(requireEnv), ",") {
		if strings.TrimSpace(s) == shell {
			t.Fatalf("%s is required (%s=%s) but %s", shell, requireEnv, os.Getenv(requireEnv), why)
		}
	}
	t.Skipf("%s: %s", shell, why)
}

// unixShells skips the test on Windows: the shells run the probe app
// through a /bin/sh wrapper script.
func unixShells(t *testing.T, shell string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		skipShell(t, shell, "shell tests run on Unix only")
	}
}

type bashInfo struct {
	path         string
	major, minor int
	version      string
}

var bashes struct {
	once  sync.Once
	found []bashInfo
}

// findBashes returns the distinct bash binaries installed in the usual
// places and on PATH, with their versions.
func findBashes() []bashInfo {
	bashes.once.Do(func() {
		candidates := []string{"/bin/bash", "/usr/bin/bash", "/usr/local/bin/bash", "/opt/homebrew/bin/bash"}
		if p, err := exec.LookPath("bash"); err == nil {
			candidates = append(candidates, p)
		}
		seen := map[string]bool{}
		for _, p := range candidates {
			real, err := filepath.EvalSymlinks(p)
			if err != nil || seen[real] {
				continue
			}
			seen[real] = true
			out, err := exec.Command(p, "--norc", "--noprofile", "-c", `echo "${BASH_VERSINFO[0]} ${BASH_VERSINFO[1]} ${BASH_VERSION}"`).Output()
			if err != nil {
				continue
			}
			f := strings.Fields(string(out))
			if len(f) != 3 {
				continue
			}
			major, _ := strconv.Atoi(f[0])
			minor, _ := strconv.Atoi(f[1])
			bashes.found = append(bashes.found, bashInfo{path: p, major: major, minor: minor, version: f[2]})
		}
	})
	return bashes.found
}

// needBash returns a bash 3.x (key "bash32") or a bash 4 or later (key
// "bash5"), or skips the test.
func needBash(t *testing.T, key string) bashInfo {
	t.Helper()
	unixShells(t, key)
	for _, b := range findBashes() {
		if (key == "bash32") == (b.major == 3) {
			t.Logf("using %s (bash %s)", b.path, b.version)
			return b
		}
	}
	skipShell(t, key, "no such bash is installed")
	return bashInfo{}
}

// anyBash returns any installed bash, or skips the test.
func anyBash(t *testing.T) bashInfo {
	t.Helper()
	unixShells(t, "bash")
	if found := findBashes(); len(found) > 0 {
		return found[len(found)-1]
	}
	skipShell(t, "bash", "bash is not installed")
	return bashInfo{}
}

// bashCompletionScript returns the main script of the bash-completion
// package, or "".
func bashCompletionScript() string {
	for _, p := range []string{
		"/usr/share/bash-completion/bash_completion",
		"/opt/homebrew/share/bash-completion/bash_completion",
		"/usr/local/share/bash-completion/bash_completion",
		"/etc/bash_completion",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// readlineWord returns the text readline replaces when completing at the
// end of line, which bash passes to the completion function as $2: the
// text after an open quote, or else after the last unquoted character of
// the default COMP_WORDBREAKS (blanks and "'><=;|&(:).
func readlineWord(line string) string {
	start := 0
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
				start = i + 1
			} else if c == '\\' && quote == '"' {
				i++
			}
		case c == '\\':
			i++
		case c == '\'' || c == '"':
			quote = c
			start = i + 1
		case strings.IndexByte(" \t\n><=;|&(:", c) >= 0:
			start = i + 1
		}
	}
	if start > len(line) {
		start = len(line)
	}
	return line[start:]
}

// bashHarness sources the script (after bash-completion, when bc is set)
// and calls the completion function the way bash does for line: with
// COMP_LINE, COMP_POINT, COMP_WORDS and COMP_CWORD set and readline's word
// as $2. Mode "nocompline" leaves COMP_LINE unset and passes no
// arguments, like an older caller.
const bashHarness = `
if [[ -n $4 ]]; then
    source "$4" || exit 4
    [[ -n ${BASH_COMPLETION_VERSINFO-} ]] || exit 5
fi
source "$1" || exit 3
COMP_WORDS=()
read -r -a COMP_WORDS <<< "$2"
case $2 in *' ') COMP_WORDS[${#COMP_WORDS[@]}]= ;; esac
COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))
if [[ $5 == nocompline ]]; then
    _probe_completions
else
    COMP_LINE=$2
    COMP_POINT=${#COMP_LINE}
    _probe_completions probe "$3" "${COMP_WORDS[COMP_CWORD-1]}"
fi
for w in "${COMPREPLY[@]}"; do
    printf '%s\n' "$w"
done
`

// bashComplete returns COMPREPLY for line, in the shell environment plus
// extra.
func bashComplete(t *testing.T, bash bashInfo, scriptPath, line, bc, mode string, extra ...string) []string {
	t.Helper()
	_, _, work := shellEnv(t)
	cmd := exec.Command(bash.path, "--norc", "--noprofile", "-c", bashHarness, "bash", scriptPath, line, readlineWord(line), bc, mode)
	cmd.Env = shellEnviron(t, extra...)
	cmd.Dir = work
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash %s: %v\n%s", bash.version, err, out)
	}
	var reply []string
	for _, l := range strings.Split(string(out), "\n") {
		if l != "" {
			reply = append(reply, l)
		}
	}
	return reply
}

type bashCase struct {
	name, line string
	want       []string
	mode       string
}

var allFlagsOfGet = []string{"--watch", "-w", "--output", "-o", "--name", "--help", "-h", "--all", "-a", "--verbose", "-v", "--org"}

// bashCases are the behaviors checked in every bash. want is COMPREPLY
// exactly: values escaped for bash, and cut to readline's word.
var bashCases = []bashCase{
	{"TopLevel", "probe ", topLevel, ""},
	{"Prefix", "probe w", []string{"weird", "widgets", "w"}, ""},
	{"NestedCommands", "probe widgets ", []string{"get", "list", "ls"}, ""},
	{"Aliases", "probe w ", []string{"get", "list", "ls"}, ""},
	{"AliasPrefix", "probe widgets l", []string{"list", "ls"}, ""},
	{"FlagsInScope", "probe widgets get -", allFlagsOfGet, ""},
	{"FlagsOutOfScope", "probe rm --", []string{"--force", "--dry-run", "--path", "--help", "--verbose", "--org"}, ""},
	{"GivenFlagsLeftOut", "probe widgets get --watch --", []string{"--output", "--name", "--help", "--all", "--verbose", "--org"}, ""},
	{"MutuallyExclusiveLeftOut", "probe rm --dry-run --", []string{"--path", "--help", "--verbose", "--org"}, ""},
	{"RepeatableFlag", "probe cp --tag red --", []string{"--tag", "--files", "--help", "--verbose", "--org"}, ""},
	{"FlagValueHook", "probe widgets get --output ", []string{"json", "yaml"}, ""},
	{"FlagValueEquals", "probe widgets get --output=", []string{"json", "yaml"}, ""},
	{"FlagValueEqualsPrefix", "probe widgets get --output=y", []string{"yaml"}, ""},
	{"BoolValueEquals", "probe widgets get --watch=", []string{"true", "false"}, ""},
	{"RootFlagValue", "probe --org ", []string{"acme", "initech"}, ""},
	{"RootFlagValueConsumed", "probe --org acme ", topLevel, ""},
	{"PositionalHook", "probe widgets get ", []string{"alpha", "beta", `gamma\ ray`}, ""},
	{"PositionalDoubleQuoted", `probe widgets get "gam`, []string{"gamma ray"}, ""},
	{"PositionalSingleQuoted", `probe widgets get 'gam`, []string{"gamma ray"}, ""},
	{"PositionalEscaped", `probe widgets get gamma\ `, []string{`gamma\ ray`}, ""},
	{"AfterDoubleDash", "probe rm -- -", []string{"-x.txt"}, ""},
	{"AfterDoubleDashValues", "probe widgets get -- ", []string{"alpha", "beta", `gamma\ ray`}, ""},
	{"FileFallback", "probe rm ", []string{"dir-a/", "evil/", "file-one.txt", "-x.txt", `sp\ ace.txt`}, ""},
	{"FileFallbackPrefix", "probe rm sp", []string{`sp\ ace.txt`}, ""},
	{"FileFallbackEscaped", `probe rm sp\ a`, []string{`sp\ ace.txt`}, ""},
	// A lone directory would get a trailing space in bash 3.2, so its
	// entries are offered instead.
	{"DirectoryEntries", "probe rm di", []string{"dir-a/inner.txt", "dir-a/other.txt"}, ""},
	{"FlagValueFile", "probe rm --path=fi", []string{"file-one.txt"}, ""},
	// File names are quoted for where the word starts, so $(...) and
	// backquotes in them are never run.
	{"FileDollarBare", "probe rm evil/zz", []string{`evil/zz\$\(touch\ pwned\).txt`}, ""},
	{"FileDollarDoubleQuoted", `probe rm "evil/zz`, []string{`evil/zz\$(touch pwned).txt`}, ""},
	{"FileDollarSingleQuoted", `probe rm 'evil/zz`, []string{`evil/zz$(touch pwned).txt`}, ""},
	{"FileBackquoteDoubleQuoted", `probe rm "evil/yy`, []string{"evil/yy\\`touch pwned\\`.txt"}, ""},
	{"FileBackquoteBare", "probe rm evil/yy", []string{"evil/yy\\`touch\\ pwned\\`.txt"}, ""},
	{"HiddenTyped", "probe internal-debug --s", []string{"--secret-flag"}, ""},
	{"WeirdNames", "probe weird ", []string{`say\"hi`, `it\'s`, `\$\(touch\ pwned\)`, `two\ words`, "x:y", `back\\slash`}, ""},
	{"WeirdSingleQuoted", "probe weird 'it", []string{`it'\''s`}, ""},
	{"WeirdDoubleQuoted", `probe weird "say`, []string{`say\"hi`}, ""},
	{"WeirdCommandSubstitution", "probe weird '$(", []string{"$(touch pwned)"}, ""},
	{"ColonWordBreak", "probe weird x:", []string{"y"}, ""},
	{"UnknownCommand", "probe nosuch ", nil, ""},
	{"NoCompLine", "probe widgets ", []string{"get", "list", "ls"}, "nocompline"},
}

func runBashCases(t *testing.T, bash bashInfo, bc string) {
	path := writeScript(t, "bash", "probe.bash")
	if out, err := exec.Command(bash.path, "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("bash -n: %v\n%s", err, out)
	}
	for _, c := range bashCases {
		t.Run(c.name, func(t *testing.T) {
			assertSet(t, "COMPREPLY for "+strconv.Quote(c.line), bashComplete(t, bash, path, c.line, bc, c.mode), c.want)
		})
	}
	// A broken config file or a bad config or environment value must not
	// stop completion: __complete ignores them, and its warning goes to
	// standard error, which the script discards.
	for name, env := range configCases(t) {
		t.Run(name, func(t *testing.T) {
			got := bashComplete(t, bash, path, "probe widgets ", bc, "", env...)
			assertSet(t, "COMPREPLY for \"probe widgets \" with "+name, got, []string{"get", "list", "ls"})
		})
	}
	assertNotPwned(t)
}

// TestShell_Bash32 drives macOS's /bin/bash 3.2 (no compopt).
func TestShell_Bash32(t *testing.T) {
	runBashCases(t, needBash(t, "bash32"), "")
}

// TestShell_Bash5 drives bash 4 or later.
func TestShell_Bash5(t *testing.T) {
	runBashCases(t, needBash(t, "bash5"), "")
}

// TestShell_Bash5_BashCompletion runs the bash cases with the
// bash-completion package loaded first.
func TestShell_Bash5_BashCompletion(t *testing.T) {
	bash := needBash(t, "bash5")
	bc := bashCompletionScript()
	if bc == "" {
		skipShell(t, "bash-completion", "the bash-completion package is not installed")
	}
	if bash.major == 4 && bash.minor < 2 {
		t.Skipf("bash-completion needs bash 4.2 or later, have %s", bash.version)
	}
	t.Logf("using %s", bc)
	runBashCases(t, bash, bc)
}

// TestShell_Syntax checks every script with its shell's parser: bash -n
// in each bash, zsh -n, fish --no-execute and the PowerShell parser.
func TestShell_Syntax(t *testing.T) {
	check := func(t *testing.T, shell string, cmd func(path string) *exec.Cmd) {
		path := writeScript(t, shell, "probe."+shell)
		if out, err := cmd(path).CombinedOutput(); err != nil {
			text, _ := os.ReadFile(path)
			t.Fatalf("%v: %v\n%s\n--- script ---\n%s", cmd(path).Args, err, out, text)
		}
	}
	for _, key := range []string{"bash32", "bash5"} {
		t.Run(key, func(t *testing.T) {
			bash := needBash(t, key)
			check(t, "bash", func(p string) *exec.Cmd { return exec.Command(bash.path, "-n", p) })
		})
	}
	t.Run("zsh", func(t *testing.T) {
		zsh := needShell(t, "zsh", "zsh")
		check(t, "zsh", func(p string) *exec.Cmd { return exec.Command(zsh, "-n", p) })
	})
	t.Run("fish", func(t *testing.T) {
		fish := needShell(t, "fish", "fish")
		check(t, "fish", func(p string) *exec.Cmd { return exec.Command(fish, "--no-execute", p) })
	})
	t.Run("pwsh", func(t *testing.T) {
		pwsh := needShell(t, "pwsh", "pwsh")
		check(t, "powershell", func(p string) *exec.Cmd {
			cmd := exec.Command(pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command",
				`$errors = $null; $null = [System.Management.Automation.Language.Parser]::ParseInput((Get-Content -Raw -LiteralPath $env:CLIX_SCRIPT), [ref]$null, [ref]$errors); if ($errors) { $errors | ForEach-Object { $_.ToString() }; exit 1 }`)
			cmd.Env = append(shellEnviron(t), "CLIX_SCRIPT="+p)
			return cmd
		})
	})
}

// needShell returns the path of the named program, or skips the test.
func needShell(t *testing.T, key, program string) string {
	t.Helper()
	unixShells(t, key)
	path, err := exec.LookPath(program)
	if err != nil {
		skipShell(t, key, program+" is not installed")
	}
	return path
}
