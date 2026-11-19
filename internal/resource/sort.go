package resource

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// SortKey is one --sort-by field: a key and whether it sorts descending.
type SortKey struct {
	Key     Key
	Reverse bool
	Text    string // the field as written, for messages
}

// ParseSortBy parses a --sort-by value as gcloud's display.py does: a
// comma-separated list of resource keys, each in ascending order unless
// prefixed with "~" (any number of them), where a [] slice means the
// first element ([0]). An empty value sorts nothing. Unlike gcloud, which
// sorts by the whole resource for an empty field and ignores whatever
// follows a key, both are errors.
func ParseSortBy(value string) ([]SortKey, error) {
	if value == "" {
		return nil, nil
	}
	var keys []SortKey
	for _, text := range strings.Split(value, ",") {
		name := strings.TrimLeft(text, "~")
		reverse := name != text
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("empty field in %q", value)
		}
		name = strings.ReplaceAll(name, "[]", "[0]")
		key, err := ParseKey(name)
		if err != nil {
			var u *UnsupportedError
			if errors.As(err, &u) && u.In == "" {
				u.In = "--sort-by"
			}
			return nil, fmt.Errorf("field %q: %w", text, err)
		}
		if len(key) == 0 {
			return nil, fmt.Errorf("field %q: a key is expected", text)
		}
		keys = append(keys, SortKey{Key: key, Reverse: reverse, Text: text})
	}
	return keys, nil
}

// SortBy returns the order of resources sorted by keys, as gcloud's
// --sort-by sorts them: by the text of each key's value (Python's str()
// of it, which is what gcloud compares under Python 3), so numbers sort
// as text ("10" before "9"), and a missing field and null sort as the
// text None. Strings compare by code point. Keys with the same direction
// form groups, applied from the last group to the first with a stable
// sort, so the first key decides first, equal elements keep their order
// in either direction, and "~" reverses only its group.
func SortBy(resources []any, keys []SortKey) ([]int, error) {
	order := make([]int, len(resources))
	for i := range order {
		order[i] = i
	}
	if len(keys) == 0 {
		return order, nil
	}
	// The text of every key for every resource, computed once.
	texts := make([][]string, len(resources))
	for i, r := range resources {
		texts[i] = make([]string, len(keys))
		for j, k := range keys {
			v, err := Get(r, k.Key)
			if err != nil {
				return nil, err
			}
			texts[i][j] = Str(v)
		}
	}
	// Group consecutive keys with the same direction.
	type group struct {
		from, to int
		reverse  bool
	}
	var groups []group
	for j, k := range keys {
		if len(groups) > 0 && groups[len(groups)-1].reverse == k.Reverse {
			groups[len(groups)-1].to = j + 1
			continue
		}
		groups = append(groups, group{from: j, to: j + 1, reverse: k.Reverse})
	}
	for g := len(groups) - 1; g >= 0; g-- {
		grp := groups[g]
		slices.SortStableFunc(order, func(a, b int) int {
			c := slices.Compare(texts[a][grp.from:grp.to], texts[b][grp.from:grp.to])
			if grp.reverse {
				return -c
			}
			return c
		})
	}
	return order, nil
}
