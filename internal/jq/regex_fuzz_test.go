package jq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestRegexDifferentialFuzz generates random patterns in the syntax
// compileRegex accepts and random texts, and checks that test() agrees
// with jq's on every pair. CLIX_JQ_FUZZ_N scales the pattern count
// (default 1500 filters means 600 patterns).
func TestRegexDifferentialFuzz(t *testing.T) {
	bin := requireJQ(t)
	n := envInt(t, "CLIX_JQ_FUZZ_N", 1500) * 2 / 5
	seed := int64(envInt(t, "CLIX_JQ_FUZZ_SEED", 1))
	if testing.Short() && n > 100 {
		n = 100
	}
	r := rand.New(rand.NewSource(seed))
	g := &reGen{r: r}

	const perBatch = 50
	checked, rejected := 0, 0
	for done := 0; done < n; {
		texts := make([]string, 30)
		for i := range texts {
			texts[i] = g.text()
		}
		input, err := json.Marshal(texts)
		if err != nil {
			t.Fatal(err)
		}
		var patterns []string
		var want []string
		for len(patterns) < perBatch && done < n {
			p := g.pattern()
			done++
			re, err := compileRegex(p)
			if err != nil {
				rejected++
				continue
			}
			var b strings.Builder
			b.WriteByte('[')
			for i, s := range texts {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteString(strconv.FormatBool(re.MatchString(s)))
			}
			b.WriteByte(']')
			patterns = append(patterns, p)
			want = append(want, b.String())
		}
		if len(patterns) == 0 {
			continue
		}
		var program strings.Builder
		for i, p := range patterns {
			if i > 0 {
				program.WriteString(", ")
			}
			fmt.Fprintf(&program, "(try [.[] | test(%s)] catch \"ERR\")", jqString(p))
		}
		cmd := exec.Command(bin.path, "-c", program.String())
		cmd.Stdin = bytes.NewReader(input)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("jq failed on the batch: %v\nprogram: %s", err, program.String())
		}
		lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
		if len(lines) != len(patterns) {
			t.Fatalf("jq printed %d lines for %d patterns", len(lines), len(patterns))
		}
		for i, line := range lines {
			checked++
			if line == `"ERR"` {
				// Oniguruma gives up on patterns that backtrack too much
				// ("retry-limit-in-match over"); RE2 does not backtrack.
				msg := jqRegexError(t, bin, patterns[i], input)
				if strings.Contains(msg, "limit") {
					t.Logf("pattern %q: jq hit a backtracking limit: %s", patterns[i], msg)
					continue
				}
				t.Errorf("pattern %q: clix accepts it, jq's Oniguruma rejects it: %s", patterns[i], msg)
				continue
			}
			if line != want[i] {
				t.Errorf("pattern %q (Go %v) on %q:\nclix %s\njq   %s", patterns[i], mustCompile(patterns[i]), texts, want[i], line)
			}
		}
	}
	t.Logf("checked %d patterns on 30 texts each; %d generated patterns were rejected", checked, rejected)
}

// jqRegexError returns jq's error for test(pattern) on input.
func jqRegexError(t *testing.T, bin jqBinary, pattern string, input []byte) string {
	t.Helper()
	cmd := exec.Command(bin.path, "-c", fmt.Sprintf("[.[] | test(%s)]", jqString(pattern)))
	cmd.Stdin = bytes.NewReader(input)
	out, _ := cmd.CombinedOutput()
	return strings.TrimSpace(string(out))
}

func mustCompile(p string) string {
	re, err := compileRegex(p)
	if err != nil {
		return err.Error()
	}
	return re.String()
}

// jqString writes s as a jq string literal.
func jqString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

type reGen struct{ r *rand.Rand }

var (
	reAlphabet = []string{"a", "b", "c", "A", "é", "0", "1", " ", "\n", ".", "-", "_", "😀", "ß", "Σ", "\r", "\r\n", "\u2028", "\t", "SS", "σ", "٣"}
	reAtoms    = []string{"a", "b", "c", "A", "é", "0", "1", " ", `\n`, "\n", `\r`, "\u2028", "σ", "٣", `\p{Z}`, `\p{Zl}`, `\p{Cc}`, `\p{N}`, `\P{Nd}`, `\p{Latin}`, `\p{Common}`, `\p{Any}`, `\.`, `\-`, "_", "😀", "ß", "Σ", ".", `\t`, `\x41`, `\x{e9}`, `\p{L}`, `\P{L}`, `\p{Lu}`, `\p{Nd}`, `\p{Greek}`, `\p{So}`, `\A`, `\z`, `\\`, `\*`, `\(`, `\[`, `\{`, `\$`, `\^`, `\|`, `\/`}
	reClass    = []string{"a", "b", "c", "a-c", "0-9", "A-Z", "é", `\n`, `\-`, `\]`, `\\`, `\p{L}`, `\P{Nd}`, " ", ".", "_", "😀", "Σ", `\x41`, "*", "+", "(", "$", "^b"}
)

func (g *reGen) text() string {
	var b strings.Builder
	for i := g.r.Intn(6); i > 0; i-- {
		b.WriteString(reAlphabet[g.r.Intn(len(reAlphabet))])
	}
	if g.r.Intn(4) == 0 {
		b.WriteString("\n")
	}
	return b.String()
}

func (g *reGen) pattern() string { return g.alt(3) }

func (g *reGen) alt(d int) string {
	parts := []string{g.concat(d)}
	for g.r.Intn(4) == 0 {
		parts = append(parts, g.concat(d))
	}
	return strings.Join(parts, "|")
}

func (g *reGen) concat(d int) string {
	var b strings.Builder
	if g.r.Intn(4) == 0 {
		b.WriteString("^")
	}
	for i := g.r.Intn(4); i > 0; i-- {
		b.WriteString(g.repeat(d))
	}
	if g.r.Intn(4) == 0 {
		b.WriteString("$")
	}
	return b.String()
}

func (g *reGen) repeat(d int) string {
	a := g.atom(d)
	switch g.r.Intn(8) {
	case 0:
		a += "*"
	case 1:
		a += "+"
	case 2:
		a += "?"
	case 3:
		a += []string{"{2}", "{1,}", "{0,2}", "{1,3}"}[g.r.Intn(4)]
	default:
		return a
	}
	if g.r.Intn(4) == 0 {
		a += "?"
	}
	return a
}

func (g *reGen) atom(d int) string {
	switch k := g.r.Intn(10); {
	case k < 6 || d <= 0:
		return reAtoms[g.r.Intn(len(reAtoms))]
	case k < 8:
		var b strings.Builder
		b.WriteByte('[')
		if g.r.Intn(3) == 0 {
			b.WriteByte('^')
		}
		for i := g.r.Intn(3) + 1; i > 0; i-- {
			b.WriteString(reClass[g.r.Intn(len(reClass))])
		}
		b.WriteByte(']')
		return b.String()
	}
	open := []string{"(", "(?:", "(?<n>"}[g.r.Intn(3)]
	return open + g.alt(d-1) + ")"
}
