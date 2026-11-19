package format

import (
	"bytes"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/filter"
	"github.com/SCKelemen/clix/v2/internal/resource"
)

// ListFlags adds the Google Cloud CLI's list flags to cmd, a command whose
// result is a list:
//
//   - --filter EXPRESSION keeps the elements the expression selects, in
//     a strict subset of gcloud's filter syntax (see the package
//     documentation and FiltersTopic): key=value, key:value, !=, <, <=,
//     >, >=, AND, OR, NOT, parentheses, quoting, lists and globs.
//   - --sort-by FIELD[,FIELD...] sorts them by those keys (dotted paths
//     such as owner.login); ~FIELD sorts by FIELD in descending order.
//   - --limit N prints at most N of them; "unlimited" is the default.
//
// Write (and WriteWith) apply them in the order gcloud does: filter, then
// sort, then limit, then the output flags (--format, --json, --jq,
// --template) on what is left. Commands that print with clix.FormatData
// themselves ignore them.
//
// The flags are read from the command line only (they depend on the
// command) and are Ambient: they choose what is printed, so "myapp list
// --limit 5" still prompts for missing required flags when the app is
// interactive. An expression, field list or limit that does not parse is
// a usage error before the command runs; a filter that clix cannot
// evaluate on the data as gcloud would (see the package documentation) is
// a usage error from Write, before anything is printed. A flag that cmd
// already has keeps its meaning: ListFlags adds only the others, and Write
// ignores the flags it did not add. Calling ListFlags again does nothing.
//
// Example:
//
//	list := clix.NewCommand("list", format.WithListFlags(),
//		clix.WithCommandRun(func(ctx *clix.Context) error {
//			return format.Write(ctx, projects)
//		}))
func ListFlags(cmd *clix.Command) {
	if cmd == nil {
		return
	}
	if cmd.Flags == nil {
		cmd.Flags = clix.NewFlagSet(cmd.Name)
	}
	fs := cmd.Flags
	if fs.Lookup("filter") == nil {
		fs.Var(clix.VarOptions{
			FlagOptions: clix.FlagOptions{
				Name:            "filter",
				Usage:           `Only list the items the expression selects, such as "name:web AND status=RUNNING"`,
				Validate:        validateFilter,
				Ambient:         true,
				CommandLineOnly: true,
			},
			Value: &filterValue{},
		})
		fs.SetCompletion("filter", noCompletion)
	}
	if fs.Lookup("sort-by") == nil {
		fs.Var(clix.VarOptions{
			FlagOptions: clix.FlagOptions{
				Name:            "sort-by",
				Usage:           "Sort by these comma-separated fields; ~FIELD sorts in descending order",
				Validate:        validateSortBy,
				Ambient:         true,
				CommandLineOnly: true,
			},
			Value: &sortByValue{},
		})
		fs.SetCompletion("sort-by", noCompletion)
	}
	if fs.Lookup("limit") == nil {
		fs.Var(clix.VarOptions{
			FlagOptions: clix.FlagOptions{
				Name:            "limit",
				Usage:           `List at most this many items ("unlimited" for all)`,
				Validate:        validateLimit,
				Ambient:         true,
				CommandLineOnly: true,
			},
			Value: &limitValue{},
		})
		fs.SetCompletion("limit", clix.CompleteValues("unlimited"))
	}
}

// WithListFlags is ListFlags as an option of clix.NewCommand.
func WithListFlags() clix.CommandOption { return listFlagsOption{} }

type listFlagsOption struct{}

func (listFlagsOption) ApplyCommand(cmd *clix.Command) { ListFlags(cmd) }

// filterValue holds --filter. Its type marks the flag as ListFlags'.
type filterValue struct {
	text string
	set  bool
}

func (v *filterValue) Set(s string) error {
	v.text, v.set = s, true
	return nil
}
func (v *filterValue) String() string { return v.text }
func (v *filterValue) Reset()         { *v = filterValue{} }

// sortByValue holds --sort-by.
type sortByValue struct{ text string }

func (v *sortByValue) Set(s string) error {
	v.text = s
	return nil
}
func (v *sortByValue) String() string { return v.text }
func (v *sortByValue) Reset()         { v.text = "" }

// limitValue holds --limit: 0 means unlimited.
type limitValue struct {
	text string
	n    int64
}

func (v *limitValue) Set(s string) error {
	n, err := parseLimit(s)
	if err != nil {
		return err
	}
	v.text, v.n = s, n
	return nil
}
func (v *limitValue) String() string { return v.text }
func (v *limitValue) Reset()         { *v = limitValue{} }

func validateFilter(value string) error {
	_, err := filter.Parse(value)
	return err
}

func validateSortBy(value string) error {
	_, err := resource.ParseSortBy(value)
	return err
}

func validateLimit(value string) error {
	_, err := parseLimit(value)
	return err
}

// parseLimit reads a --limit: "unlimited" (0) or a decimal integer of at
// least 1, as gcloud's BoundedInt(1, sys.maxsize, unlimited=True) takes.
func parseLimit(s string) (int64, error) {
	if s == "unlimited" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%q is not a positive integer or unlimited", s)
	}
	return n, nil
}

// unsupportedIn marks an *resource.UnsupportedError as one of flag.
func unsupportedIn(err error, flag string) error {
	var u *resource.UnsupportedError
	if errors.As(err, &u) && u.In == "" {
		u.In = flag
	}
	return err
}

// listSettings are the list flags of one Run.
type listSettings struct {
	filter *filter.Filter
	sortBy []resource.SortKey
	limit  int64
	names  []string // the flags given, for messages
}

func (s listSettings) active() bool { return len(s.names) > 0 }

// readListSettings reads the list flags ListFlags added to the running
// command.
func readListSettings(ctx *clix.Context) (listSettings, error) {
	var s listSettings
	if ctx.Command == nil || ctx.Command.Flags == nil {
		return s, nil
	}
	fs := ctx.Command.Flags
	if f := fs.Lookup("filter"); f != nil {
		if v, ok := f.Value.(*filterValue); ok && v.set && strings.TrimSpace(v.text) != "" {
			flt, err := filter.Parse(v.text)
			if err != nil {
				return s, clix.UsageErrorf("invalid value for --filter: %w", err)
			}
			s.filter = flt
			s.names = append(s.names, "--filter")
		}
	}
	if f := fs.Lookup("sort-by"); f != nil {
		if v, ok := f.Value.(*sortByValue); ok && v.text != "" {
			keys, err := resource.ParseSortBy(v.text)
			if err != nil {
				return s, clix.UsageErrorf("invalid value for --sort-by: %w", err)
			}
			s.sortBy = keys
			s.names = append(s.names, "--sort-by")
		}
	}
	if f := fs.Lookup("limit"); f != nil {
		if v, ok := f.Value.(*limitValue); ok && v.n > 0 {
			s.limit = v.n
			s.names = append(s.names, "--limit")
		}
	}
	return s, nil
}

// applyList filters, sorts and limits data, a list, as the list flags
// ask, and returns what is left: a slice of the same element type when
// data is a slice or an array (so every output format prints the
// elements exactly as it would print them in data), and otherwise the
// elements' JSON.
func applyList(data any, s listSettings) (any, error) {
	var raw bytes.Buffer
	if err := clix.FormatData(&raw, data, clix.FormatJSON); err != nil {
		return nil, err
	}
	doc, err := resource.Decode(raw.Bytes())
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return data, nil // nothing to list
	}
	list, ok := doc.([]any)
	if !ok {
		return nil, clix.UsageErrorf("%s apply to a list, and this command prints a single %s", strings.Join(s.names, ", "), kindOf(doc))
	}
	order := make([]int, len(list))
	for i := range order {
		order[i] = i
	}
	if s.filter != nil {
		if order, err = s.filter.Select(list); err != nil {
			return nil, clix.UsageErrorf("--filter: %w", err)
		}
	}
	if len(s.sortBy) > 0 {
		selected := make([]any, len(order))
		for i, j := range order {
			selected[i] = list[j]
		}
		sorted, err := resource.SortBy(selected, s.sortBy)
		if err != nil {
			return nil, clix.UsageErrorf("--sort-by: %w", unsupportedIn(err, "--sort-by"))
		}
		reordered := make([]int, len(sorted))
		for i, k := range sorted {
			reordered[i] = order[k]
		}
		order = reordered
	}
	if s.limit > 0 && int64(len(order)) > s.limit {
		order = order[:s.limit]
	}
	return subset(data, raw.Bytes(), len(list), order)
}

func kindOf(v any) string {
	switch v.(type) {
	case *resource.Object:
		return "object"
	case string:
		return "string"
	}
	return "value"
}

// subset returns the elements of data at the indices in order. n is the
// length of the JSON list data encodes as.
func subset(data any, raw []byte, n int, order []int) (any, error) {
	rv := reflect.ValueOf(data)
	for rv.IsValid() && (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) && !rv.IsNil() && !marshalsItself(rv.Type()) {
		rv = rv.Elem()
	}
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) && !marshalsItself(rv.Type()) && rv.Len() == n {
		out := reflect.MakeSlice(reflect.SliceOf(rv.Type().Elem()), 0, len(order))
		for _, i := range order {
			out = reflect.Append(out, rv.Index(i))
		}
		return out.Interface(), nil
	}
	// data encodes itself as a list: keep the JSON of the elements.
	dec := jsontext.NewDecoder(bytes.NewReader(raw), decodeOptions)
	if _, err := dec.ReadToken(); err != nil {
		return nil, err
	}
	elems := make([]jsonv1.RawMessage, 0, n)
	for dec.PeekKind() != ']' {
		v, err := dec.ReadValue()
		if err != nil {
			return nil, err
		}
		elems = append(elems, jsonv1.RawMessage(bytes.Clone(v)))
	}
	out := make([]jsonv1.RawMessage, 0, len(order))
	for _, i := range order {
		out = append(out, elems[i])
	}
	return out, nil
}
