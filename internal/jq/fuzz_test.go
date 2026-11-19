package jq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// TestDifferentialFuzz generates random inputs (shaped like FormatData's
// JSON) and random filters from the supported subset, and checks that
// Query.Write prints exactly what jq -r prints and fails exactly when jq
// fails. CLIX_JQ_FUZZ_N sets the number of cases (default 1500) and
// CLIX_JQ_FUZZ_SEED the seed (default 1).
func TestDifferentialFuzz(t *testing.T) {
	bin := requireJQ(t)
	n := envInt(t, "CLIX_JQ_FUZZ_N", 1500)
	seed := int64(envInt(t, "CLIX_JQ_FUZZ_SEED", 1))
	if testing.Short() && n > 300 {
		n = 300
	}
	t.Logf("%d cases, seed %d, %s (decNumber: %v)", n, seed, bin.version, bin.decNum)

	const perBatch = 40
	type batch struct {
		input   string
		filters []string
	}
	g := &fuzzGen{r: rand.New(rand.NewSource(seed)), decNum: bin.decNum}
	var batches []batch
	for made := 0; made < n; {
		b := batch{input: g.input()}
		for i := 0; i < perBatch && made < n; i++ {
			b.filters = append(b.filters, g.filter())
			made++
		}
		batches = append(batches, b)
	}

	var (
		mu       sync.Mutex
		failures []string
		compared int
		skipped  int
	)
	fail := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		failures = append(failures, msg)
	}
	work := make(chan batch)
	var wg sync.WaitGroup
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for b := range work {
				c, s := fuzzBatch(t, bin, b.input, b.filters, fail)
				mu.Lock()
				compared += c
				skipped += s
				mu.Unlock()
			}
		}()
	}
	for _, b := range batches {
		mu.Lock()
		stop := len(failures) >= 10
		mu.Unlock()
		if stop {
			break
		}
		work <- b
	}
	close(work)
	wg.Wait()
	t.Logf("compared %d cases, skipped %d with very large output", compared, skipped)
	for _, f := range failures {
		t.Error(f)
	}
}

func envInt(t *testing.T, name string, def int) int {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("%s=%q: %v", name, v, err)
	}
	return n
}

const (
	caseMark = "\x01CASE"
	errMark  = "\x01ERR"
)

// errTooLarge stops a case whose output is too large to compare cheaply.
var errTooLarge = errors.New("output too large")

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, errTooLarge
	}
	return b.Buffer.Write(p)
}

// fuzzBatch runs the filters on input in one jq process, each wrapped in
// try/catch and preceded by a marker, and compares each case with
// Query.Write. A case that seems to differ is run again on its own, so
// batching cannot cause a false report.
func fuzzBatch(t *testing.T, bin jqBinary, input string, filters []string, fail func(string)) (compared, skipped int) {
	type result struct {
		out         string
		skip        bool
		unsupported bool // failed with "not supported in clix --jq"
	}
	ours := make([]result, len(filters))
	var program strings.Builder
	var kept []int
	for i, f := range filters {
		q, err := Parse(f)
		if err != nil {
			// The generator only writes supported filters, so this is a
			// parser bug unless jq rejects the filter too.
			want, ok, stderr := runJQ(t, bin, f, input)
			if ok {
				fail(fmt.Sprintf("filter %q: Parse failed but jq succeeded\nParse: %v\njq: %q\ninput: %s", f, err, want, input))
			} else if !strings.Contains(stderr, "compile error") {
				fail(fmt.Sprintf("filter %q: Parse failed, jq failed at run time\nParse: %v\njq: %s", f, err, stderr))
			}
			ours[i].skip = true
			continue
		}
		buf := &limitedBuffer{limit: 64 << 10}
		err = q.Write(buf, []byte(input))
		if errors.Is(err, errTooLarge) {
			ours[i].skip = true
			skipped++
			continue
		}
		out := buf.String()
		if err != nil {
			ours[i].unsupported = strings.Contains(err.Error(), "not supported in clix --jq")
			out += errMark + "\n"
		}
		ours[i].out = out
		kept = append(kept, i)
		if program.Len() > 0 {
			program.WriteString(",\n")
		}
		// \u0001 is the marker's first character, in jq's string syntax.
		fmt.Fprintf(&program, `"\u0001CASE%d", (try (%s`+"\n"+`) catch "\u0001ERR")`, i, f)
	}
	if len(kept) == 0 {
		return 0, skipped
	}
	cmd := exec.Command(bin.path, "-r", program.String())
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	batchOK := err == nil
	jqOut := map[int]string{}
	if batchOK {
		cur := -1
		for _, line := range strings.SplitAfter(stdout.String(), "\n") {
			if strings.HasPrefix(line, caseMark) {
				cur, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, caseMark)))
				jqOut[cur] = ""
				continue
			}
			if cur >= 0 {
				jqOut[cur] += line
			}
		}
	}
	for _, i := range kept {
		if batchOK && jqOut[i] == ours[i].out {
			compared++
			continue
		}
		// Run the case on its own to confirm.
		want, ok, jqErr := runJQ(t, bin, filters[i], input)
		if !ok {
			want += errMark + "\n"
		}
		if ours[i].unsupported && strings.HasPrefix(want, strings.TrimSuffix(ours[i].out, errMark+"\n")) {
			// clix refused something jq accepts, after printing the same
			// results up to that point: the documented behavior.
			compared++
			continue
		}
		if want != ours[i].out {
			fail(fmt.Sprintf("filter %q: output differs\nclix: %q\njq:   %q\njq stderr: %s\ninput: %s", filters[i], ours[i].out, want, jqErr, input))
		}
		compared++
	}
	return compared, skipped
}

// fuzzGen writes random inputs and filters.
type fuzzGen struct {
	r      *rand.Rand
	decNum bool
	recs   int // ".." used in the current filter
}

var (
	fuzzKeys        = []string{"a", "b", "c", "id", "name", "tags", "x", "é", "a b", "", "if"}
	fuzzStrings     = []string{"a", "b", "x", "", "A", "B", "abc", "ABC", "name", "Widget", "é", "😀", "a\nb", "line\n", "tab\t", "q\"", `back\`, "\u007f", "\u001f", " ", "<>&", "a.b", "αβγ", "1", "true", "null", "\u0000"}
	fuzzSafeNumbers = []string{"0", "1", "2", "3", "-1", "-2", "10", "42", "1.5", "2.5", "0.1", "-0.5", "123456789", "9007199254740991"}
	fuzzNumbers     = []string{"0", "1", "2", "3", "-1", "-2", "10", "42", "1.5", "2.5", "0.1", "-0.5", "1e-7", "1e+21", "1.5e+300", "9007199254740993", "100000000000000000001", "18446744073709551615", "-9223372036854775808", "123456789012345678"}
	// fuzzRegexes are patterns valid in RE2 and Oniguruma with the same
	// meaning (see compileRegex).
	fuzzRegexes        = []string{"a", "^a", "a$", "^$", "", ".", "b.c", "[a-c]", "[^a]", "a|b", "(ab)+", "x*", `\\.`, "é", `^[0-9]+$`, `\\p{L}`, `\\p{Lu}`, `[\\p{L}0-9]`, "a{2}", "^a{1,2}$", "(?:a|b)c$", "(?<n>a)b", `\\x41`, `\\x{e9}`, `\\t`, `\\n$`, "foo$|bar", "^(a|bc)$", `[a\\-z]`, "[-a]", "[a-]", "a+?", `\\p{Greek}`, `\\P{L}`, `\\$`, `\\^`, `\\\\`, "[.]", `\\n`, "b$", "^.$", "^..$", "[ab]$", "(a$|b)", "^$|x"}
	fuzzLiterals       = []string{"null", "true", "false", "0", "1", "2", "-1", "1.5", "0.1", `"a"`, `"x"`, `""`, `"é"`, `"\n"`, `"a\tb"`, `"name"`, `"é"`, `"A"`, "[]", "{}"}
	fuzzLiteralNumbers = []string{"1.0", "1e2", "1.50", ".5", "1.", "100000000000000000001", "9007199254740993", "-0", "1e1000", "0.0", "1E-7"}
)

func (g *fuzzGen) pick(list []string) string { return list[g.r.Intn(len(list))] }

// input returns a random JSON document.
func (g *fuzzGen) input() string {
	var v any
	if g.r.Intn(4) == 0 {
		v = g.array(3)
	} else {
		v = g.object(3)
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		panic(err)
	}
	return b.String()
}

// orderedObject marshals its keys in order, as FormatData writes struct
// fields in declaration order.
type orderedObject struct {
	keys []string
	vals []any
}

func (o orderedObject) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		vb, err := json.Marshal(o.vals[i])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func (g *fuzzGen) value(depth int) any {
	n := 6
	if depth > 0 {
		n = 9
	}
	switch g.r.Intn(n) {
	case 0:
		return nil
	case 1:
		return g.r.Intn(2) == 0
	case 2:
		if !g.decNum {
			// Without decNumber, jq prints numbers as doubles; keep to
			// numbers whose text is the same either way.
			return json.Number(g.pick(fuzzSafeNumbers))
		}
		return json.Number(g.pick(fuzzNumbers))
	case 3:
		if !g.decNum {
			return float64(g.r.Intn(20000)-10000) / 16
		}
		return g.float()
	case 4, 5:
		return g.pick(fuzzStrings)
	case 6:
		return g.array(depth - 1)
	default:
		return g.object(depth - 1)
	}
}

// float returns a float64 as encoding/json formats it.
func (g *fuzzGen) float() any {
	switch g.r.Intn(4) {
	case 0:
		return float64(g.r.Intn(2000)-1000) / 8
	case 1:
		return g.r.NormFloat64() * math.Pow(10, float64(g.r.Intn(40)-20))
	case 2:
		return float64(g.r.Int63n(1 << 60))
	}
	return g.r.Float64()
}

func (g *fuzzGen) array(depth int) any {
	arr := []any{}
	for i := g.r.Intn(5); i > 0; i-- {
		arr = append(arr, g.value(depth))
	}
	return arr
}

func (g *fuzzGen) object(depth int) any {
	o := orderedObject{}
	seen := map[string]bool{}
	for i := g.r.Intn(6); i > 0; i-- {
		k := g.pick(fuzzKeys)
		if seen[k] {
			continue
		}
		seen[k] = true
		o.keys = append(o.keys, k)
		o.vals = append(o.vals, g.value(depth))
	}
	if g.r.Intn(2) == 0 {
		// Sorted, as FormatData writes maps.
		sortOrdered(&o)
	}
	return o
}

func sortOrdered(o *orderedObject) {
	for i := 1; i < len(o.keys); i++ {
		for j := i; j > 0 && o.keys[j] < o.keys[j-1]; j-- {
			o.keys[j], o.keys[j-1] = o.keys[j-1], o.keys[j]
			o.vals[j], o.vals[j-1] = o.vals[j-1], o.vals[j]
		}
	}
}

// filter returns a random filter from the supported subset.
func (g *fuzzGen) filter() string {
	g.recs = 0
	return g.expr(3)
}

func paren(s string) string { return "(" + s + ")" }

func (g *fuzzGen) expr(d int) string {
	if d <= 0 {
		return g.term(0)
	}
	switch g.r.Intn(16) {
	case 0, 1:
		return g.expr(d-1) + g.pick([]string{" | ", " | ", " |\r\n", " | # note\n", " | # note \\\n still a note\n", " | # note \\\\\n"}) + g.expr(d-1)
	case 2:
		return g.expr(d-1) + ", " + g.expr(d-1)
	case 3:
		return paren(g.expr(d-1)) + " // " + paren(g.expr(d-1))
	case 4:
		return paren(g.expr(d-1)) + g.pick([]string{" and ", " or "}) + paren(g.expr(d-1))
	case 5, 6:
		return paren(g.expr(d-1)) + g.pick([]string{" == ", " != ", " < ", " <= ", " > ", " >= "}) + paren(g.expr(d-1))
	case 7:
		switch g.r.Intn(3) {
		case 0:
			return "if " + g.expr(d-1) + " then " + g.expr(d-1) + " else " + g.expr(d-1) + " end"
		case 1:
			return "if " + g.expr(d-1) + " then " + g.expr(d-1) + " elif " + g.expr(d-1) + " then " + g.expr(d-1) + " end"
		}
		return "if " + g.expr(d-1) + " then " + g.expr(d-1) + " end"
	case 8:
		return paren(g.expr(d-1)) + "?"
	}
	return g.term(d)
}

func (g *fuzzGen) key() string { return g.pick(fuzzKeys) }

func isIdent(k string) bool {
	if k == "" || !isIdentStart(k[0]) {
		return false
	}
	for i := 1; i < len(k); i++ {
		if !isIdentChar(k[i]) {
			return false
		}
	}
	return true
}

// field returns .k or ."k".
func (g *fuzzGen) field() string {
	k := g.key()
	if isIdent(k) && g.r.Intn(3) > 0 {
		return "." + k
	}
	return "." + strconv.Quote(k)
}

func (g *fuzzGen) path(d int) string {
	var b strings.Builder
	switch g.r.Intn(3) {
	case 0:
		b.WriteString(".")
		if g.r.Intn(2) == 0 {
			// . followed by a bracket suffix, as in .[0].
			b.WriteString("[" + g.pick([]string{"0", "-1", `"a"`, "1.5"}) + "]")
		}
	default:
		b.WriteString(g.field())
	}
	for i := g.r.Intn(4); i > 0; i-- {
		if b.String() == "." {
			b.Reset()
			b.WriteString(g.field())
			continue
		}
		switch g.r.Intn(7) {
		case 0, 1:
			b.WriteString(g.field())
		case 2:
			b.WriteString("[" + g.pick([]string{"0", "1", "-1", "2", "0,1", "-2", "1.5"}) + "]")
		case 3:
			b.WriteString("[]")
		case 4:
			if d > 0 {
				b.WriteString("[" + g.expr(d-1) + "]")
			} else {
				b.WriteString(`["` + g.key() + `"]`)
			}
		case 5:
			b.WriteString("[]?")
		case 6:
			b.WriteString(g.field() + "?")
		}
	}
	return b.String()
}

func (g *fuzzGen) literal() string {
	if g.decNum && g.r.Intn(5) == 0 {
		return g.pick(fuzzLiteralNumbers)
	}
	return g.pick(fuzzLiterals)
}

func (g *fuzzGen) term(d int) string {
	n := 6
	if d > 0 {
		n = 16
	}
	switch g.r.Intn(n) {
	case 0, 1, 2:
		return g.path(d)
	case 3:
		return g.literal()
	case 4:
		return g.call0()
	case 5:
		if g.recs == 0 && g.r.Intn(4) == 0 {
			g.recs++
			return ".."
		}
		return "."
	case 6:
		return "[" + g.expr(d-1) + "]"
	case 7:
		return g.objectCons(d - 1)
	case 8:
		return `"` + g.pick([]string{"", "a", "x-", "é "}) + `\(` + g.expr(d-1) + `)` + g.pick([]string{"", "b", "\\n", "\\(.)"}) + `"`
	case 9, 10, 11:
		return g.call1(d - 1)
	case 12:
		return paren(g.expr(d - 1))
	case 13:
		if g.decNum && g.r.Intn(2) == 0 {
			return "-" + g.pick(fuzzLiteralNumbers)
		}
		return "-" + g.pick([]string{"1", "0", "1.5", "2"})
	case 14:
		return "@" + g.pick([]string{"tsv", "csv", "json", "text"})
	}
	if d > 0 && g.r.Intn(2) == 0 {
		return g.suffixed(d - 1)
	}
	if d > 0 && g.r.Intn(2) == 0 {
		// Collect values, then apply a function that wants an array.
		return "[" + g.expr(d-1) + "] | " + g.pick([]string{"@tsv", "@csv", `join(",")`, "sort", "first", "last", "length", "keys", "to_entries", "map(tostring)", "sort_by(type)", "map(values)"})
	}
	return g.path(d) + "?"
}

// suffixed writes a non-path term followed by index suffixes, as in
// (e).a, [1,2][0] or {a: 1}.a?.
func (g *fuzzGen) suffixed(d int) string {
	var base string
	switch g.r.Intn(6) {
	case 0:
		base = paren(g.expr(d))
	case 1:
		base = "[" + g.expr(d) + "]"
	case 2:
		base = g.objectCons(d)
	case 3:
		// jq 1.8: if ... end is a term that takes suffixes.
		base = "if " + g.expr(d) + " then " + g.expr(d) + " else " + g.expr(d) + " end"
	case 4:
		// jq 1.8: e? is a term that takes suffixes.
		base = paren(g.expr(d)) + "?"
	default:
		base = g.pick([]string{`"abc"`, "1", "null", "[1,2]", `{"a":1}`, "[]", "{}"})
	}
	for i := g.r.Intn(3) + 1; i > 0; i-- {
		switch g.r.Intn(5) {
		case 0:
			base += g.field()
		case 1:
			base += "[" + g.pick([]string{"0", "-1", `"a"`, "1"}) + "]"
		case 2:
			base += "[]"
		case 3:
			base += ".[" + g.pick([]string{"0", `"a"`}) + "]"
		}
		if g.r.Intn(3) == 0 {
			base += "?"
		}
	}
	return base
}

func (g *fuzzGen) call0() string {
	return g.pick([]string{"keys", "keys_unsorted", "length", "to_entries", "first", "last", "tostring", "tojson", "ascii_downcase", "ascii_upcase", "type", "values", "empty", "not", "sort", "true", "false", "null"})
}

func (g *fuzzGen) call1(d int) string {
	switch g.r.Intn(9) {
	case 0:
		return "select(" + g.expr(d) + ")"
	case 1:
		return "map(" + g.expr(d) + ")"
	case 2:
		return "has(" + g.pick([]string{`"a"`, `"id"`, `"x"`, "0", "1", "-1", g.expr(d)}) + ")"
	case 3:
		return "join(" + g.pick([]string{`", "`, `"-"`, `""`, "null", g.expr(d)}) + ")"
	case 4:
		return `test("` + g.pick(fuzzRegexes) + `")`
	case 5:
		return "sort_by(" + g.expr(d) + ")"
	case 6:
		return g.pick([]string{"first", "last"}) + "(" + g.expr(d) + ")"
	case 7:
		return "test(" + g.pick([]string{`("a", "^x")`, `"b$"`, `.`, `.name`}) + ")"
	}
	return g.call0()
}

// objectCons writes {...}; values are terms, as jq's grammar requires.
// dictExpr returns an object value as jq 1.8 allows it without
// parentheses (DictExpr): expressions with //, or, and, comparisons and
// "|" but no top-level comma.
func (g *fuzzGen) dictExpr(d int) string {
	if d <= 0 {
		return g.term(0)
	}
	switch g.r.Intn(7) {
	case 0:
		return g.dictExpr(d-1) + " | " + g.dictExpr(d-1)
	case 1:
		return g.term(d-1) + " // " + g.dictExpr(d-1)
	case 2:
		return g.term(d-1) + g.pick([]string{" and ", " or "}) + g.term(d-1)
	case 3:
		return g.term(d-1) + g.pick([]string{" == ", " != ", " < ", " >= "}) + g.term(d-1)
	case 4:
		return "if " + g.expr(d-1) + " then " + g.expr(d-1) + " else " + g.expr(d-1) + " end"
	case 5:
		return g.term(d-1) + "?"
	}
	return g.term(d)
}

func (g *fuzzGen) objectCons(d int) string {
	var entries []string
	for i := g.r.Intn(3); i >= 0; i-- {
		value := g.path(0)
		switch {
		case d > 0 && g.r.Intn(3) == 0:
			value = paren(g.expr(d))
		case d > 0 && g.r.Intn(2) == 0:
			value = g.dictExpr(d)
		}
		switch g.r.Intn(5) {
		case 0:
			k := g.key()
			if isIdent(k) {
				entries = append(entries, k)
				continue
			}
			entries = append(entries, strconv.Quote(k))
		case 1:
			entries = append(entries, strconv.Quote(g.key())+": "+value)
		case 2:
			if d > 0 {
				entries = append(entries, "("+g.expr(d)+"): "+value)
				continue
			}
			fallthrough
		default:
			k := g.key()
			if !isIdent(k) {
				k = strconv.Quote(k)
			}
			entries = append(entries, k+": "+value)
		}
	}
	return "{" + strings.Join(entries, ", ") + "}"
}
