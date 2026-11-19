package autocomplete

import (
	"os/exec"
	"strings"
	"testing"
)

// candidateCase is a completion request and what the shell must and must
// not offer for it.
type candidateCase struct {
	name, line string
	want, not  []string
	exact      bool     // want is the complete set
	env        []string // extra environment
}

func checkCandidates(t *testing.T, c candidateCase, got []string) {
	t.Helper()
	t.Logf("%q: %q", c.line, got)
	if c.exact {
		assertSet(t, "candidates for "+c.line, got, c.want)
		return
	}
	for _, w := range c.want {
		found := false
		for _, g := range got {
			if g == w || strings.HasSuffix(w, "*") && strings.Contains(g, strings.TrimSuffix(w, "*")) {
				found = true
			}
		}
		if !found {
			t.Errorf("candidates for %q lack %q: %q", c.line, w, got)
		}
	}
	for _, w := range c.not {
		for _, g := range got {
			if g == w {
				t.Errorf("candidates for %q include %q: %q", c.line, w, got)
			}
		}
	}
}

// fishCases: "complete --do-complete" prints each candidate unescaped,
// as the whole token, with its description after a tab.
var fishCases = []candidateCase{
	{"TopLevel", "probe ", topLevel, nil, true, nil},
	{"NestedCommands", "probe widgets ", []string{"get", "list", "ls"}, nil, true, nil},
	{"Aliases", "probe w ", []string{"get", "list", "ls"}, nil, true, nil},
	{"FlagsInScope", "probe widgets get -", allFlagsOfGet, nil, true, nil},
	{"FlagsOutOfScope", "probe rm --", []string{"--force", "--path", "--help", "--verbose", "--org"}, nil, true, nil},
	{"FlagValueHook", "probe widgets get --output ", []string{"json", "yaml"}, nil, true, nil},
	{"FlagValueEquals", "probe widgets get --output=", []string{"--output=json", "--output=yaml"}, nil, true, nil},
	{"PositionalHook", "probe widgets get ", []string{"alpha", "beta", "gamma ray"}, nil, true, nil},
	{"WeirdNames", "probe weird ", weirdNames, nil, true, nil},
	{"FileFallback", "probe rm fi", []string{"file-one.txt"}, nil, false, nil},
	{"AfterDoubleDash", "probe rm -- -", []string{"-x.txt"}, []string{"--force"}, false, nil},
	{"HiddenOmitted", "probe in", nil, []string{"internal-debug"}, false, nil},
	{"NoFilesForCommands", "probe widgets ", nil, []string{"file-one.txt", "dir-a/"}, false, nil},
	{"FlagValueFile", "probe rm --path=fi", []string{"--path=file-one.txt"}, nil, false, nil},
	{"FileDollar", "probe rm evil/zz", []string{"evil/zz$(touch pwned).txt"}, nil, false, nil},
	{"FileBackquote", "probe rm evil/yy", []string{"evil/yy`touch pwned`.txt"}, nil, false, nil},
}

// TestShell_Fish asks fish for completions with complete --do-complete
// after loading the script (CI only: fish is not installed everywhere).
func TestShell_Fish(t *testing.T) {
	fish := needShell(t, "fish", "fish")
	_, _, work := shellEnv(t)
	path := writeScript(t, "fish", "probe.fish")
	for _, c := range withConfigCases(t, fishCases) {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(fish, "--no-config", "-c", `source $CLIX_SCRIPT; and complete --do-complete=$CLIX_LINE`)
			cmd.Env = shellEnviron(t, append(c.env, "CLIX_SCRIPT="+path, "CLIX_LINE="+c.line)...)
			cmd.Dir = work
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("fish: %v\n%s", err, out)
			}
			var got []string
			for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
				if v, _, _ := strings.Cut(l, "\t"); v != "" {
					got = append(got, v)
				}
			}
			checkCandidates(t, c, got)
		})
	}
	assertNotPwned(t)
}

// withConfigCases adds to cases one per broken configuration (see
// configCases), each completing "probe widgets ".
func withConfigCases(t *testing.T, cases []candidateCase) []candidateCase {
	t.Helper()
	out := append([]candidateCase(nil), cases...)
	for _, name := range []string{"MalformedFile", "BadConfigValue", "BadEnvValue"} {
		out = append(out, candidateCase{name: name, line: "probe widgets ", want: []string{"get", "list", "ls"}, exact: true, env: configCases(t)[name]})
	}
	return out
}

// pwshCases: TabExpansion2 returns CompletionText, which PowerShell
// inserts as is, so values that need it are single-quoted.
var pwshCases = []candidateCase{
	{"TopLevel", "probe ", topLevel, nil, true, nil},
	{"NestedCommands", "probe widgets ", []string{"get", "list", "ls"}, nil, true, nil},
	{"Aliases", "probe w ", []string{"get", "list", "ls"}, nil, true, nil},
	{"FlagsInScope", "probe widgets get -", allFlagsOfGet, nil, true, nil},
	{"FlagsOutOfScope", "probe rm --", []string{"--force", "--path", "--help", "--verbose", "--org"}, nil, true, nil},
	{"FlagValueHook", "probe widgets get --output ", []string{"json", "yaml"}, nil, true, nil},
	{"FlagValueEquals", "probe widgets get --output=", []string{"--output=json", "--output=yaml"}, nil, true, nil},
	{"PositionalHook", "probe widgets get ", []string{"alpha", "beta", "'gamma ray'"}, nil, true, nil},
	{"WeirdNames", "probe weird ", []string{`'say"hi'`, "'it''s'", "'$(touch pwned)'", "'two words'", "x:y", `back\slash`}, nil, true, nil},
	{"QuotedWord", "probe weird 'tw", []string{"'two words'"}, nil, true, nil},
	{"FileFallback", "probe rm fi", []string{"file-one.txt*"}, nil, false, nil},
	{"AfterDoubleDash", "probe rm -- -", nil, []string{"--force"}, false, nil},
	{"HiddenOmitted", "probe in", nil, []string{"internal-debug"}, false, nil},
	{"NoFilesForCommands", "probe nosuch ", nil, []string{"file-one.txt", "./file-one.txt"}, false, nil},
	{"FlagValueFile", "probe rm --path=fi", []string{"--path=*", "file-one.txt*"}, nil, false, nil},
}

// psDriver loads the script and prints the completion text of every
// match for $env:CLIX_LINE with the cursor at its end.
const psDriver = `. $env:CLIX_SCRIPT
$line = $env:CLIX_LINE
$result = TabExpansion2 -inputScript $line -cursorColumn $line.Length
foreach ($match in $result.CompletionMatches) { $match.CompletionText }`

// TestShell_PowerShell asks PowerShell for completions with TabExpansion2
// after loading the script (CI only: pwsh is not installed everywhere).
func TestShell_PowerShell(t *testing.T) {
	pwsh := needShell(t, "pwsh", "pwsh")
	_, _, work := shellEnv(t)
	path := writeScript(t, "powershell", "probe.ps1")
	for _, c := range withConfigCases(t, pwshCases) {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", psDriver)
			cmd.Env = shellEnviron(t, append(c.env, "CLIX_SCRIPT="+path, "CLIX_LINE="+c.line)...)
			cmd.Dir = work
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("pwsh: %v\n%s", err, out)
			}
			var got []string
			for _, l := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
				if l != "" {
					got = append(got, l)
				}
			}
			checkCandidates(t, c, got)
		})
	}
	assertNotPwned(t)
}

// psLiteralDriver completes $env:CLIX_LINE and, for each match, prints the
// kind of argument PowerShell parses its completion text as, and its value:
// a file name with $(...) must come back as a constant string, never as an
// expandable one that would run the subexpression.
const psLiteralDriver = `. $env:CLIX_SCRIPT
$line = $env:CLIX_LINE
$result = TabExpansion2 -inputScript $line -cursorColumn $line.Length
foreach ($match in $result.CompletionMatches) {
    $ast = [System.Management.Automation.Language.Parser]::ParseInput('Write-Output ' + $match.CompletionText, [ref]$null, [ref]$null)
    $arg = $ast.EndBlock.Statements[0].PipelineElements[0].CommandElements[1]
    $arg.GetType().Name + "` + "\t" + `" + $arg.Extent.Text + "` + "\t" + `" + $arg.Value
}`

// TestShell_PowerShell_FileNamesAreLiteral checks that PowerShell's own
// file completion, which the script falls back to, quotes file names with
// $(...) and backquotes (CI only).
func TestShell_PowerShell_FileNamesAreLiteral(t *testing.T) {
	pwsh := needShell(t, "pwsh", "pwsh")
	_, _, work := shellEnv(t)
	path := writeScript(t, "powershell", "probe.ps1")
	for _, line := range []string{"probe rm evil/zz", "probe rm evil/yy"} {
		cmd := exec.Command(pwsh, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", psLiteralDriver)
		cmd.Env = shellEnviron(t, "CLIX_SCRIPT="+path, "CLIX_LINE="+line)
		cmd.Dir = work
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("pwsh: %v\n%s", err, out)
		}
		t.Logf("%q:\n%s", line, out)
		lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(out), "\r\n", "\n")), "\n")
		if len(lines) == 0 || lines[0] == "" {
			t.Errorf("%q: no file completion", line)
		}
		for _, l := range lines {
			kind, rest, _ := strings.Cut(l, "\t")
			_, value, _ := strings.Cut(rest, "\t")
			if kind != "StringConstantExpressionAst" || !strings.HasSuffix(value, ".txt") || !strings.Contains(value, "touch pwned") {
				t.Errorf("%q: completion %q is not the literal file name", line, l)
			}
		}
	}
	assertNotPwned(t)
}
