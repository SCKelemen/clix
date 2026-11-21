package format

import (
	"fmt"
	"math/big"
	"slices"

	"github.com/SCKelemen/clix/v2/internal/filter"
	"github.com/SCKelemen/clix/v2/internal/resource"
)

// Filter is a parsed --filter expression, in the subset of gcloud's filter
// syntax described in the package documentation. It is what ListFlags'
// --filter evaluates, as a value a command can use on its own: Select
// evaluates it on records, Expr returns its parse tree, and Split divides
// it between a server and the client.
//
// A Filter is immutable and safe for concurrent use. The zero Filter and
// the nil *Filter select every record, as the empty expression does.
type Filter struct {
	f    *filter.Filter
	expr FilterExpr // nil when f selects everything
}

// ParseFilter parses a --filter expression exactly as ListFlags' --filter
// does, with the same errors: a syntax error, or an error that says what
// "is not supported in clix --filter". An empty expression selects every
// record. Dates and times without a UTC offset are in the local time
// zone, as gcloud reads them.
func ParseFilter(expr string) (*Filter, error) {
	f, err := filter.Parse(expr)
	if err != nil {
		return nil, err
	}
	return &Filter{f: f, expr: newExpr(f, f.Root())}, nil
}

// everything is the internal filter that selects every record.
var everything = func() *filter.Filter {
	f, err := filter.Parse("")
	if err != nil {
		panic(err)
	}
	return f
}()

func (f *Filter) inner() *filter.Filter {
	if f == nil || f.f == nil {
		return everything
	}
	return f.f
}

// NewFilter returns a Filter that selects the records e selects, as if e
// were the whole expression: a subtree of a parsed Filter, such as one of
// the conjuncts Split hands to a callback. A nil e selects every record.
// NewFilter panics if e is not a node that Filter.Expr returned, such as
// a zero &FilterTerm{}, which has no expression to evaluate.
func NewFilter(e FilterExpr) *Filter {
	if e == nil {
		return &Filter{f: everything}
	}
	b := e.base()
	if b == nil || b.src == nil {
		panic(fmt.Sprintf("format: NewFilter of a %T that Filter.Expr did not return", e))
	}
	return &Filter{f: b.src.Sub(b.n), expr: e}
}

// Expr returns the parse tree of f, nil when f selects every record.
func (f *Filter) Expr() FilterExpr {
	if f == nil {
		return nil
	}
	return f.expr
}

// String returns f's expression in a canonical form that ParseFilter
// reads back as the same tree: AND and OR written out, parentheses where
// gcloud requires them, NOT for - before a term, and keys and operands
// quoted where needed ("name:'a b'" is name:"a b"). It is "" when f
// selects every record.
func (f *Filter) String() string { return f.inner().String() }

// Select returns the indices of the records f selects, in order, as
// --filter selects them: records is a list as Write takes one (a slice,
// an array, a pointer to one, or a value whose JSON is a list), and each
// record is read as its --format json output (see the package
// documentation), so struct fields are matched by their JSON names. nil
// has no records. Any other value is an error.
//
// One call evaluates the records as one list, as gcloud does: a term
// whose first non-empty value is a date and time compares dates and times
// for the rest of the list. To evaluate a list that comes in parts, such
// as pages, as one list, use a FilterSelector.
//
// Errors are those of --filter when Write applies it: what clix cannot
// evaluate on the data exactly as gcloud would is an error that says it
// "is not supported in clix --filter", never a different result.
func (f *Filter) Select(records any) ([]int, error) {
	return f.Selector().Select(records)
}

// Selector returns a new FilterSelector that evaluates f.
func (f *Filter) Selector() *FilterSelector {
	return &FilterSelector{s: f.inner().Selector()}
}

// FilterSelector evaluates a Filter on a list that comes in parts, such as
// the pages of a listing: calling Select on each part in turn selects
// what one Filter.Select of the whole list would, part by part. It is not
// safe for concurrent use. The zero FilterSelector and the nil
// *FilterSelector select every record, as the zero Filter's does.
type FilterSelector struct{ s *filter.Selector }

// Select returns the indices in records of the records the filter
// selects, in order, evaluated as the continuation of the records of
// earlier calls (see Filter.Select).
func (s *FilterSelector) Select(records any) ([]int, error) {
	list, err := readRecords(records)
	if err != nil {
		return nil, err
	}
	if s == nil || s.s == nil {
		return everything.Select(list) // keeps no state
	}
	return s.s.Select(list)
}

// Split divides f into two filters whose conjunction is f: server holds
// the conjuncts of f for which canServer returns true, and client holds
// the others, each in their order in f. The conjuncts are the operands of
// f's top-level AND (a AND b, a b, and a AND (b AND c), flattened), or f
// itself when it has none. Either filter may be empty, selecting every
// record; with a nil canServer, client is f.
//
// Split is for commands that hand the part of a --filter that a server
// can evaluate to the server, such as an AIP-160 filter in a List
// request, and evaluate the rest on the client:
//
//	server, client := f.Split(func(e format.FilterExpr) bool { return toAIP160(e) != "" })
//	req.Filter = toAIP160(server.Expr())
//	kept, err := client.Select(resp.Items)
//
// The safety rule: client is ANDed with server, never ORed, so evaluating
// it on the server's results can only remove records from them; it
// never adds one. Split moves whole conjuncts only. It never sends one
// operand of an OR, or what is under a NOT, to the server: evaluating
// the rest on the client could not bring back the records the server
// would then drop.
//
// The result is exactly f's when the server returns exactly the records
// server selects. canServer must therefore return true only for a
// conjunct that the server evaluates as gcloud does, or more broadly
// (returning every record gcloud's evaluation would, and maybe more),
// judging the whole conjunct: a server whose a is broader than gcloud's
// has a NOT a that is narrower. A server whose = is case-sensitive, where
// gcloud's is not, is narrower and would drop records. When the server
// may return more than server selects, evaluate f itself, not client,
// on its results: f.Select keeps exactly f's records of any superset of
// them.
//
// A term that compares dates and times decides so by the first value it
// sees (see Select), and client sees only what the server returned, so
// for a key whose values mix dates and times with other text, client can
// decide differently from f.
func (f *Filter) Split(canServer func(FilterExpr) bool) (server, client *Filter) {
	var onServer, onClient []FilterExpr
	for _, e := range f.Conjuncts() {
		if canServer != nil && canServer(e) {
			onServer = append(onServer, e)
		} else {
			onClient = append(onClient, e)
		}
	}
	return f.all(onServer), f.all(onClient)
}

// Conjuncts returns the operands of f's top-level AND (flattened, as
// FilterAnd.Operands), or f's expression alone when it is not an AND, and
// nothing when f selects every record. f selects a record when all of
// them do. They are the units Split divides.
func (f *Filter) Conjuncts() []FilterExpr {
	switch e := f.Expr().(type) {
	case nil:
		return nil
	case *FilterAnd:
		return e.Operands()
	default:
		return []FilterExpr{e}
	}
}

// all returns the conjunction of exprs, f's conjuncts, in order.
func (f *Filter) all(exprs []FilterExpr) *Filter {
	switch len(exprs) {
	case 0:
		return &Filter{f: everything}
	case 1:
		return NewFilter(exprs[0])
	}
	nodes := make([]filter.Node, len(exprs))
	for i, e := range exprs {
		nodes[i] = e.base().n
	}
	sub := f.inner().Sub(nodes...)
	return &Filter{f: sub, expr: &FilterAnd{filterNode{sub, sub.Root()}, exprs}}
}

// FilterExpr is a node of a parsed filter: a *FilterTerm, *FilterAnd,
// *FilterOr or *FilterNot. Nodes are read only, and Expr returns the same
// nodes every time, so they may be compared and used as map keys.
// Grouping is the shape of the tree: (a OR b) c is an AND whose first
// operand is an OR. NewFilter evaluates a node on its own.
type FilterExpr interface {
	// String returns the node as an expression, in the canonical form
	// of Filter.String.
	String() string
	base() *filterNode
}

// filterNode is a node of src's tree. src is nil in a node that no parse
// produced, such as a zero &FilterTerm{}.
type filterNode struct {
	src *filter.Filter
	n   filter.Node
}

// The base methods return a node's filterNode, nil for a nil pointer.

func (e *FilterAnd) base() *filterNode {
	if e == nil {
		return nil
	}
	return &e.filterNode
}

func (e *FilterOr) base() *filterNode {
	if e == nil {
		return nil
	}
	return &e.filterNode
}

func (e *FilterNot) base() *filterNode {
	if e == nil {
		return nil
	}
	return &e.filterNode
}

func (t *FilterTerm) base() *filterNode {
	if t == nil {
		return nil
	}
	return &t.filterNode
}

func newExpr(src *filter.Filter, n filter.Node) FilterExpr {
	b := filterNode{src, n}
	children := func() []FilterExpr {
		var out []FilterExpr
		for _, c := range n.Children() {
			out = append(out, newExpr(src, c))
		}
		return out
	}
	switch n.Kind() {
	case filter.TermKind:
		return &FilterTerm{b}
	case filter.AndKind:
		return &FilterAnd{b, children()}
	case filter.OrKind:
		return &FilterOr{b, children()}
	case filter.NotKind:
		return &FilterNot{b, children()[0]}
	}
	return nil
}

// FilterAnd is a conjunction, a AND b or a b: it selects a record when
// every operand does, evaluating them left to right.
type FilterAnd struct {
	filterNode
	ops []FilterExpr
}

// Operands returns the two or more operands, in order. Nested ANDs are
// flattened: a AND (b AND c) has three operands.
func (e *FilterAnd) Operands() []FilterExpr { return slices.Clone(e.ops) }

// String returns the conjunction as an expression (see Filter.String).
func (e *FilterAnd) String() string { return e.n.String() }

// FilterOr is a disjunction, a OR b: it selects a record when any operand
// does, evaluating them left to right.
type FilterOr struct {
	filterNode
	ops []FilterExpr
}

// Operands returns the two or more operands, in order. Nested ORs are
// flattened.
func (e *FilterOr) Operands() []FilterExpr { return slices.Clone(e.ops) }

// String returns the disjunction as an expression (see Filter.String).
func (e *FilterOr) String() string { return e.n.String() }

// FilterNot is a negation, NOT a or -term.
type FilterNot struct {
	filterNode
	op FilterExpr
}

// Operand returns the negated node.
func (e *FilterNot) Operand() FilterExpr { return e.op }

// String returns the negation as an expression (see Filter.String).
func (e *FilterNot) String() string { return e.n.String() }

// FilterTerm is a term, key operator operand, such as labels.env=prod or
// name:(web db).
type FilterTerm struct{ filterNode }

// Key returns the term's key.
func (t *FilterTerm) Key() Key { return Key{t.n.Key()} }

// Op returns the term's operator.
func (t *FilterTerm) Op() FilterOp { return FilterOp(t.n.Op()) }

// Operands returns the term's operands as text, without quotes: one, or
// the elements of a list, key:(a b) or key=(a,b). The term matches when
// one of them does (see the package documentation for how each operator
// matches).
func (t *FilterTerm) Operands() []string { return t.n.Operands() }

// IsList reports whether the operands were written as a list, even of one
// element: key:(a) rather than key:a.
func (t *FilterTerm) IsList() bool { return t.n.IsList() }

// String returns the term as an expression (see Filter.String).
func (t *FilterTerm) String() string { return t.n.String() }

// FilterOp is the operator of a FilterTerm.
type FilterOp string

// The operators of filter terms.
const (
	FilterHas          FilterOp = ":"  // key:pattern, the value contains the pattern
	FilterEqual        FilterOp = "="  // key=value
	FilterNotEqual     FilterOp = "!=" // key!=value
	FilterLess         FilterOp = "<"  // key<value
	FilterLessEqual    FilterOp = "<=" // key<=value
	FilterGreater      FilterOp = ">"  // key>value
	FilterGreaterEqual FilterOp = ">=" // key>=value
)

// Key is a resource key of a filter term or a sort key, such as
// labels.env, disks[0].name or tags.items[]: names, list indices and []
// slices, applied left to right to a record's --format json output.
type Key struct{ k resource.Key }

// String returns the key as an expression writes it: names joined with
// dots, [N] and [], names quoted where needed ("k.e.y".value), and "."
// for the empty key, the record itself.
func (k Key) String() string { return k.k.Expr() }

// Parts returns the parts of the key, in order.
func (k Key) Parts() []KeyPart {
	out := make([]KeyPart, len(k.k))
	for i, p := range k.k {
		out[i] = KeyPart{Name: p.Name, Slice: p.Slice}
		if p.Index != nil {
			out[i].Index = new(big.Int).Set(p.Index)
		}
	}
	return out
}

// KeyPart is one part of a Key: exactly one of a name, an index or a
// slice.
type KeyPart struct {
	// Name is a name, such as "labels" (empty for an index or a slice).
	Name string
	// Index is [N], the element of a list at N, negative from the end
	// (nil for a name or a slice). Keys are read as Python's int() reads
	// N, so it may be large.
	Index *big.Int
	// Slice is [], every element of a list.
	Slice bool
}
