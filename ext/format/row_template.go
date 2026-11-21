package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"text/template"
	"text/template/parse"

	"github.com/SCKelemen/clix/v2"
)

type rowTemplate struct {
	template *template.Template
	header   string
	table    bool
	missing  Missing
	fields   []string // the record fields the template reads (MissingError only; see rowFields)
}

func parseRowTemplate(source string, missing Missing) (*rowTemplate, error) {
	body, table := strings.CutPrefix(source, "table ")
	funcs := template.FuncMap{
		"json": rowJSON, "join": rowJoin, "split": strings.Split,
		"lower": strings.ToLower, "upper": strings.ToUpper,
	}
	option := "missingkey=error"
	if missing == MissingZero {
		option = "missingkey=default"
		funcs["split"], funcs["lower"], funcs["upper"] = zeroSplit, zeroString(strings.ToLower), zeroString(strings.ToUpper)
		funcs[zeroFieldFunc], funcs[zeroTextFunc] = zeroField, zeroText
	}
	t, err := template.New("row").Option(option).Funcs(funcs).Parse(body)
	if err != nil {
		return nil, err
	}
	for _, definition := range t.Templates() {
		expandRowText(definition.Root)
	}
	rows := &rowTemplate{template: t, table: table, missing: missing}
	if table {
		if rows.header, err = rowHeader(t); err != nil {
			return nil, err
		}
	}
	if missing == MissingZero {
		for _, definition := range t.Templates() {
			zeroList(definition.Root)
		}
	} else {
		rows.fields = rowFields(t.Root, true, nil)
	}
	return rows, nil
}

var rowTextEscapes = strings.NewReplacer(`\t`, "\t", `\n`, "\n")

// Expand separators in literal output. Quoted arguments use Go template syntax.
func expandRowText(list *parse.ListNode) {
	if list == nil {
		return
	}
	for _, node := range list.Nodes {
		var branch *parse.BranchNode
		switch n := node.(type) {
		case *parse.TextNode:
			n.Text = []byte(rowTextEscapes.Replace(string(n.Text)))
		case *parse.IfNode:
			branch = &n.BranchNode
		case *parse.RangeNode:
			branch = &n.BranchNode
		case *parse.WithNode:
			branch = &n.BranchNode
		}
		if branch != nil {
			expandRowText(branch.List)
			expandRowText(branch.ElseList)
		}
	}
}

// A table header comes from the selected field paths, including for an empty list.
func rowHeader(t *template.Template) (string, error) {
	var header strings.Builder
	for _, node := range t.Root.Nodes {
		switch n := node.(type) {
		case *parse.TextNode:
			header.Write(n.Text)
			continue
		case *parse.ActionNode:
			if len(n.Pipe.Decl) == 0 && len(n.Pipe.Cmds) == 1 && len(n.Pipe.Cmds[0].Args) == 1 {
				if f, ok := n.Pipe.Cmds[0].Args[0].(*parse.FieldNode); ok {
					header.WriteString(strings.ToUpper(strings.Join(f.Ident, ".")))
					continue
				}
			}
		}
		return "", fmt.Errorf("table row templates require direct field selectors such as {{.name}}")
	}
	return header.String(), nil
}

func rowJSON(v any) (string, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func rowJoin(v any, separator string) (string, error) {
	switch values := v.(type) {
	case nil:
		return "", nil
	case []string: // split returns a Go string slice.
		return strings.Join(values, separator), nil
	case []any: // A JSON array.
		return joinFunc(separator, values)
	default:
		return "", fmt.Errorf("join requires an array, got %T", v)
	}
}

func (f *rowTemplate) write(out io.Writer, doc []byte, escape bool) error {
	data, err := templateData(doc)
	if err != nil {
		return err
	}
	if escape {
		data = escapeStrings(data)
	}
	items, list := data.([]any)
	if !list && data != nil {
		items = []any{data}
	}

	// Validate every row before writing stdout. A command used in a pipeline
	// should not emit a plausible partial result when a later field is missing.
	var buf bytes.Buffer
	var writer io.Writer = &buf
	var columns *tabwriter.Writer
	if f.table {
		columns = tabwriter.NewWriter(&buf, 0, 0, 3, ' ', 0)
		writer = columns
		if _, err := fmt.Fprintln(writer, f.header); err != nil {
			return err
		}
	}
	for i, item := range items {
		if err := f.template.Execute(writer, item); err != nil {
			return clix.UsageErrorf("--format row %d: %w", i+1, err)
		}
		if _, err := io.WriteString(writer, "\n"); err != nil {
			return err
		}
	}
	if columns != nil {
		if err := columns.Flush(); err != nil {
			return err
		}
	}
	_, err = buf.WriteTo(out)
	return err
}

// checkFields reports a field the template reads from its record that
// is not among available, the fields a schema declares and the keys of
// the records, so that a mistyped name fails even when there are no
// rows. Only references whose record is known before running are
// checked (see rowFields); MissingZero templates are not checked.
func (f *rowTemplate) checkFields(available []string) error {
	if f.missing == MissingZero {
		return nil
	}
	for _, name := range f.fields {
		if !slices.Contains(available, name) {
			sorted := slices.Sorted(slices.Values(available))
			return unknownFieldError("--format", name, sorted)
		}
	}
	return nil
}

// rowFields adds to fields the first name of each field reference in list
// that reads from the record: .name where dot is the record (outside the
// body of range and with, whose dot is something else, and outside
// defined templates), and $.name anywhere.
func rowFields(list *parse.ListNode, dotIsRecord bool, fields []string) []string {
	if list == nil {
		return fields
	}
	for _, node := range list.Nodes {
		switch n := node.(type) {
		case *parse.ActionNode:
			fields = pipeFields(n.Pipe, dotIsRecord, fields)
		case *parse.IfNode:
			fields = pipeFields(n.Pipe, dotIsRecord, fields)
			fields = rowFields(n.List, dotIsRecord, fields)
			fields = rowFields(n.ElseList, dotIsRecord, fields)
		case *parse.RangeNode:
			fields = pipeFields(n.Pipe, dotIsRecord, fields)
			fields = rowFields(n.List, false, fields)
			fields = rowFields(n.ElseList, dotIsRecord, fields)
		case *parse.WithNode:
			fields = pipeFields(n.Pipe, dotIsRecord, fields)
			fields = rowFields(n.List, false, fields)
			fields = rowFields(n.ElseList, dotIsRecord, fields)
		case *parse.TemplateNode:
			fields = pipeFields(n.Pipe, dotIsRecord, fields)
		}
	}
	return fields
}

func pipeFields(pipe *parse.PipeNode, dotIsRecord bool, fields []string) []string {
	if pipe == nil {
		return fields
	}
	add := func(name string) {
		if !slices.Contains(fields, name) {
			fields = append(fields, name)
		}
	}
	for _, cmd := range pipe.Cmds {
		for _, arg := range cmd.Args {
			switch a := arg.(type) {
			case *parse.FieldNode:
				if dotIsRecord {
					add(a.Ident[0])
				}
			case *parse.VariableNode:
				if a.Ident[0] == "$" && len(a.Ident) > 1 {
					add(a.Ident[1])
				}
			case *parse.ChainNode:
				if p, ok := a.Node.(*parse.PipeNode); ok {
					fields = pipeFields(p, dotIsRecord, fields)
				}
			case *parse.PipeNode:
				fields = pipeFields(a, dotIsRecord, fields)
			}
		}
	}
	return fields
}

// The functions a MissingZero template calls in place of field
// references and around the value an action prints.
const (
	zeroFieldFunc = "zeroField"
	zeroTextFunc  = "zeroText"
)

// zeroField returns the field at names below v: nil when a key is
// missing or a value on the way is null, and an error when a value on
// the way is not an object, as text/template reports one.
func zeroField(v any, names ...string) (any, error) {
	for _, name := range names {
		switch x := v.(type) {
		case nil:
			return nil, nil
		case map[string]any:
			v = x[name]
		default:
			return nil, fmt.Errorf("can't evaluate field %s in type %T", name, v)
		}
	}
	return v, nil
}

// zeroText prints nil as nothing, where text/template prints "<no value>".
func zeroText(v any) any {
	if v == nil {
		return ""
	}
	return v
}

// zeroString and zeroSplit are lower, upper and split for MissingZero
// templates, taking nil as the empty string.
func zeroString(f func(string) string) func(any) (string, error) {
	return func(v any) (string, error) {
		switch s := v.(type) {
		case nil:
			return "", nil
		case string:
			return f(s), nil
		}
		return "", fmt.Errorf("wrong type for value; expected string; got %T", v)
	}
}

func zeroSplit(v any, separator string) ([]string, error) {
	switch s := v.(type) {
	case nil:
		return nil, nil
	case string:
		return strings.Split(s, separator), nil
	}
	return nil, fmt.Errorf("wrong type for value; expected string; got %T", v)
}

// zeroList rewrites a MissingZero template: every field reference becomes
// a call of zeroField, so a missing key or a null value on the way gives
// nil instead of an error, and every action that prints passes its value
// through zeroText, so nil prints as nothing.
func zeroList(list *parse.ListNode) {
	if list == nil {
		return
	}
	for _, node := range list.Nodes {
		switch n := node.(type) {
		case *parse.ActionNode:
			zeroPipe(n.Pipe)
			if len(n.Pipe.Decl) == 0 {
				n.Pipe.Cmds = append(n.Pipe.Cmds, &parse.CommandNode{
					NodeType: parse.NodeCommand, Pos: n.Pos,
					Args: []parse.Node{parse.NewIdentifier(zeroTextFunc).SetPos(n.Pos)},
				})
			}
		case *parse.IfNode:
			zeroBranch(&n.BranchNode)
		case *parse.RangeNode:
			zeroBranch(&n.BranchNode)
		case *parse.WithNode:
			zeroBranch(&n.BranchNode)
		case *parse.TemplateNode:
			zeroPipe(n.Pipe)
		}
	}
}

func zeroBranch(b *parse.BranchNode) {
	zeroPipe(b.Pipe)
	zeroList(b.List)
	zeroList(b.ElseList)
}

func zeroPipe(pipe *parse.PipeNode) {
	if pipe == nil {
		return
	}
	for _, cmd := range pipe.Cmds {
		for i, arg := range cmd.Args {
			cmd.Args[i] = zeroArg(arg)
		}
	}
}

// zeroArg returns the call of zeroField that replaces a field reference:
// .a.b is (zeroField . "a" "b"), $x.a is (zeroField $x "a"), and (p).a
// is (zeroField (p) "a").
func zeroArg(arg parse.Node) parse.Node {
	switch a := arg.(type) {
	case *parse.FieldNode:
		return zeroCall(a.Pos, &parse.DotNode{NodeType: parse.NodeDot, Pos: a.Pos}, a.Ident)
	case *parse.VariableNode:
		if len(a.Ident) > 1 {
			root := &parse.VariableNode{NodeType: parse.NodeVariable, Pos: a.Pos, Ident: a.Ident[:1]}
			return zeroCall(a.Pos, root, a.Ident[1:])
		}
	case *parse.ChainNode:
		return zeroCall(a.Pos, zeroArg(a.Node), a.Field)
	case *parse.PipeNode:
		zeroPipe(a)
	}
	return arg
}

func zeroCall(pos parse.Pos, receiver parse.Node, names []string) parse.Node {
	args := []parse.Node{parse.NewIdentifier(zeroFieldFunc).SetPos(pos), receiver}
	for _, name := range names {
		args = append(args, &parse.StringNode{NodeType: parse.NodeString, Pos: pos, Quoted: strconv.Quote(name), Text: name})
	}
	return &parse.PipeNode{NodeType: parse.NodePipe, Pos: pos, Cmds: []*parse.CommandNode{{NodeType: parse.NodeCommand, Pos: pos, Args: args}}}
}
