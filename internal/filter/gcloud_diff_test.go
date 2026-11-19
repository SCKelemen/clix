package filter

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/internal/gcloudtest"
	"github.com/SCKelemen/clix/v2/internal/resource"
)

// filterScript compiles each case's filter with gcloud and evaluates it
// on each resource of the case's list in order, as gcloud filters a list
// (one compiled filter, so terms keep their state).
// Its time zone is set by the first line, which filterScriptIn replaces.
const filterScript = `import os, time; os.environ["TZ"] = "UTC"; time.tzset()
from googlecloudsdk.core.resource import resource_filter
out = []
for c in json.load(sys.stdin):
    try:
        f = resource_filter.Compile(c["filter"])
    except Exception as e:
        out.append({"error": "syntax: " + type(e).__name__ + ": " + str(e)})
        continue
    try:
        out.append({"result": [bool(f.Evaluate(r)) for r in json.loads(c["json"])]})
    except Exception as e:
        out.append({"error": "eval: " + type(e).__name__ + ": " + str(e)})
print(json.dumps(out))
`

// filterScriptIn is filterScript in the time zone tz.
func filterScriptIn(tz string) string {
	return strings.Replace(filterScript, `"UTC"`, strconv.Quote(tz), 1)
}

type filterCase struct {
	JSON   string `json:"json"`
	Filter string `json:"filter"`
}

type filterResult struct {
	Result []bool `json:"result"`
	Error  string `json:"error"`
}

// diffDocs are resource lists the differential tests filter.
var diffDocs = []string{
	`[{"name":"abcpdqxyz"},{"name":"abc.pdq.xyz"},{"name":"ABC"},{"name":null},{"name":""},{"x":1},{"name":"Café au lait"},{"name":"line1\nline2"},{"name":"<b>bold</b>"},{"name":"a*b"}]`,
	`[{"n":10},{"n":"10"},{"n":null},{},{"n":true},{"n":false},{"n":[3,20]},{"n":"abc"},{"n":"007"},{"n":7},{"n":1.5},{"n":0},{"n":-0.0},{"n":"1e3"},{"n":"nan"},{"n":"inf"},{"n":[]},{"n":{}},{"n":{"a":1}},{"n":" 5 "}]`,
	`[{"name":"vm-1","zone":"https://compute/projects/p/zones/us-central1-a","status":"RUNNING","labels":{"env":"prod","team":"core"},"tags":{"items":["web","db"]},"disks":[{"name":"boot","sizeGb":10},{"name":"data","sizeGb":500}],"projectId":"p1"},` +
		`{"name":"vm-2","zone":"us-east1-b","status":"TERMINATED","labels":{"env":"test"},"tags":{"items":["web"]},"disks":[{"name":"boot","sizeGb":20}],"projectId":"p2"},` +
		`{"name":"db-1","zone":"europe-west1-c","status":"RUNNING","labels":{},"tags":{"items":[]},"disks":[],"projectId":"p1"},` +
		`{"name":"DB-2","region":"us-central1","status":"running","project_id":"p3"}]`,
	`[{"t":"2021-02-10T19:19:49.242Z","id":1},{"t":"2018-01-15T00:00:00Z","id":2},{"t":"2018-01-14T23:59:59.999999Z","id":3},{"t":"","id":4},{"id":5},{"t":"2021-02-10 19:19:49","id":6},{"t":"2019-06-01T12:00:00+02:00","id":7},{"t":"2019-06-01T10:00:00Z","id":8}]`,
	`[{"t":"pending"},{"t":"2021-01-01T00:00:00Z"},{"t":"2017-05-05T05:05:05Z"}]`,
	`[{"b":true},{"b":false},{"b":"true"},{"b":"True"},{"b":1},{"b":0},{"b":"yes"},{"b":null}]`,
	`[{"s":"Hello World"},{"s":"hello-world"},{"s":"HELLO"},{"s":"world hello"},{"s":"helloworld"},{"s":"say hello."},{"s":["hello","x"]},{"s":[["hello"]]},{"s":{"hello":1}},{"s":{"k":{"hello":1}}}]`,
}

var diffFilters = []string{
	// ":" with and without globs
	"name:pdq", "name:abc", "name:abc*", "name:pdq*", "name:xyz", "name:*xyz", "name:a*z", "name:*", "-name:*", "name:ABC",
	"name:cafe", "name:CAFÉ", "name:line2", "name:line1*", "name:*line1", "name:bold", "name:<b>", "name:'a*b'", "name:a\\*b",
	"name:(abc xyz)", "name:(abc,ABC)", "name:( abc , pdq )", "name:(abc AND xyz)", "name:null", "name:nul", "name:''",
	// "=" and "!="
	"name=abc", "name=ABC", "name=abcpdqxyz", "name=line2", "name=null", `name=""`, "name=(ABC abcpdqxyz)", "name!=ABC",
	"name!=null", "name=a*b",
	// numbers, constants and their text
	"n=10", "n=7", "n=007", "n=1.5", "n=true", "n=false", "n=0", "n>5", "n<5", "n>=10", "n<=1.5", "n>abc", "n<abc", "n:1",
	"n:10", "n=1e3", "n=1000", "n>1e2", "n!=10", "n:*", "-n:*", "n=nan", "n=inf", "n>inf", "n<nan", "n=5", "n:a",
	"n=(7 10)", "n:(1 abc)", "n>=-0", "n=1_0",
	// keys: nested, lists, case conversions, zone and region
	"labels.env=prod", "labels:prod", "labels:env", "labels.env:*", "-labels.env:*", "tags.items=web", "tags.items:db",
	"tags.items=(web db)", "disks.name=data", "disks.sizeGb>100", "disks[0].sizeGb>15", "disks[].name:boot", "project_id=p1",
	"projectId=p3", "zone=us-central1-a", "zone:us-central1", "zone:us-central1*", "region=us-central1", "status=running",
	"status:RUN*", "name:vm*", "name=vm-1", "-status=RUNNING",
	// logic
	"name:vm AND status=RUNNING", "name:vm OR status=RUNNING", "name:vm status=RUNNING", "NOT name:vm",
	"name:vm OR status=RUNNING labels.env=prod", "(name:vm AND status=RUNNING) OR name:db", "name:vm AND (status=RUNNING OR name:db)",
	"NOT (name:vm OR name:db)", "-(name:vm)", "name:vm AND NOT status=RUNNING", "((name:vm))", "", "()",
	// dates and times
	"t>2018-01-15", "t>=2018-01-15", "t<2018-01-15T00:00:00Z", "t=2018-01-15T00:00:00Z", "t!=2018-01-15T00:00:00Z",
	"t>2019-06-01T09:00:00Z", "t<2019-06-01T11:00:00+01:00", "t>2021-02-10T19:19:49", "t>=2018-01-14T23:59:59.9999995Z",
	"t=(2018-01-15T00:00:00Z 2019-06-01T10:00:00Z)", "t>'2018-01-15 00:00:00'", "t>2018-01-15T00:00", "t:2021", "t>2018",
	"t>-P2W", "t>abc", "t<2020-01-01", "t:*", "id>2 t>2018-01-15", "t>2018-01-15 id>2",
	// booleans
	"b=true", "b=True", "b=1", "b=false", "b:true", "b=yes", "b!=true", "b<true", "b>false",
	// word matches
	"s:hello", "s=hello", "s:world", "s=helloworld", "s:'hello world'", "s=HELLO", "s:hel*", "s:*lo", "s:h*o",
	// errors
	"name", "name:", "name=", "name~abc", "name!~abc", "name:=abc", "name==abc", "a AND b=1 OR c=1", "a=1 AND b=1 OR c=1",
	"a=1 OR b=1 AND c=1", "(a=1", "a=1)", "a=1 AND", "NOT", "a.b(1)=2", "name:a*b*c", "*a=1", "a=(1", "OR=1", "a=OR",
	"name.len()=3", "a[x]=1", "a=1 ()", "NOT ()", "a=1 OR ()", "name<(a b)", "name!=(a b)",
}

func diffCases() []filterCase {
	var cases []filterCase
	for _, d := range diffDocs {
		for _, f := range diffFilters {
			cases = append(cases, filterCase{d, f})
		}
	}
	return cases
}

// TestFilterMatchesGcloud compares every filter of diffFilters on every
// list of diffDocs with gcloud, or without gcloud, with its recorded
// results (see gcloudtest.Golden).
func TestFilterMatchesGcloud(t *testing.T) {
	cases := diffCases()
	var want []filterResult
	gcloudtest.Golden(t, "testdata/gcloud.json", filterScript, cases, &want)
	compared, unsupportedCount := 0, 0
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
		case err == nil && w.Error != "":
			t.Errorf("filter %q on %s = %v; gcloud failed: %s", c.Filter, c.JSON, got, w.Error)
		case err == nil:
			compared++
			if fmt.Sprint(got) != fmt.Sprint(w.Result) {
				t.Errorf("filter %q on %s:\n got %v\nwant %v", c.Filter, c.JSON, got, w.Result)
			}
		}
	}
	t.Logf("%d cases: %d compared with gcloud, %d unsupported", len(cases), compared, unsupportedCount)
}

// run filters the list in doc and reports, for each resource, whether the
// filter selects it. Dates and times without an offset are in UTC.
func run(expr, doc string) ([]bool, error) {
	return runIn(time.UTC, expr, doc)
}

func runIn(loc *time.Location, expr, doc string) ([]bool, error) {
	f, err := Parse(expr, WithLocation(loc))
	if err != nil {
		return nil, err
	}
	v, err := resource.Decode([]byte(doc))
	if err != nil {
		return nil, err
	}
	list := v.([]any)
	sel, err := f.Select(list)
	if err != nil {
		return nil, err
	}
	out := make([]bool, len(list))
	for _, i := range sel {
		out[i] = true
	}
	return out, nil
}

// TestFilterMatchesGcloudInTimeZones compares the filters on dates and
// times with gcloud in time zones with daylight saving time, one of them
// shifting by 30 minutes.
func TestFilterMatchesGcloudInTimeZones(t *testing.T) {
	if os.Getenv("CLIX_GCLOUD_SDK") == "" {
		t.Skip("set CLIX_GCLOUD_SDK to compare with gcloud")
	}
	docs := []string{
		diffDocs[3],
		`[{"t":"2021-03-14T06:59:59Z"},{"t":"2021-03-14T07:00:00Z"},{"t":"2021-11-07 01:30:00"},{"t":"2021-10-03T02:00:00+11:00"},{"t":"2021-04-04 01:45:00"}]`,
	}
	filters := []string{"t>2021-03-14T02:30:00", "t<2021-03-14", "t>=2021-03-14T01:59:59", "t=2021-11-07T01:30:00", "t>2021-11-07",
		"t<2021-04-04T01:45:00", "t<2021-10-03T02:15:00", "t>2018-01-15", "t<=2019-06-01T12:00:00", "t!=2021-03-14T06:59:59Z"}
	for _, tz := range []string{"America/New_York", "Australia/Lord_Howe", "Asia/Kolkata"} {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			t.Skipf("no time zone data: %v", err)
		}
		var cases []filterCase
		for _, d := range docs {
			for _, f := range filters {
				cases = append(cases, filterCase{d, f})
			}
		}
		var want []filterResult
		gcloudtest.Run(t, filterScriptIn(tz), cases, &want)
		for i, c := range cases {
			got, err := runIn(loc, c.Filter, c.JSON)
			var unsupported *resource.UnsupportedError
			switch {
			case errors.As(err, &unsupported):
			case err != nil && want[i].Error == "":
				t.Errorf("%s: filter %q on %s: %v; gcloud %v", tz, c.Filter, c.JSON, err, want[i].Result)
			case err == nil && want[i].Error != "":
				t.Errorf("%s: filter %q on %s = %v; gcloud failed: %s", tz, c.Filter, c.JSON, got, want[i].Error)
			case err == nil && fmt.Sprint(got) != fmt.Sprint(want[i].Result):
				t.Errorf("%s: filter %q on %s:\n got %v\nwant %v", tz, c.Filter, c.JSON, got, want[i].Result)
			}
		}
	}
}

func init() {
	// Keep the list of filters free of accidental duplicates.
	seen := map[string]bool{}
	for _, f := range diffFilters {
		if seen[f] {
			panic("duplicate filter " + f)
		}
		seen[f] = true
	}
	_ = strings.TrimSpace
}
