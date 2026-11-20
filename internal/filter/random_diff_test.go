package filter

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2/internal/gcloudtest"
	"github.com/SCKelemen/clix/v2/internal/resource"
)

// TestFilterRandomMatchesGcloud compares random filters on random
// resources with gcloud. CLIX_GCLOUD_CASES sets the number of cases
// (default 3000) and CLIX_GCLOUD_SEED the seed.
func TestFilterRandomMatchesGcloud(t *testing.T) {
	n := 3000
	if s := os.Getenv("CLIX_GCLOUD_CASES"); s != "" {
		n, _ = strconv.Atoi(s)
	}
	seed := uint64(1)
	if s := os.Getenv("CLIX_GCLOUD_SEED"); s != "" {
		seed, _ = strconv.ParseUint(s, 10, 64)
	}
	rng := rand.New(rand.NewPCG(seed, seed))
	g := &gen{rng: rng}
	cases := make([]filterCase, n)
	for i := range cases {
		cases[i] = filterCase{JSON: g.doc(), Filter: g.expr(0)}
	}
	var want []filterResult
	gcloudtest.Run(t, filterScript, cases, &want)
	compared, unsupportedCount, failures := 0, 0, 0
	for i, c := range cases {
		w := want[i]
		got, err := run(c.Filter, c.JSON)
		var unsupported *resource.UnsupportedError
		switch {
		case errors.As(err, &unsupported):
			unsupportedCount++
			continue
		case err != nil && w.Error == "":
			t.Errorf("filter %q on %s: %v; gcloud %v", c.Filter, c.JSON, err, w.Result)
			failures++
		case err == nil && w.Error != "":
			t.Errorf("filter %q on %s = %v; gcloud failed: %s", c.Filter, c.JSON, got, w.Error)
			failures++
		case err == nil:
			compared++
			if fmt.Sprint(got) != fmt.Sprint(w.Result) {
				t.Errorf("filter %q on %s:\n got %v\nwant %v", c.Filter, c.JSON, got, w.Result)
				failures++
			}
		}
		if failures > 30 {
			t.Fatal("too many failures")
		}
	}
	t.Logf("%d cases (seed %d): %d compared with gcloud, %d unsupported", n, seed, compared, unsupportedCount)
}

type gen struct{ rng *rand.Rand }

func (g *gen) pick(options ...string) string { return options[g.rng.IntN(len(options))] }

var genKeys = []string{"name", "n", "s", "t", "b", "labels.env", "labels", "tags.items", "tags.items[0]", "disks.name",
	"disks.size", "disks[]", "zone", "region", "missing", "project_id", "projectId", "list", "obj", "obj.k"}

var genOperands = []string{"abc", "ABC", "a*", "*c", "a*c", "*", "10", "007", "1.5", "-1", "0", "true", "false", "True",
	"null", "''", `""`, "'a b'", "web", "db", "prod", "Café", "cafe", "line", "us-central1-a", "us-central1", "2020-01-01",
	"2020-01-01T00:00:00Z", "2021-06-15T12:30:00+02:00", "1e3", "nan", "inf", "x_y", "<b>", "é", "1_0", " 5", "'10'",
	"run*", "RUNNING", "hello", "world"}

var genOps = []string{":", "=", "!=", "<", "<=", ">", ">=", ":", "="}

func (g *gen) term() string {
	key := g.pick(genKeys...)
	op := g.pick(genOps...)
	var operand string
	if (op == ":" || op == "=") && g.rng.IntN(8) == 0 {
		operand = "(" + g.pick(genOperands...) + " " + g.pick(genOperands...) + ")"
	} else {
		operand = g.pick(genOperands...)
	}
	t := key + op + operand
	if g.rng.IntN(6) == 0 {
		t = "-" + t
	}
	return t
}

func (g *gen) expr(depth int) string {
	if depth > 2 || g.rng.IntN(3) == 0 {
		return g.term()
	}
	switch g.rng.IntN(6) {
	case 0:
		return g.expr(depth+1) + " AND " + g.expr(depth+1)
	case 1:
		return g.expr(depth+1) + " OR " + g.expr(depth+1)
	case 2:
		return g.expr(depth+1) + " " + g.expr(depth+1)
	case 3:
		return "NOT " + g.term()
	case 4:
		return "(" + g.expr(depth+1) + ")"
	default:
		return g.term()
	}
}

var genStrings = []string{"abc", "ABC", "abcpdqxyz", "a.b.c", "xyz", "Café", "cafe", "line\nabc", "<b>abc</b>", "", "10",
	"007", "1.5", "-1", "true", "True", "null", "a b", "web", "prod", "us-central1-a",
	"https://x/zones/us-central1-a", "2020-01-01T00:00:00Z", "2021-06-15T10:30:00Z", "2019-12-31 23:59:59", "1e3", "nan",
	"RUNNING", "running", "hello world", "x_y", "é", " 5 ", "a*c"}

func (g *gen) value(depth int) any {
	switch g.rng.IntN(12) {
	case 0:
		return nil
	case 1:
		return g.rng.IntN(2) == 0
	case 2:
		return g.rng.IntN(25) - 5
	case 3:
		return []float64{1.5, 0.0, -1.0, 1e3, 10.0, 2.5e-7}[g.rng.IntN(6)]
	case 4, 5:
		if depth < 2 {
			var l []any
			for range g.rng.IntN(4) {
				l = append(l, g.value(depth+1))
			}
			return l
		}
	case 6:
		if depth < 2 {
			m := map[string]any{}
			for range g.rng.IntN(3) {
				m[g.pick("k", "env", "a", "hello")] = g.value(depth + 1)
			}
			return m
		}
	}
	return g.pick(genStrings...)
}

func (g *gen) doc() string {
	var list []map[string]any
	for range 1 + g.rng.IntN(5) {
		r := map[string]any{}
		for _, k := range []string{"name", "n", "s", "t", "b", "labels", "tags", "disks", "zone", "region", "projectId", "list", "obj"} {
			if g.rng.IntN(3) == 0 {
				continue
			}
			switch k {
			case "labels":
				r[k] = map[string]any{"env": g.value(1)}
			case "tags":
				r[k] = map[string]any{"items": g.value(0)}
			case "disks":
				var ds []any
				for range g.rng.IntN(3) {
					ds = append(ds, map[string]any{"name": g.value(1), "size": g.value(1)})
				}
				r[k] = ds
			case "t":
				r[k] = g.pick("2020-01-01T00:00:00Z", "2021-06-15T10:30:00Z", "2019-12-31T23:59:59.5Z", "2020-01-01T01:00:00+01:00", "", "2020-01-01")
			default:
				r[k] = g.value(0)
			}
		}
		list = append(list, r)
	}
	b, err := json.Marshal(list)
	if err != nil {
		panic(err)
	}
	return strings.ReplaceAll(string(b), `\u003c`, "<")
}
