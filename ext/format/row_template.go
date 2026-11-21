package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
}

func parseRowTemplate(source string) (*rowTemplate, error) {
	body, table := strings.CutPrefix(source, "table ")
	t, err := template.New("row").Option("missingkey=error").Funcs(template.FuncMap{
		"json": rowJSON, "join": rowJoin, "split": strings.Split,
		"lower": strings.ToLower, "upper": strings.ToUpper,
	}).Parse(body)
	if err != nil {
		return nil, err
	}
	for _, definition := range t.Templates() {
		expandRowText(definition.Root)
	}
	rows := &rowTemplate{template: t, table: table}
	if table {
		rows.header, err = rowHeader(t)
	}
	return rows, err
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
