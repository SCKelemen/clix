package autocomplete

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

var topLevel = []string{"autocomplete", "cp", "rm", "weird", "widgets", "w"}

func TestComplete_TopLevel(t *testing.T) {
	got, directive := completeWords(t, "")
	assertSet(t, "probe <TAB>", got, topLevel)
	if directive != directiveNoFiles {
		t.Errorf("directive = %s, want %s", directive, directiveNoFiles)
	}
	got, _ = completeWords(t, "w")
	assertSet(t, "probe w<TAB>", got, []string{"weird", "widgets", "w"})
}

func TestComplete_NestedGroup(t *testing.T) {
	got, _ := completeWords(t, "", "widgets")
	assertSet(t, "probe widgets <TAB>", got, []string{"get", "list", "ls"})
	got, _ = completeWords(t, "", "widgets", "get")
	for _, w := range got {
		if w == "rm" || w == "get" {
			t.Errorf("probe widgets get <TAB> offers command %q: %q", w, got)
		}
	}
}

func TestComplete_Aliases(t *testing.T) {
	got, _ := completeWords(t, "", "w")
	assertSet(t, "probe w <TAB> (w is an alias of widgets)", got, []string{"get", "list", "ls"})
	got, _ = completeWords(t, "l", "widgets")
	assertSet(t, "probe widgets l<TAB>", got, []string{"list", "ls"})
	got, _ = completeWords(t, "", "w", "ls")
	if len(got) != 0 {
		t.Errorf("probe w ls <TAB> = %q, want nothing", got)
	}
}

func TestComplete_HiddenCommandsExcluded(t *testing.T) {
	for _, cur := range []string{"", "i", "_"} {
		got, _ := completeWords(t, cur)
		for _, w := range got {
			if w == "internal-debug" || w == completeName {
				t.Errorf("probe %s<TAB> offers hidden command %q", cur, w)
			}
		}
	}
	// A hidden command the user typed still routes, like in the parser.
	got, _ := completeWords(t, "--s", "internal-debug")
	assertSet(t, "probe internal-debug --s<TAB>", got, []string{"--secret-flag"})
}

func TestComplete_FlagsInScope(t *testing.T) {
	// The leaf's flags, then its ancestors' and the root's: get's -o
	// (--output) hides the root's -o (--org), as in the parser.
	got, directive := completeWords(t, "-", "widgets", "get")
	want := []string{"--watch", "-w", "--output", "-o", "--name", "--help", "-h", "--all", "-a", "--verbose", "-v", "--org"}
	assertSet(t, "probe widgets get -<TAB>", got, want)
	if directive != directiveNoFiles {
		t.Errorf("directive = %s, want %s", directive, directiveNoFiles)
	}
	got, _ = completeWords(t, "--o", "widgets", "get")
	assertSet(t, "probe widgets get --o<TAB>", got, []string{"--output", "--org"})

	app, out := testApp(t)
	lines, _ := runComplete(t, app, out, "--cur=-o", "--", "probe", "widgets", "get")
	if !reflect.DeepEqual(lines, []string{"-o\tOutput format"}) {
		t.Errorf("probe widgets get -o<TAB> = %q, want get's -o with its usage", lines)
	}
	lines, _ = runComplete(t, app, out, "--cur=-o", "--", "probe")
	if !reflect.DeepEqual(lines, []string{"-o\tOrganization to use"}) {
		t.Errorf("probe -o<TAB> = %q, want the root's -o", lines)
	}
}

func TestComplete_FlagsOutOfScope(t *testing.T) {
	got, _ := completeWords(t, "--", "rm")
	assertSet(t, "probe rm --<TAB>", got, []string{"--force", "--path", "--help", "--verbose", "--org"})
	got, _ = completeWords(t, "--", "widgets")
	assertSet(t, "probe widgets --<TAB>", got, []string{"--all", "--help", "--verbose", "--org"})
	got, _ = completeWords(t, "-")
	assertSet(t, "probe -<TAB>", got, []string{"--help", "-h", "--verbose", "-v", "--org", "-o"})
}

func TestComplete_GivenFlagsLeftOut(t *testing.T) {
	got, _ := completeWords(t, "--", "widgets", "get", "--watch")
	assertSet(t, "after --watch", got, []string{"--output", "--name", "--help", "--all", "--verbose", "--org"})
	// A cluster of switches, a short flag with its value attached, and a
	// root flag before the command.
	got, _ = completeWords(t, "--", "-v", "widgets", "get", "-wa", "-ojson")
	assertSet(t, "after -v -wa -ojson", got, []string{"--name", "--help", "--org"})
	// A switch written before the command name belongs to the command.
	got, _ = completeWords(t, "--", "--force", "rm")
	assertSet(t, "probe --force rm --<TAB>", got, []string{"--path", "--help", "--verbose", "--org"})
	// Repeatable flags stay.
	got, _ = completeWords(t, "--", "cp", "--tag", "red")
	assertSet(t, "probe cp --tag red --<TAB>", got, []string{"--tag", "--files", "--help", "--verbose", "--org"})
}

func TestComplete_FlagValueSeparate(t *testing.T) {
	app, out := testApp(t)
	lines, directive := runComplete(t, app, out, "--cur=", "--", "probe", "widgets", "get", "--output")
	want := []string{"json\tJSON document", "yaml\tYAML document"}
	if !reflect.DeepEqual(lines, want) || directive != directiveNoFiles {
		t.Errorf("probe widgets get --output <TAB> = %q %s, want %q %s", lines, directive, want, directiveNoFiles)
	}
	got, _ := completeWords(t, "y", "widgets", "get", "-o")
	assertSet(t, "probe widgets get -o y<TAB>", got, []string{"yaml"})
	got, _ = completeWords(t, "", "--org")
	assertSet(t, "probe --org <TAB>", got, []string{"acme", "initech"})
	// The value is consumed: "acme" is not a stray word.
	got, _ = completeWords(t, "", "--org", "acme")
	assertSet(t, "probe --org acme <TAB>", got, topLevel)
	got, _ = completeWords(t, "", "cp", "--tag")
	assertSet(t, "probe cp --tag <TAB>", got, []string{"red", "blue"})
}

func TestComplete_FlagValueEquals(t *testing.T) {
	cases := []struct {
		cur   string
		words []string
		want  []string
	}{
		{"--output=", []string{"widgets", "get"}, []string{"--output=json", "--output=yaml"}},
		{"--output=j", []string{"widgets", "get"}, []string{"--output=json"}},
		{"-o=y", []string{"widgets", "get"}, []string{"-o=yaml"}},
		{"-oj", []string{"widgets", "get"}, []string{"-ojson"}},
		{"-wo", []string{"widgets", "get"}, nil},
		{"-woy", []string{"widgets", "get"}, []string{"-woyaml"}},
		{"--watch=", []string{"widgets", "get"}, []string{"--watch=true", "--watch=false"}},
		{"--org=i", nil, []string{"--org=initech"}},
		{"--nope=", []string{"widgets", "get"}, nil},
	}
	for _, c := range cases {
		got, directive := completeWords(t, c.cur, c.words...)
		assertSet(t, "probe "+strings.Join(c.words, " ")+" "+c.cur+"<TAB>", got, c.want)
		if directive != directiveNoFiles {
			t.Errorf("%s: directive = %s", c.cur, directive)
		}
	}
	// A value without a hook falls back to file names.
	got, directive := completeWords(t, "--path=", "rm")
	if len(got) != 0 || directive != directiveFiles {
		t.Errorf("probe rm --path=<TAB> = %q %s, want file names", got, directive)
	}
}

func TestComplete_PositionalHook(t *testing.T) {
	app, out := testApp(t)
	lines, directive := runComplete(t, app, out, "--cur=", "--", "probe", "-v", "widgets", "get")
	want := []string{"alpha\tThe first widget", "beta\tThe second widget", "gamma ray\tA widget with a space"}
	if !reflect.DeepEqual(lines, want) || directive != directiveNoFiles {
		t.Errorf("probe widgets get <TAB> = %q %s, want %q", lines, directive, want)
	}
	recorded.Lock()
	req := recorded.req
	recorded.Unlock()
	if req.Command == nil || req.Command.Name != "get" || req.Flag == nil || req.Flag.Name != "name" || req.App != app || req.Context == nil {
		t.Errorf("request = %+v, want command get, flag name and the app", req)
	}

	got, _ := completeWords(t, "g", "widgets", "get", "--watch")
	assertSet(t, "probe widgets get --watch g<TAB>", got, []string{"gamma ray"})
	// The only positional is taken: nothing more to complete.
	got, directive = completeWords(t, "", "widgets", "get", "alpha")
	if len(got) != 0 || directive != directiveNoFiles {
		t.Errorf("probe widgets get alpha <TAB> = %q %s, want nothing", got, directive)
	}
	// Given by name, the positional is not offered by position.
	got, _ = completeWords(t, "", "widgets", "get", "--name", "alpha")
	if len(got) != 0 {
		t.Errorf("probe widgets get --name alpha <TAB> = %q, want nothing", got)
	}
}

func TestComplete_PositionalArgs(t *testing.T) {
	app, out := testApp(t)
	runComplete(t, app, out, "--cur=b", "--", "probe", "widgets", "get")
	recorded.Lock()
	req := recorded.req
	recorded.Unlock()
	if req.ToComplete != "b" || len(req.Args) != 0 {
		t.Errorf("ToComplete = %q, Args = %q; want b and none", req.ToComplete, req.Args)
	}
}

func TestComplete_DoubleDash(t *testing.T) {
	// After "--" everything is an argument: no flags, even for "-".
	got, directive := completeWords(t, "-", "rm", "--")
	if len(got) != 0 || directive != directiveFiles {
		t.Errorf("probe rm -- -<TAB> = %q %s, want file names only", got, directive)
	}
	got, _ = completeWords(t, "-", "widgets", "get", "--")
	if len(got) != 0 {
		t.Errorf("probe widgets get -- -<TAB> = %q, want no flags (no widget starts with -)", got)
	}
	got, _ = completeWords(t, "", "widgets", "get", "--")
	assertSet(t, "probe widgets get -- <TAB>", got, []string{"alpha", "beta", "gamma ray"})
	got, _ = completeWords(t, "", "--", "widgets")
	if len(got) != 0 {
		t.Errorf("probe -- widgets <TAB> = %q: after -- words are not commands", got)
	}
}

func TestComplete_FileDirective(t *testing.T) {
	cases := []struct {
		cur       string
		words     []string
		directive string
	}{
		{"", []string{"rm"}, directiveFiles},
		{"fi", []string{"rm"}, directiveFiles},
		{"", []string{"rm", "--path"}, directiveFiles},
		{"", []string{"cp", "a", "b"}, directiveFiles}, // variadic: always more
		{"", []string{"rm", "a"}, directiveNoFiles},    // the one argument is taken
		{"", []string{"widgets"}, directiveNoFiles},
		{"", nil, directiveNoFiles},
		{"--", []string{"rm"}, directiveNoFiles},
		{"", []string{"widgets", "get", "--output"}, directiveNoFiles},
	}
	for _, c := range cases {
		_, directive := completeWords(t, c.cur, c.words...)
		if directive != c.directive {
			t.Errorf("probe %s %s<TAB>: directive = %s, want %s", strings.Join(c.words, " "), c.cur, directive, c.directive)
		}
	}
}

func TestComplete_RootNameRepeated(t *testing.T) {
	got, _ := completeWords(t, "", "probe")
	assertSet(t, "probe probe <TAB>", got, topLevel)
}

func TestComplete_UnknownCommand(t *testing.T) {
	got, directive := completeWords(t, "", "nosuch")
	if len(got) != 0 || directive != directiveNoFiles {
		t.Errorf("probe nosuch <TAB> = %q %s, want nothing", got, directive)
	}
	got, _ = completeWords(t, "", "widgets", "nosuch")
	if len(got) != 0 {
		t.Errorf("probe widgets nosuch <TAB> = %q, want nothing", got)
	}
}

func TestComplete_CaseMatching(t *testing.T) {
	got, _ := completeWords(t, "", "WIDGETS")
	assertSet(t, "probe WIDGETS <TAB>", got, []string{"get", "list", "ls"})
	got, _ = completeWords(t, "WID")
	assertSet(t, "probe WID<TAB>", got, []string{"widgets"})

	app, out := testApp(t)
	app.CaseSensitive = true
	lines, _ := runComplete(t, app, out, "--cur=", "--", "probe", "WIDGETS")
	if len(lines) != 0 {
		t.Errorf("case-sensitive probe WIDGETS <TAB> = %q, want nothing", lines)
	}
	lines, _ = runComplete(t, app, out, "--cur=WID", "--", "probe")
	if len(lines) != 0 {
		t.Errorf("case-sensitive probe WID<TAB> = %q, want nothing", lines)
	}
	lines, _ = runComplete(t, app, out, "--cur=", "--", "probe", "widgets")
	assertSet(t, "case-sensitive probe widgets <TAB>", values(lines), []string{"get", "list", "ls"})
}

func TestComplete_WeirdNames(t *testing.T) {
	got, _ := completeWords(t, "", "weird")
	assertSet(t, "probe weird <TAB>", got, weirdNames)
	app, out := testApp(t)
	lines, _ := runComplete(t, app, out, "--cur=", "--", "probe", "weird")
	for _, l := range lines {
		if !strings.HasSuffix(l, "\t"+`Says "hi" [loudly]: it's \ fine`) {
			t.Errorf("candidate %q lost its description", l)
		}
	}
}

func TestComplete_IgnoresEnvAndConfig(t *testing.T) {
	// Values for __complete's own flags from the environment or the config
	// file must not change what is completed.
	app, out := testApp(t)
	t.Setenv("PROBE_LINE", "probe rm ")
	t.Setenv("PROBE_WORD", "rm")
	t.Setenv("PROBE_SHELL", "zsh")
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "probe")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "config.yaml"), []byte("cur: wid\nline: probe rm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, _ := runComplete(t, app, out, "--cur=", "--", "probe", "widgets")
	assertSet(t, "probe widgets <TAB>", values(lines), []string{"get", "list", "ls"})
}

func TestComplete_BrokenConfigFile(t *testing.T) {
	app, out := testApp(t)
	cfg := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "probe")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "config.yaml"), []byte("key: [unclosed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, _ := runComplete(t, app, out, "--cur=", "--", "probe", "widgets")
	assertSet(t, "probe widgets <TAB> with a broken config file", values(lines), []string{"get", "list", "ls"})
}

func TestComplete_RequiredRootFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("req", clix.WithAppOut(out), clix.WithAppErr(io.Discard), clix.WithAppIn(strings.NewReader("")))
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Required: true}})
	app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandRun(noop)))
	app.AddExtension(Extension{})
	lines, _ := runComplete(t, app, out, "--cur=", "--", "req")
	assertSet(t, "req <TAB> with a required root flag", values(lines), []string{"autocomplete", "deploy"})
}

func TestComplete_NewCompleteCommandOnly(t *testing.T) {
	// The autocomplete command adds __complete when built by hand.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	out := &bytes.Buffer{}
	app := clix.NewApp("hand", clix.WithAppOut(out), clix.WithAppErr(io.Discard))
	app.Root.AddCommand(clix.NewCommand("go", clix.WithCommandRun(noop)))
	app.Root.AddCommand(NewAutocompleteCommand(app))
	lines, _ := runComplete(t, app, out, "--cur=", "--", "hand")
	assertSet(t, "hand <TAB>", values(lines), []string{"autocomplete", "go"})
	if err := app.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestComplete_ProbeAppIsValid(t *testing.T) {
	app, _ := testApp(t)
	if err := app.Validate(); err != nil {
		t.Fatalf("the test app has definition problems: %v", err)
	}
}

func TestComplete_AutocompleteShellNames(t *testing.T) {
	got, _ := completeWords(t, "", "autocomplete")
	assertSet(t, "probe autocomplete <TAB>", got, []string{"bash", "zsh", "fish", "powershell"})
}

// --- output formats ---

func TestComplete_FormatBash(t *testing.T) {
	cases := []struct {
		line, cur string // the command line up to the cursor and readline's word ($2)
		want      []string
	}{
		{"probe ", "", topLevel},
		{"probe weird ", "", []string{`say\"hi`, `it\'s`, `\$\(touch\ pwned\)`, `two\ words`, `x:y`, `back\\slash`}},
		// Inside quotes readline closes the quote itself.
		{`probe weird "say`, "say", []string{`say\"hi`}},
		{`probe weird 'it`, "it", []string{`it'\''s`}},
		{`probe weird "$(t`, "$(t", []string{`\$(touch pwned)`}},
		{`probe weird 'two`, "two", []string{`two words`}},
		{`probe weird two\ w`, `two\ w`, []string{`two\ words`}},
		// readline breaks words at "=" and ":"; candidates replace only
		// its word.
		{"probe widgets get --output=", "", []string{"json", "yaml"}},
		{"probe widgets get --output=y", "y", []string{"yaml"}},
		{"probe weird x:", "", []string{"y"}},
		{`probe widgets get --name="gam`, "gam", []string{"gamma ray"}},
		// Without word breaks at "=" readline's word is the whole word.
		{"probe widgets get --output=y", "--output=y", []string{"--output=yaml"}},
		{"probe", "probe", nil},
	}
	for _, c := range cases {
		app, out := testApp(t)
		lines, _ := runComplete(t, app, out, "--shell", "bash", "--cur="+c.cur, "--line="+c.line)
		assertSet(t, "bash "+c.line+"<TAB>", lines, c.want)
	}
}

func TestComplete_FormatZsh(t *testing.T) {
	app, out := testApp(t)
	lines, _ := runComplete(t, app, out, "--shell", "zsh", "--cur=x", "--", "probe", "weird")
	want := []string{`x\:y:Says "hi" [loudly]: it's \\ fine`}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("zsh probe weird x<TAB> = %q, want %q", lines, want)
	}
	lines, _ = runComplete(t, app, out, "--shell", "zsh", "--cur=b", "--", "probe", "weird")
	want = []string{`back\\slash:Says "hi" [loudly]: it's \\ fine`}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("zsh probe weird b<TAB> = %q, want %q", lines, want)
	}
	lines, _ = runComplete(t, app, out, "--shell", "zsh", "--cur=", "--", "probe", "widgets", "get", "--output")
	want = []string{"json:JSON document", "yaml:YAML document"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("zsh --output <TAB> = %q, want %q", lines, want)
	}
}

func TestComplete_FormatFish(t *testing.T) {
	app, out := testApp(t)
	lines, _ := runComplete(t, app, out, "--shell", "fish", "--cur=", "--", "probe", "widgets")
	want := []string{"get\tShow one widget", "list\tList widgets", "ls\tList widgets"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("fish probe widgets <TAB> = %q, want %q", lines, want)
	}
	// fish passes the raw token; its quotes are removed.
	lines, _ = runComplete(t, app, out, "--shell", "fish", `--cur='it\'`, "--", "probe", "weird")
	assertSet(t, "fish probe weird 'it\\'<TAB>", values(lines), []string{"it's"})
}

func TestComplete_FormatPowerShell(t *testing.T) {
	app, out := testApp(t)
	lines, _ := runComplete(t, app, out, "--shell", "pwsh", "--cur=", "--word=probe", "--word=weird")
	want := []string{
		"'say\"hi'\tsay\"hi\t" + `Says "hi" [loudly]: it's \ fine`,
		"'it''s'\tit's\t" + `Says "hi" [loudly]: it's \ fine`,
		"'$(touch pwned)'\t$(touch pwned)\t" + `Says "hi" [loudly]: it's \ fine`,
		"'two words'\ttwo words\t" + `Says "hi" [loudly]: it's \ fine`,
		"x:y\tx:y\t" + `Says "hi" [loudly]: it's \ fine`,
		`back\slash` + "\t" + `back\slash` + "\t" + `Says "hi" [loudly]: it's \ fine`,
	}
	assertSet(t, "powershell probe weird <TAB>", lines, want)
	// An empty word and a word that is "--" survive as --word= arguments.
	lines, directive := runComplete(t, app, out, "--shell", "powershell", "--cur=-", "--word=probe", "--word=rm", "--word=--")
	if len(lines) != 0 || directive != directiveFiles {
		t.Errorf("powershell probe rm -- -<TAB> = %q %s, want file names", lines, directive)
	}
	lines, _ = runComplete(t, app, out, "--shell", "powershell", "--cur='two", "--word=probe", "--word=weird")
	assertSet(t, "powershell probe weird 'two<TAB>", lines, []string{"'two words'\ttwo words\t" + `Says "hi" [loudly]: it's \ fine`})
	// Without a description the tooltip is the value (it must not be empty).
	lines, _ = runComplete(t, app, out, "--shell", "powershell", "--cur=", "--word=probe", "--word=autocomplete")
	assertSet(t, "powershell probe autocomplete <TAB>", lines, []string{"bash\tbash\tbash", "zsh\tzsh\tzsh", "fish\tfish\tfish", "powershell\tpowershell\tpowershell"})
}

func TestComplete_UnsupportedShell(t *testing.T) {
	app, _ := testApp(t)
	err := app.Run(context.Background(), []string{completeName, "--shell", "tcsh", "--cur=", "--", "probe"})
	if err == nil || !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("__complete --shell tcsh: err = %v, want unsupported shell", err)
	}
}

// --- shell word helpers ---

func TestSplitPOSIX(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"", []string{""}},
		{"probe ", []string{"probe", ""}},
		{"probe wid", []string{"probe", "wid"}},
		{`probe "a b" 'c d' e\ f`, []string{"probe", "a b", "c d", "e f"}},
		{`probe "it's \"x\" \$y \z"`, []string{"probe", `it's "x" $y \z`}},
		{`probe 'unterminated`, []string{"probe", "unterminated"}},
		{`probe "unterminated \"`, []string{"probe", `unterminated "`}},
		{`probe a""b ''`, []string{"probe", "ab", ""}},
		{"probe a\\\nb", []string{"probe", "ab"}},
	}
	for _, c := range cases {
		var got []string
		for _, w := range splitPOSIX(c.line) {
			got = append(got, w.value)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitPOSIX(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestUnquotePOSIXPrefix(t *testing.T) {
	cases := []struct {
		raw   string
		value string
		state byte
	}{
		{"--output=", "--output=", unquoted},
		{`"`, "", doubleQuoted},
		{`--x="a\"`, `--x=a"`, doubleQuoted},
		{`'it`, "it", singleQuoted},
		{`a\ b`, "a b", unquoted},
		{`"a"'b'`, "ab", unquoted},
	}
	for _, c := range cases {
		value, state := unquotePOSIXPrefix(c.raw)
		if value != c.value || state != c.state {
			t.Errorf("unquotePOSIXPrefix(%q) = %q, %q; want %q, %q", c.raw, value, state, c.value, c.state)
		}
	}
}

func TestQuoting(t *testing.T) {
	if got := shellSingleQuote(`it's`); got != `'it'\''s'` {
		t.Errorf("shellSingleQuote = %s", got)
	}
	if got := fishQuote(`it's \ ok`); got != `'it\'s \\ ok'` {
		t.Errorf("fishQuote = %s", got)
	}
	if got := psQuote("it's"); got != "'it''s'" {
		t.Errorf("psQuote = %s", got)
	}
	if got := quotePowerShell("--output=json"); got != "--output=json" {
		t.Errorf("quotePowerShell plain = %s", got)
	}
	if got := quotePowerShell("a,b"); got != "'a,b'" {
		t.Errorf("quotePowerShell comma = %s", got)
	}
	if got := zshDescribe("rm", "a [b]: c"); got != "rm:a [b]: c" {
		t.Errorf("zshDescribe = %s", got)
	}
	if got := unquoteFish(`"a\$b" 'c\'d' e\ f`); got != `a$b c'd e f` {
		t.Errorf("unquoteFish = %q", got)
	}
	if got := unquotePowerShell("'it''s' \"a`\"b\" c`d"); got != `it's a"b cd` {
		t.Errorf("unquotePowerShell = %q", got)
	}
	if got := identifier("my-app.v2"); got != "my_app_v2" {
		t.Errorf("identifier = %q", got)
	}
}
