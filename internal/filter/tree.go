package filter

import (
	"strings"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

// Kind is the kind of a Node.
type Kind int

const (
	// NoKind is the zero Node: no expression, which selects everything.
	NoKind Kind = iota
	// TermKind is a key operator operand term.
	TermKind
	// AndKind is the conjunction of two or more nodes (AND, or
	// juxtaposition).
	AndKind
	// OrKind is the disjunction of two or more nodes.
	OrKind
	// NotKind is the negation of one node (NOT, or - before a term).
	NotKind
)

// Node is a node of a parsed expression, read only. The zero Node is the
// empty expression.
type Node struct{ n node }

// Root returns the root of f's expression, the zero Node when f selects
// everything.
func (f *Filter) Root() Node { return Node{f.root} }

// Kind returns the kind of n.
func (n Node) Kind() Kind {
	switch n.n.(type) {
	case *termNode:
		return TermKind
	case andNode:
		return AndKind
	case orNode:
		return OrKind
	case notNode:
		return NotKind
	}
	return NoKind
}

// Children returns the operands of an AND or an OR, flattened, in order
// (a AND b AND c, a b c and a AND (b AND c) have the same three), the one
// operand of a NOT, and nothing for a term.
func (n Node) Children() []Node {
	switch x := n.n.(type) {
	case andNode, orNode:
		return flatten(x, n.Kind(), nil)
	case notNode:
		return []Node{{x.expr}}
	}
	return nil
}

// flatten appends the operands of n, descending into the operands of
// kind, AndKind or OrKind.
func flatten(n node, kind Kind, out []Node) []Node {
	switch x := n.(type) {
	case andNode:
		if kind == AndKind {
			return flatten(x.right, kind, flatten(x.left, kind, out))
		}
	case orNode:
		if kind == OrKind {
			return flatten(x.right, kind, flatten(x.left, kind, out))
		}
	}
	return append(out, Node{n})
}

// term returns n's term, or nil.
func (n Node) term() *termNode {
	t, _ := n.n.(*termNode)
	return t
}

// Key returns a term's key (nil for other nodes). The caller must not
// modify it.
func (n Node) Key() resource.Key {
	if t := n.term(); t != nil {
		return t.key
	}
	return nil
}

// Op returns a term's operator: ":", "=", "!=", "<", "<=", ">" or ">="
// ("" for other nodes).
func (n Node) Op() string {
	if t := n.term(); t != nil {
		return t.op.String()
	}
	return ""
}

// Operands returns a term's operands, as text without quotes: one, or
// the elements of a list (key:(a b)).
func (n Node) Operands() []string {
	t := n.term()
	if t == nil {
		return nil
	}
	out := make([]string, len(t.operands))
	for i, o := range t.operands {
		out[i] = o.text
	}
	return out
}

// IsList reports whether a term's operands were written as a list,
// key:(a b) or key=(a,b), even of one element.
func (n Node) IsList() bool {
	t := n.term()
	return t != nil && t.list
}

// Sub returns a filter that selects what all of nodes select, evaluated
// left to right as gcloud evaluates a AND b (everything, for no nodes),
// with f's options. Zero nodes are left out.
func (f *Filter) Sub(nodes ...Node) *Filter {
	var root node
	for i := len(nodes) - 1; i >= 0; i-- {
		switch {
		case nodes[i].n == nil:
		case root == nil:
			root = nodes[i].n
		default:
			root = andNode{nodes[i].n, root}
		}
	}
	return &Filter{root: root, loc: f.loc, now: f.now}
}

// Selector evaluates a filter on a list that comes in parts, such as the
// pages of a listing: Select on each part in turn selects what one Select
// of the whole list would, part by part. It is not safe for concurrent
// use.
type Selector struct{ ev *evaluator }

// Selector returns a new Selector for f.
func (f *Filter) Selector() *Selector {
	return &Selector{ev: &evaluator{f: f, states: map[*termNode]*termState{}}}
}

// Select returns the indices of the resources the filter selects, in
// order, continuing the evaluation state of earlier calls (see
// Filter.Select).
func (s *Selector) Select(resources []any) ([]int, error) {
	var out []int
	for i, r := range resources {
		ok, err := s.ev.eval(s.ev.f.root, r)
		if err != nil {
			return nil, unsupportedIn(err)
		}
		if ok {
			out = append(out, i)
		}
	}
	return out, nil
}

// String returns f's expression in a canonical form that Parse reads back
// as the same tree: AND and OR written out, grouped with parentheses
// where needed, NOT for - before a term, and keys and operands quoted
// where needed. It is "" when f selects everything.
func (f *Filter) String() string { return f.Root().String() }

// String returns n as an expression in the canonical form of
// Filter.String.
func (n Node) String() string {
	var b strings.Builder
	writeNode(&b, n.n)
	return b.String()
}

func writeNode(b *strings.Builder, n node) {
	switch x := n.(type) {
	case *termNode:
		writeTerm(b, x)
	case andNode:
		writeJoined(b, Node{x}.Children(), " AND ", OrKind)
	case orNode:
		writeJoined(b, Node{x}.Children(), " OR ", AndKind)
	case notNode:
		b.WriteString("NOT ")
		// NOT NOT is a syntax error, and NOT binds to one term.
		writeGrouped(b, x.expr, x.expr != nil && (Node{x.expr}).Kind() != TermKind)
	}
}

// writeJoined writes nodes separated by sep, in parentheses those of kind
// group: gcloud requires them when AND and OR are combined.
func writeJoined(b *strings.Builder, nodes []Node, sep string, group Kind) {
	for i, c := range nodes {
		if i > 0 {
			b.WriteString(sep)
		}
		writeGrouped(b, c.n, c.Kind() == group)
	}
}

func writeGrouped(b *strings.Builder, n node, group bool) {
	if group {
		b.WriteByte('(')
	}
	writeNode(b, n)
	if group {
		b.WriteByte(')')
	}
}

// operatorChars start an operator; one of them right after an operator
// would be read as part of it.
const operatorChars = ":=!<>~"

func writeTerm(b *strings.Builder, t *termNode) {
	b.WriteString(t.key.Expr())
	b.WriteString(t.op.String())
	if t.list {
		b.WriteByte('(')
		for i, o := range t.operands {
			if i > 0 {
				b.WriteByte(',')
			}
			// Args reads the elements up to a separator or ")", with
			// parentheses balanced.
			b.WriteString(resource.QuoteToken(o.text, " \t\n,()"))
		}
		b.WriteByte(')')
		return
	}
	text := t.operands[0].text
	q := resource.QuoteToken(text, "(),")
	if q == text && (contains(logical, text) || strings.ContainsAny(text[:1], operatorChars)) {
		// AND, OR and NOT are logical operators where an operand is
		// expected, and an operator character would extend the operator.
		q = `"` + text + `"`
	}
	b.WriteString(q)
}
