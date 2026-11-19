package projection

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

// Names lists the format names Parse accepts, in the order help shows
// them.
var Names = []string{"table", "csv", "value", "flattened", "list", "json", "yaml", "none"}

// Format is a parsed --format expression, NAME[ATTRIBUTES](PROJECTION).
type Format struct {
	// Name is the format name, one of Names.
	Name string
	// Columns are the projection's keys, in order; nil when the
	// expression has no projection.
	Columns []Column
	// HasProjection reports whether the expression has a (...)
	// projection, even an empty one.
	HasProjection bool

	attrs map[string]attrValue
	root  *node // the projection tree (resource_projection_parser._Tree)
}

// Column is one key of a projection.
type Column struct {
	Key        resource.Key
	Label      string // the heading: :label=, or gcloud's ANGRY_SNAKE_CASE default
	Align      string // left, center or right (table only)
	Sort       int    // :sort=N, 0 when the column does not sort (table only)
	Reverse    bool   // :reverse (table only)
	Transforms []transform
}

// attrValue is a format attribute's value: a string, an int, or set
// (a boolean attribute written without "=").
type attrValue struct {
	str    string
	num    int
	isNum  bool
	quoted bool
}

// The projection algorithm's node flags (resource_projection_spec).
const (
	flagDefault = 0
	flagInner   = 1
	flagProject = 2
)

// node is a projection tree node: an attribute and the subtrees by name,
// index or slice.
type node struct {
	flag       int
	transforms []transform // attr.Transforms, used by the projector
	attr       *Column     // the key attribute of a column's last node, nil for inner nodes
	names      map[string]*node
	indices    map[int64]*node
	slice      *node
}

func newNode(flag int) *node {
	return &node{flag: flag, names: map[string]*node{}, indices: map[int64]*node{}}
}

func (n *node) empty() bool { return len(n.names) == 0 && len(n.indices) == 0 && n.slice == nil }

// child returns the subtree for part, or nil.
func (n *node) child(p resource.Part) *node {
	switch {
	case p.Slice:
		return n.slice
	case p.Index != nil:
		return n.indices[p.Index.Int64()]
	}
	return n.names[p.Name]
}

func (n *node) setChild(p resource.Part, c *node) {
	switch {
	case p.Slice:
		n.slice = c
	case p.Index != nil:
		n.indices[p.Index.Int64()] = c
	default:
		n.names[p.Name] = c
	}
}

// formatAttributes lists the attributes each format accepts, and whether
// each takes a value: "bool" (no value), "string" or "int".
var formatAttributes = map[string]map[string]string{
	"table":     {"box": "bool", "all-box": "bool", "no-heading": "bool", "title": "string", "pad": "int"},
	"csv":       {"no-heading": "bool", "no-quote": "bool", "separator": "string", "delimiter": "string", "terminator": "string"},
	"value":     {"quote": "bool", "separator": "string", "delimiter": "string", "terminator": "string"},
	"flattened": {"no-pad": "bool", "separator": "string"},
	"list":      {"compact": "bool", "title": "string", "always-display-title": "bool"},
	"json":      {},
	"yaml":      {},
	"none":      {},
}

// Parse parses a gcloud --format expression, NAME[ATTRIBUTES](PROJECTION),
// as gcloud's resource_projection_parser does, for the subset of formats,
// attributes, key attributes and transforms clix implements (see the
// package documentation). Syntax errors are *resource.SyntaxError values
// and everything outside the subset is a *resource.UnsupportedError.
func Parse(expr string) (*Format, error) {
	l := resource.NewLexer(expr)
	f := &Format{root: newNode(flagDefault)}
	p := &parser{l: l, f: f, headings: map[string]bool{}}
	if !skip(l) {
		return nil, l.Errorf("Name expected")
	}
	if peekAny(l, "[(:") {
		return nil, resource.Unsupportedf("a format expression that does not start with its name (write NAME[ATTRIBUTES](PROJECTION))")
	}
	here := l.Position()
	name, err := l.Token(":([", false, true)
	if err != nil {
		return nil, err
	}
	if !isAlpha(name.Text) {
		return nil, l.ErrorAt(here, "Name expected")
	}
	if _, ok := formatAttributes[name.Text]; !ok {
		return nil, unknownFormat(name.Text)
	}
	f.Name = name.Text
	if c, _ := l.IsCharacter("[", false, true); c != 0 {
		if err := p.attributes(); err != nil {
			return nil, err
		}
		skip(l)
	}
	if c, _ := l.IsCharacter("(", false, true); c != 0 {
		f.HasProjection = true
		f.Columns = []Column{}
		if err := p.keys(); err != nil {
			return nil, err
		}
		skip(l)
	}
	if !l.EndOfInput() {
		if peekAny(l, "[(:") {
			return nil, resource.Unsupportedf("attributes or a projection after the projection, or given twice (write NAME[ATTRIBUTES](PROJECTION))")
		}
		return nil, l.Errorf("Expected NAME[ATTRIBUTES](PROJECTION), unexpected text")
	}
	if f.Name == "none" && f.HasProjection {
		return nil, resource.Unsupportedf("a projection with the none format")
	}
	return f, nil
}

// unknownFormat is the error for a format name clix does not implement.
func unknownFormat(name string) error {
	switch name {
	case "text":
		return resource.Unsupportedf("the text format with attributes or a projection (text is clix's own format here, not gcloud's alias for flattened; use flattened)")
	case "config", "default", "diff", "disable", "get", "multi", "object":
		return resource.Unsupportedf("the %s format (supported: text, %s)", name, strings.Join(Names, ", "))
	}
	return &resource.SyntaxError{Msg: fmt.Sprintf("unknown format %q (supported: text, %s)", name, strings.Join(Names, ", "))}
}

func isAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// peekAny reports whether the next character is one of chars.
func peekAny(l *resource.Lexer, chars string) bool {
	c, _ := l.IsCharacter(chars, true, false)
	return c != 0
}

func skip(l *resource.Lexer) bool {
	ok, _ := l.SkipSpace("", "")
	return ok
}

type parser struct {
	l        *resource.Lexer
	f        *Format
	headings map[string]bool // default labels given so far (_snake_headings)
	labels   []string        // every label so far, which gcloud treats as aliases
}

// attributes parses [no-]name[=value],... up to "]" (_ParseAttributes);
// the "[" has been consumed.
func (p *parser) attributes() error {
	l := p.l
	allowed := formatAttributes[p.f.Name]
	attrs := map[string]attrValue{}
	for {
		name, err := l.Token("=,])", false, false)
		if err != nil {
			return err
		}
		if name.Text != "" {
			kind, ok := allowed[name.Text]
			if !ok {
				return resource.Unsupportedf("the %s attribute %q", p.f.Name, name.Text)
			}
			var v attrValue
			if c, _ := l.IsCharacter("=", false, true); c != 0 {
				vt, err := l.Token(",])", false, false)
				if err != nil {
					return err
				}
				if kind == "bool" {
					return resource.Unsupportedf("a value for the %s attribute", name.Text)
				}
				v, err = attributeValue(vt, kind, name.Text)
				if err != nil {
					return err
				}
			} else if kind != "bool" {
				return resource.Unsupportedf("the %s attribute without a value", name.Text)
			}
			attrs[name.Text] = v
		}
		c, err := l.IsCharacter("]", false, false)
		if err != nil {
			return err
		}
		if c != 0 {
			break
		}
		if c, _ := l.IsCharacter(",", false, true); c == 0 {
			return l.Errorf("Expected ] in attribute list")
		}
	}
	p.f.attrs = attrs
	return nil
}

// attributeValue converts an attribute value as gcloud does: an unquoted
// number is a number, and in a string \n and \t (backslash, letter) are a
// newline and a tab.
func attributeValue(t resource.Token, kind, name string) (attrValue, error) {
	isNum := false
	var n int
	if !t.Quoted && t.Text != "" {
		if i, ok, err := resource.ParseInt(t.Text); err != nil {
			return attrValue{}, err
		} else if ok {
			if !i.IsInt64() || i.Int64() > 1<<20 {
				return attrValue{}, resource.Unsupportedf("the %s value %s", name, t.Text)
			}
			isNum, n = true, int(i.Int64())
		} else if _, ok, err := resource.ParseFloat(t.Text); err != nil {
			return attrValue{}, err
		} else if ok {
			return attrValue{}, resource.Unsupportedf("the number %s as the value of %s", t.Text, name)
		}
	}
	switch kind {
	case "int":
		if !isNum || n < 0 {
			return attrValue{}, resource.Unsupportedf("%s=%s (it must be a whole number of 0 or more)", name, t.Text)
		}
		return attrValue{num: n, isNum: true}, nil
	case "string":
		if isNum {
			return attrValue{}, resource.Unsupportedf("the number %s as the value of %s (gcloud fails on it; quote it)", t.Text, name)
		}
		s := strings.ReplaceAll(t.Text, `\n`, "\n")
		s = strings.ReplaceAll(s, `\t`, "\t")
		return attrValue{str: s, quoted: t.Quoted}, nil
	}
	return attrValue{}, nil
}

// keys parses the comma-separated keys up to ")" (_ParseKeys); the "("
// has been consumed.
func (p *parser) keys() error {
	l := p.l
	if c, _ := l.IsCharacter(")", false, true); c != 0 {
		return nil
	}
	for {
		if err := p.key(); err != nil {
			return err
		}
		skip(l)
		c, err := l.IsCharacter(")", false, false)
		if err != nil {
			return err
		}
		if c != 0 {
			return nil
		}
		if c, _ := l.IsCharacter(",", false, true); c == 0 {
			return l.Errorf("Expected ) in projection expression")
		}
	}
}

// key parses one key with its transforms and attributes (_ParseKey) and
// adds it (_AddKey).
func (p *parser) key() error {
	l := p.l
	here := l.Position()
	key, err := l.Key()
	if err != nil {
		return err
	}
	if len(key) > 0 && key[0].IsName() && slices.Contains(p.labels, key[0].Name) {
		return resource.Unsupportedf("the key %q, which is also the label of an earlier column (gcloud reads it as that column)", key[0].Name)
	}
	col := Column{Align: "left"}
	if c, _ := l.IsCharacter("(", false, true); c != 0 {
		if len(key) == 0 || !key[len(key)-1].IsName() {
			return l.ErrorAt(here, "Transform function expected")
		}
		fn := key[len(key)-1].Name
		key = key[:len(key)-1]
		col.Transforms, err = p.transforms(fn)
		if err != nil {
			return err
		}
	}
	if len(key) == 0 {
		return resource.Unsupportedf("a projection key that is empty or only a transform")
	}
	col.Key = key
	var set attrSet
	skip(l)
	if c, _ := l.IsCharacter(":", false, true); c != 0 {
		if set, err = p.keyAttributes(&col); err != nil {
			return err
		}
	}
	return p.addKey(col, set)
}

// addKey adds a parsed column (_AddKey). Its attributes start from the
// tree node's when the key is already there: a copy for a key that an
// earlier column has (that column keeps its own), the node's own for an
// inner node, and a deep copy of the slice's for an index after a []
// slice of the same list (gcloud's copy.deepcopy(tree[None])). Then what
// the key gives is applied: a label, an alignment other than left, a
// sort order, reverse and transforms replace what was there.
func (p *parser) addKey(col Column, set attrSet) error {
	key := col.Key
	n := p.f.root
	for _, part := range key[:len(key)-1] {
		c := n.child(part)
		if c == nil {
			c = newNode(flagInner)
			n.setChild(part, c)
		} else if c.flag != flagProject {
			c.flag = flagInner
		}
		n = c
	}
	last := key[len(key)-1]
	c := n.child(last)
	dup := false
	for _, prev := range p.f.Columns {
		dup = dup || keysEqual(prev.Key, key)
	}
	switch {
	case c != nil:
	case last.Index != nil && n.slice != nil:
		c = cloneNode(n.slice)
		n.setChild(last, c)
	default:
		c = newNode(flagProject)
		n.setChild(last, c)
	}
	base := Column{Align: "left"}
	labelSet := false
	if c.attr != nil {
		base = *c.attr
		labelSet = true
	}
	base.Key = key
	if set.label {
		base.Label, labelSet = col.Label, true
	}
	if set.align && col.Align != "left" {
		base.Align = col.Align
	}
	if set.sort {
		base.Sort = col.Sort
	}
	if set.reverse {
		base.Reverse = col.Reverse
	}
	if len(col.Transforms) > 0 {
		base.Transforms = col.Transforms
	}
	if !labelSet {
		label, err := p.angrySnakeCase(key)
		if err != nil {
			return err
		}
		base.Label = label
	}
	if !dup {
		a := base
		c.attr = &a
		c.transforms = a.Transforms
		c.flag = flagProject
	}
	p.labels = append(p.labels, base.Label)
	p.f.Columns = append(p.f.Columns, base)
	return nil
}

// attrSet records which key attributes were given.
type attrSet struct{ label, align, sort, reverse bool }

func keysEqual(a, b resource.Key) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x.Slice != y.Slice || x.Name != y.Name || (x.Index == nil) != (y.Index == nil) || (x.Index != nil && x.Index.Cmp(y.Index) != 0) {
			return false
		}
	}
	return true
}

// keyAttributes parses :name[=value]... (_ParseKeyAttributes) into col;
// the first ":" has been consumed. It reports which were given.
func (p *parser) keyAttributes(col *Column) (set attrSet, err error) {
	l := p.l
	for {
		name, err := l.Token("=:,)", false, false)
		if err != nil {
			return set, err
		}
		here := l.Position()
		var value resource.Token
		hasValue := false
		if c, _ := l.IsCharacter("=", false, true); c != 0 {
			hasValue = true
			value, err = l.Token(":,)", false, false)
			if err != nil {
				return set, err
			}
		}
		attr := name.Text
		boolValue := true
		if !hasValue && strings.HasPrefix(attr, "no-") {
			attr, boolValue = attr[3:], false
		}
		switch attr {
		case "label":
			if !hasValue {
				return set, l.ErrorAt(here, "value expected")
			}
			if !value.Quoted && value.Text != "" {
				if _, ok, _ := resource.ParseFloat(value.Text); ok {
					return set, resource.Unsupportedf("the number %s as a label (quote it)", value.Text)
				}
			}
			col.Label, set.label = value.Text, true
		case "align":
			if !hasValue {
				return set, l.ErrorAt(here, "value expected")
			}
			if p.f.Name != "table" {
				return set, resource.Unsupportedf("the align key attribute with the %s format", p.f.Name)
			}
			switch value.Text {
			case "left", "center", "right":
				col.Align, set.align = value.Text, true
			default:
				return set, l.ErrorAt(here, "Unknown alignment")
			}
		case "sort":
			if !hasValue {
				return set, l.ErrorAt(here, "value expected")
			}
			if p.f.Name != "table" {
				return set, resource.Unsupportedf("the sort key attribute with the %s format", p.f.Name)
			}
			n, ok, err := resource.ParseInt(value.Text)
			if err != nil {
				return set, err
			}
			if !ok || value.Quoted || n.Sign() <= 0 || !n.IsInt64() || n.Int64() > 1<<20 {
				return set, resource.Unsupportedf("sort=%s (it must be a whole number of 1 or more)", value.Text)
			}
			col.Sort, set.sort = int(n.Int64()), true
		case "reverse":
			if hasValue {
				return set, l.ErrorAt(here, "value not expected")
			}
			if p.f.Name != "table" {
				return set, resource.Unsupportedf("the reverse key attribute with the %s format", p.f.Name)
			}
			col.Reverse, set.reverse = boolValue, true
		default:
			return set, resource.Unsupportedf("the key attribute %q", name.Text)
		}
		if c, _ := l.IsCharacter(":", false, true); c == 0 {
			return set, nil
		}
	}
}

// angrySnakeCase is the default label of a key (_AngrySnakeCase): for
// the names A, B, C of the key, the first of C, B_C, A_B_C (each name in
// ANGRY_SNAKE_CASE) that no earlier default label used, or A_B_C.
func (p *parser) angrySnakeCase(key resource.Key) (string, error) {
	label := ""
	for i := len(key) - 1; i >= 0; i-- {
		if !key[i].IsName() {
			continue
		}
		s, err := angrySnake(key[i].Name)
		if err != nil {
			return "", err
		}
		if label != "" {
			label = s + "_" + label
		} else {
			label = s
		}
		if !p.headings[label] {
			p.headings[label] = true
			break
		}
	}
	return label, nil
}

// angrySnake is re.sub('((?<=[a-z0-9])[A-Z]+|(?!^)[A-Z](?=[a-z]))',
// r'_\1', name).upper(): an underscore before a run of capitals that
// follows a lower-case letter or digit, and before a capital that starts
// a capitalized word, then upper case.
func angrySnake(name string) (string, error) {
	s := name
	isUpper := func(i int) bool { return i < len(s) && s[i] >= 'A' && s[i] <= 'Z' }
	isLower := func(i int) bool { return i < len(s) && s[i] >= 'a' && s[i] <= 'z' }
	var b strings.Builder
	for i := 0; i < len(s); {
		if i > 0 && isUpper(i) && (isLower(i-1) || (s[i-1] >= '0' && s[i-1] <= '9')) {
			j := i
			for isUpper(j) {
				j++
			}
			b.WriteByte('_')
			b.WriteString(s[i:j])
			i = j
			continue
		}
		if i > 0 && isUpper(i) && isLower(i+1) {
			b.WriteByte('_')
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return pyUpper(b.String())
}

// pyUpper is Python's str.upper() where it agrees with Go's: characters
// whose Python upper case is longer (ß is SS) are an
// *resource.UnsupportedError.
func pyUpper(s string) (string, error) {
	for _, r := range s {
		if (unicode.Is(unicode.Ll, r) && unicode.ToUpper(r) == r && unicode.ToTitle(r) == r) || (r >= 0x1f80 && r <= 0x1fff) || r == 0x149 || r == 0x1f0 {
			return "", resource.Unsupportedf("the default label of a key with %q (give it a :label=)", string(r))
		}
	}
	return strings.ToUpper(s), nil
}

func cloneNode(n *node) *node {
	c := newNode(n.flag)
	c.transforms = n.transforms
	if n.attr != nil {
		a := *n.attr
		c.attr = &a
	}
	for k, v := range n.names {
		c.names[k] = cloneNode(v)
	}
	for k, v := range n.indices {
		c.indices[k] = cloneNode(v)
	}
	if n.slice != nil {
		c.slice = cloneNode(n.slice)
	}
	return c
}

// WithColumns returns f with a projection of keys, as if they had been
// written in its (...), when f has no projection; clix uses it for the
// columns of table, csv and value without one, where gcloud uses the
// command's default projection. f is returned as it is when it has a
// projection.
func (f *Format) WithColumns(keys []resource.Key) (*Format, error) {
	if f.HasProjection {
		return f, nil
	}
	g := &Format{Name: f.Name, attrs: f.attrs, root: newNode(flagDefault), HasProjection: true, Columns: []Column{}}
	p := &parser{f: g, headings: map[string]bool{}}
	for _, key := range keys {
		if len(key) == 0 {
			continue
		}
		dup := false
		for _, c := range g.Columns {
			dup = dup || keysEqual(c.Key, key)
		}
		if dup {
			continue
		}
		if err := p.addKey(Column{Key: key, Align: "left"}, attrSet{}); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// NeedsColumns reports whether the format prints columns that a missing
// projection leaves undefined (table, csv and value).
func (f *Format) NeedsColumns() bool {
	switch f.Name {
	case "table", "csv", "value":
		return !f.HasProjection
	}
	return false
}
