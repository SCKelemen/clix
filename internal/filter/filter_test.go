package filter

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

// projects is a list like gcloud projects list's, for examples.
const projects = `[
	{"projectId": "web-prod", "name": "Web", "lifecycleState": "ACTIVE", "labels": {"env": "prod"}, "projectNumber": "1001", "createTime": "2021-02-10T19:19:49.242Z", "zone": "https://compute/zones/us-central1-a"},
	{"projectId": "web-test", "name": "Web (test)", "lifecycleState": "ACTIVE", "labels": {"env": "test"}, "projectNumber": "1002", "createTime": "2018-01-15T00:00:00Z", "zone": "us-east1-b"},
	{"projectId": "old", "name": "Café", "lifecycleState": "DELETE_REQUESTED", "labels": {}, "projectNumber": "99", "createTime": "2017-05-05T05:05:05Z"}
]`

func TestSelect(t *testing.T) {
	for _, tt := range []struct {
		filter string
		want   string // the selected projectIds
	}{
		{"", "web-prod web-test old"},
		{"lifecycleState:DELETE_REQUESTED", "old"},
		{"projectId:web", "web-prod web-test"},
		{"projectId:web*", "web-prod web-test"},
		{"projectId:*test", "web-test"},
		{"project_id=old", "old"},
		{"name:cafe", "old"}, // accents and case are ignored
		{"name=web", "web-prod"},
		{"labels.env=prod OR labels.env=test", "web-prod web-test"},
		{"labels.env:*", "web-prod web-test"},
		{"-labels.env:*", "old"},
		{"NOT labels.env=prod", "web-test old"},
		{"projectNumber>1000", "web-prod web-test"}, // "1001" is read as a number
		{"projectNumber<100", "old"},
		{"labels.env=(prod test)", "web-prod web-test"},
		{"projectId:web lifecycleState=ACTIVE OR name:cafe", "web-prod web-test"},
		{"(projectId:web AND labels.env=test) OR projectId=old", "web-test old"},
		{"createTime>=2018-01-15", "web-prod web-test"},
		{"createTime<2018-01-15T00:00:00Z", "old"},
		{"zone=us-central1-a", "web-prod"}, // the last part of a zone URL
	} {
		got, err := selectIDs(tt.filter, projects)
		if err != nil {
			t.Errorf("%q: %v", tt.filter, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%q selects %q, want %q", tt.filter, got, tt.want)
		}
	}
}

func selectIDs(expr, doc string) (string, error) {
	f, err := Parse(expr, WithLocation(time.UTC))
	if err != nil {
		return "", err
	}
	v, err := resource.Decode([]byte(doc))
	if err != nil {
		return "", err
	}
	list := v.([]any)
	sel, err := f.Select(list)
	if err != nil {
		return "", err
	}
	var ids []string
	for _, i := range sel {
		id, _ := list[i].(*resource.Object).Get("projectId")
		ids = append(ids, id.(string))
	}
	return strings.Join(ids, " "), nil
}

func TestErrors(t *testing.T) {
	for _, tt := range []struct {
		filter      string
		unsupported bool
		msg         string
	}{
		{"a=1 AND b=1 OR c=1", false, "Parenthesis grouping is required when AND and OR are are combined [a=1 AND b=1 OR *HERE* c=1]."},
		{"a=1 OR b=1 AND c=1", false, "Parenthesis grouping is required when AND and OR are are combined"},
		{"name", true, `a term without an operator ("name", a gcloud global restriction) is not supported in clix --filter`},
		{"name~abc", true, "the ~ operator (a Python regular expression) is not supported in clix --filter"},
		{"name!~abc", true, "the !~ operator"},
		{"name.len()=3", true, "calling a transform such as name.len() in a term"},
		{"*a=1", true, "the * map shorthand in a key"},
		{"a[x]=1", true, "the key index [x]"},
		{"name:", false, "Term operand expected"},
		{"name:=a", false, "Malformed operator [name *HERE* :=a]."},
		{"(a=1", false, "More tokens expected"},
		{"a=1)", false, "Unexpected tokens [a=1 *HERE* )] in expression."},
		{"a=1 AND", false, "Term expected"},
		{"a=OR", false, "Logical operator not expected"},
		{"name:a*b*c", false, "At most one * expected in : patterns [a*b*c]."},
		{`name="abc`, false, "Unterminated"},
		{"NOT ()", false, "Term expected"},
	} {
		_, err := Parse(tt.filter)
		if err == nil {
			t.Errorf("%q parsed", tt.filter)
			continue
		}
		var unsupported *resource.UnsupportedError
		var syntax *resource.SyntaxError
		if tt.unsupported && !errors.As(err, &unsupported) || !tt.unsupported && !errors.As(err, &syntax) {
			t.Errorf("%q: %T %v, want unsupported=%v", tt.filter, err, err, tt.unsupported)
		}
		if !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("%q: %q, want it to contain %q", tt.filter, err, tt.msg)
		}
	}

	// Errors that depend on the data.
	for _, tt := range []struct{ filter, doc, msg string }{
		{"t:2021", `[{"t":"2021-02-10T19:19:49Z"}]`, "the : operator on t, which holds dates and times, is not supported in clix --filter"},
		{"t>2021", `[{"t":"2021-02-10T19:19:49Z"}]`, `comparing t, which holds dates and times, with "2021"`},
		{"t>-P2W", `[{"t":"2021-02-10T19:19:49Z"}]`, `with "-P2W"`},
		{"t=(2021-01-01 2022-01-01)", `[{"t":"2021-02-10T19:19:49Z"}]`, "a list of operands"},
		{"t>2021-01-01", `[{"t":"2021-02-10T19:19:49Z"},{"t":"pending"}]`, `reading "pending" of t`},
		{"t>2021-01-01", `[{"t":"2021-02-10 19:19:49 UTC"}]`, `reading "2021-02-10 19:19:49 UTC" of t as a date and time`},
		{"n.__doc__:x", `[{"n":1}]`, "__doc__"},
	} {
		f, err := Parse(tt.filter)
		if err != nil {
			t.Fatalf("%q: %v", tt.filter, err)
		}
		v, _ := resource.Decode([]byte(tt.doc))
		_, err = f.Select(v.([]any))
		var unsupported *resource.UnsupportedError
		if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("%q on %s: %v, want an *UnsupportedError containing %q", tt.filter, tt.doc, err, tt.msg)
		}
	}
}

func TestDateTimeTimeZones(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	doc := `[{"t":"2021-06-01T03:59:59Z"},{"t":"2021-06-01T04:00:00Z"},{"t":"2021-06-01 00:00:00"}]`
	f, err := Parse("t>=2021-06-01", WithLocation(ny)) // midnight in New York is 04:00 UTC in June
	if err != nil {
		t.Fatal(err)
	}
	v, _ := resource.Decode([]byte(doc))
	sel, err := f.Select(v.([]any))
	if err != nil || fmt.Sprint(sel) != "[1 2]" {
		t.Errorf("Select = %v, %v; want [1 2]", sel, err)
	}
	// 2021-03-14 02:30 does not exist in New York, and 2021-11-07 01:30
	// happens twice.
	for _, op := range []string{"2021-03-14T02:30:00", "2021-11-07T01:30:00"} {
		f, err := Parse("t>"+op, WithLocation(ny))
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.Select(v.([]any))
		var unsupported *resource.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Errorf("t>%s: %v, want an *UnsupportedError", op, err)
		}
	}
}

func TestParseDateTime(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"2018-01-15", "2018-01-15T00:00:00Z"},
		{"2018-01-15T12:30", "2018-01-15T12:30:00Z"},
		{"2018-01-15 12:30:15", "2018-01-15T12:30:15Z"},
		{"2018-01-15T12:30:15.1234567Z", "2018-01-15T12:30:15.123456Z"},
		{"2018-01-15T12:30:15+02:00", "2018-01-15T10:30:15Z"},
		{"2018-01-15T12:30:15-0530", "2018-01-15T18:00:15Z"},
		{"2020-02-29", "2020-02-29T00:00:00Z"},
	} {
		got, err := parseDateTime(tt.in, time.UTC)
		if err != nil || got.UTC().Format(time.RFC3339Nano) != tt.want {
			t.Errorf("parseDateTime(%q) = %v, %v; want %s", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{"2018", "2018-01", "20180115", "2018-1-15", "2018-01-15T12", "2018-01-15t12:00", "2018-01-15T12:00z",
		"2018-01-15Z", "2018-01-15T25:00", "2019-02-29", "2018-01-15T12:00:60", "2018-01-15T12:00 UTC", "0000-01-01", "２０１８-01-15",
		"2018-01-15T12:00:00.", "2018-01-15T12:00:00+2400"} {
		if _, err := parseDateTime(bad, time.UTC); err == nil {
			t.Errorf("parseDateTime(%q) succeeded", bad)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range append(append([]string{}, diffFilters...), "a=1 b=2 OR (c:3* AND -d:*)", `x="a\"b" y:'c d'`, "NOT NOT a=1") {
		f.Add(s)
	}
	docs := make([][]any, len(diffDocs))
	for i, d := range diffDocs {
		v, err := resource.Decode([]byte(d))
		if err != nil {
			f.Fatal(err)
		}
		docs[i] = v.([]any)
	}
	f.Fuzz(func(t *testing.T, expr string) {
		flt, err := Parse(expr, WithLocation(time.UTC))
		var unsupported *resource.UnsupportedError
		var syntax *resource.SyntaxError
		if err != nil {
			if !errors.As(err, &unsupported) && !errors.As(err, &syntax) {
				t.Fatalf("Parse(%q): %T %v", expr, err, err)
			}
			return
		}
		for _, doc := range docs {
			sel, err := flt.Select(doc)
			if err != nil {
				if !errors.As(err, &unsupported) {
					t.Fatalf("Select(%q): %T %v", expr, err, err)
				}
				continue
			}
			for k := 1; k < len(sel); k++ {
				if sel[k] <= sel[k-1] || sel[k] >= len(doc) {
					t.Fatalf("Select(%q) = %v", expr, sel)
				}
			}
		}
		// NOT selects the complement, unless the data makes the filter
		// unsupported.
		if expr != "" {
			neg, err := Parse("NOT ("+expr+")", WithLocation(time.UTC))
			if err != nil {
				return // ( ) changes what parses, such as a trailing ")"
			}
			for _, doc := range docs {
				a, errA := flt.Select(doc)
				b, errB := neg.Select(doc)
				if errA != nil || errB != nil {
					continue
				}
				if len(a)+len(b) != len(doc) {
					t.Fatalf("%q selects %v and its negation %v of %d", expr, a, b, len(doc))
				}
			}
		}
	})
}
