package format_test

import (
	"fmt"
	"strings"

	"github.com/SCKelemen/clix/v2/ext/format"
)

type instance struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	CPUs   int    `json:"cpus"`
}

var fleet = []instance{
	{"web-1", "RUNNING", 2},
	{"web-2", "TERMINATED", 4},
	{"db-1", "RUNNING", 16},
	{"web-3", "RUNNING", 8},
}

// ApplyList filters, sorts and limits records as --filter, --sort-by and
// --limit would, and returns them rather than printing them.
func ExampleApplyList() {
	f, err := format.ParseFilter("status=running name:web*")
	if err != nil {
		panic(err)
	}
	sortBy, err := format.ParseSortBy("~cpus")
	if err != nil {
		panic(err)
	}
	kept, err := format.ApplyList(fleet, format.ListOptions{Filter: f, SortBy: sortBy, Limit: 1})
	if err != nil {
		panic(err)
	}
	fmt.Println(kept)
	// Output: [{web-3 RUNNING 8}]
}

// Split hands the terms a server can evaluate to the server and keeps
// the rest for the client. Here the server takes equality on status
// only; the client part is ANDed with it, so it can only narrow what the
// server returns.
func ExampleFilter_Split() {
	f, err := format.ParseFilter("status=RUNNING name:web* (cpus>4 OR name=db-1)")
	if err != nil {
		panic(err)
	}
	server, client := f.Split(func(e format.FilterExpr) bool {
		t, ok := e.(*format.FilterTerm)
		return ok && t.Key().String() == "status" && t.Op() == format.FilterEqual
	})
	fmt.Println("server:", server)
	fmt.Println("client:", client)

	// The server returns the RUNNING instances; the client keeps those
	// its part selects.
	var returned []instance
	for _, i := range fleet {
		if strings.EqualFold(i.Status, "RUNNING") {
			returned = append(returned, i)
		}
	}
	kept, err := format.ApplyList(returned, format.ListOptions{Filter: client})
	if err != nil {
		panic(err)
	}
	fmt.Println(kept)
	// Output:
	// server: status=RUNNING
	// client: name:web* AND (cpus>4 OR name=db-1)
	// [{web-3 RUNNING 8}]
}

// Expr returns the parse tree, for translating a filter into another
// language.
func ExampleFilter_Expr() {
	f, err := format.ParseFilter("labels.env=prod -name:test*")
	if err != nil {
		panic(err)
	}
	for _, e := range f.Expr().(*format.FilterAnd).Operands() {
		switch e := e.(type) {
		case *format.FilterTerm:
			fmt.Println("term:", e.Key(), e.Op(), e.Operands())
		case *format.FilterNot:
			fmt.Println("not:", e.Operand())
		}
	}
	// Output:
	// term: labels.env = [prod]
	// not: name:test*
}
