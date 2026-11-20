package format

import (
	"bytes"
	"fmt"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/internal/filter"
	"github.com/SCKelemen/clix/v2/internal/resource"
)

// SortBy is a parsed --sort-by value: the keys to sort records by, as
// ListFlags' --sort-by sorts them. The zero SortBy keeps the order.
type SortBy struct {
	keys []resource.SortKey
	text string
}

// ParseSortBy parses a --sort-by value exactly as ListFlags' --sort-by
// does, with the same errors: comma-separated keys, each ascending unless
// it starts with ~, where [] means the first element ([0]). An empty
// value sorts nothing.
func ParseSortBy(value string) (SortBy, error) {
	keys, err := resource.ParseSortBy(value)
	if err != nil {
		return SortBy{}, err
	}
	return SortBy{keys: keys, text: value}, nil
}

// String returns the value ParseSortBy parsed.
func (s SortBy) String() string { return s.text }

// Keys returns the keys, in order.
func (s SortBy) Keys() []SortKey {
	out := make([]SortKey, len(s.keys))
	for i, k := range s.keys {
		out[i] = SortKey{Key: Key{k.Key}, Descending: k.Reverse}
	}
	return out
}

// SortKey is one key of a SortBy.
type SortKey struct {
	Key Key
	// Descending is set by ~ before the key.
	Descending bool
}

// Sort returns the indices of records in the order --sort-by sorts them,
// for records as Filter.Select takes them. It sorts as gcloud does, by
// the text of the values (see the package documentation), with a stable
// sort, so records with equal keys keep their order.
func (s SortBy) Sort(records any) ([]int, error) {
	list, err := readRecords(records)
	if err != nil {
		return nil, err
	}
	order, _, err := listOrder(list, nil, s.keys, 0)
	return order, err
}

// ListOptions are ListFlags' --filter, --sort-by and --limit as operations
// on records, for a command that needs them as data rather than in what
// Write prints: to filter the records of each page before deciding
// whether to read another, for example. Write applies the flags with the
// same code, so Select keeps exactly the records Write would print.
type ListOptions struct {
	// Filter keeps the records it selects; nil keeps every record.
	Filter *Filter
	// SortBy sorts them; the zero SortBy keeps their order.
	SortBy SortBy
	// Limit keeps at most this many of them; 0 or less keeps all.
	Limit int
}

// Select returns the indices of the records o keeps, in the order it puts
// them, applying the operations in the order gcloud and Write apply the
// flags: filter, then sort, then limit. records is a list as
// Filter.Select takes it.
func (o ListOptions) Select(records any) ([]int, error) {
	list, err := readRecords(records)
	if err != nil {
		return nil, err
	}
	var flt *filter.Filter
	if o.Filter != nil {
		flt = o.Filter.f
	}
	order, _, err := listOrder(list, flt, o.SortBy.keys, int64(max(o.Limit, 0)))
	return order, err
}

// ApplyList returns the records that o keeps, in the order it puts them
// (see ListOptions.Select): the elements Write would print for records
// with the list flags of o. records is not modified.
func ApplyList[T any](records []T, o ListOptions) ([]T, error) {
	order, err := o.Select(records)
	if err != nil {
		return nil, err
	}
	out := make([]T, len(order))
	for i, j := range order {
		out[i] = records[j]
	}
	return out, nil
}

// decoded is a command's result read as a list of records.
type decoded struct {
	list    []any  // the records
	raw     []byte // the JSON clix.FormatData writes for the result
	null    bool   // the result is null: there is nothing to list
	notList string // what the result is when it is not a list or null, such as "object"
}

// decodeList reads data, a list, as Write and the list flags read it:
// the JSON clix.FormatData writes for it, decoded as gcloud reads JSON
// (see internal/resource.Decode).
func decodeList(data any) (decoded, error) {
	var raw bytes.Buffer
	if err := clix.FormatData(&raw, data, clix.FormatJSON); err != nil {
		return decoded{}, err
	}
	doc, err := resource.Decode(raw.Bytes())
	if err != nil {
		return decoded{}, err
	}
	d := decoded{raw: raw.Bytes()}
	switch v := doc.(type) {
	case nil:
		d.null = true
	case []any:
		d.list = v
	default:
		d.notList = kindOf(doc)
	}
	return d, nil
}

// readRecords returns the records of data for the operations: none for
// null, and an error for a value that is not a list.
func readRecords(data any) ([]any, error) {
	d, err := decodeList(data)
	if err != nil {
		return nil, err
	}
	if d.notList != "" {
		return nil, fmt.Errorf("format: the records are a single %s, not a list", d.notList)
	}
	return d.list, nil
}

// listOrder returns the indices of list that flt, keys and limit keep, in
// order: filter, then sort, then limit, as gcloud applies --filter,
// --sort-by and --limit. A nil flt keeps everything, as do no keys and a
// limit of 0. On an error, failed is the flag whose operation failed.
func listOrder(list []any, flt *filter.Filter, keys []resource.SortKey, limit int64) (order []int, failed string, err error) {
	order = make([]int, len(list))
	for i := range order {
		order[i] = i
	}
	if flt != nil {
		if order, err = flt.Select(list); err != nil {
			return nil, "--filter", err
		}
	}
	if len(keys) > 0 {
		selected := make([]any, len(order))
		for i, j := range order {
			selected[i] = list[j]
		}
		sorted, err := resource.SortBy(selected, keys)
		if err != nil {
			return nil, "--sort-by", unsupportedIn(err, "--sort-by")
		}
		reordered := make([]int, len(sorted))
		for i, k := range sorted {
			reordered[i] = order[k]
		}
		order = reordered
	}
	if limit > 0 && int64(len(order)) > limit {
		order = order[:limit]
	}
	return order, "", nil
}
