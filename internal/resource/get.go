package resource

import (
	"math/big"
	"strings"
)

// Get returns the value key selects in v, or nil (gcloud's
// resource_property.Get with default None). A key that the resource does
// not have is not an error: Get returns nil.
//
// For an object, a name selects the key of that name; failing that, its
// camelCase or snake_case form (so project_id finds projectId); failing
// that, when the object has an "items" list, the "value" of the item
// whose "key" is the name (compute metadata), parsed as JSON when more of
// the key follows. A [] slice selects every value of the object, or with
// more key, a list of that key applied to every value.
//
// For a list, an index selects an element (negative from the end; out of
// range is nil), and a [] slice every element, or with more key, a list of
// that key applied to every element. A name on a list of objects selects
// as gcloud does: with more key after it, the first object whose field
// <name> equals the next part (then continuing after that part); without,
// the "value" of the object whose "key" is the name; otherwise, when the
// first object has the field, a list of the field of every element (nil
// for those without it), and when it does not, the non-empty values of
// the field in the objects that have it, or nil.
//
// A string is a list of characters for indices and slices. On a number or
// a bool, the names real, imag, numerator and denominator select Python's
// attributes of that name.
//
// It returns an *UnsupportedError for a name that starts with "__" on a
// value that is not an object, which Python would look up as an attribute
// of the value's class, and where gcloud would fail on a list of mixed
// objects and other values.
func Get(v any, key Key) (any, error) {
	resource := v
	for i := 0; i < len(key); i++ {
		part := key[i]
		rest := key[i+1:]
		if resource == nil {
			return nil, nil
		}
		if obj, ok := resource.(*Object); ok {
			if part.Slice {
				if len(rest) == 0 {
					return obj, nil
				}
				out := make([]any, 0, obj.Len())
				for _, k := range obj.keys {
					r, err := Get(obj, append(Key{{Name: k}}, rest...))
					if err != nil {
						return nil, err
					}
					out = append(out, r)
				}
				return out, nil
			}
			if part.IsName() {
				if name, ok := matchingName(part.Name, func(n string) bool { _, ok := obj.vals[n]; return ok }); ok {
					resource = obj.vals[name]
					continue
				}
			}
			if items, ok := obj.vals["items"]; ok {
				r, err := metadataValueMatching(items, part, len(rest) > 0)
				if err != nil {
					return nil, err
				}
				resource = r
				continue
			}
			return nil, nil
		}

		if part.IsName() {
			r, found, err := attribute(resource, part.Name)
			if err != nil || found {
				if err != nil {
					return nil, err
				}
				resource = r
				continue
			}
		}

		var n int
		switch r := resource.(type) {
		case []any:
			n = len(r)
		case string:
			n = len([]rune(r))
		default:
			return nil, nil // not iterable
		}
		if part.Slice {
			if len(rest) == 0 {
				return resource, nil
			}
			out := make([]any, 0, n)
			for j := 0; j < n; j++ {
				r, err := Get(resource, append(Key{{Index: big.NewInt(int64(j))}}, rest...))
				if err != nil {
					return nil, err
				}
				out = append(out, r)
			}
			return out, nil
		}
		if part.Index == nil {
			list, ok := resource.([]any)
			if !ok || len(list) == 0 {
				return nil, nil
			}
			first, ok := list[0].(*Object)
			if !ok {
				return nil, nil
			}
			if len(rest) > 0 {
				if item := metaDict(list, part.Name, keyPartValue(rest[0])); item != nil {
					resource = item
					i++ // the part the item matched
					continue
				}
			} else if r := metaDataValue(list, part.Name); r != nil {
				return r, nil
			}
			if _, ok := first.vals[part.Name]; ok {
				out := make([]any, 0, len(list))
				for j := range list {
					r, err := Get(list, append(Key{{Index: big.NewInt(int64(j))}, part}, rest...))
					if err != nil {
						return nil, err
					}
					out = append(out, r)
				}
				return out, nil
			}
			var out []any
			for _, e := range list {
				o, ok := e.(*Object)
				if !ok {
					return nil, Unsupportedf("selecting %q from a list whose elements are not all objects", part.Name)
				}
				if f := o.vals[part.Name]; Truthy(f) {
					out = append(out, f)
				}
			}
			if len(out) == 0 {
				return nil, nil
			}
			return out, nil
		}
		// An index.
		if !part.Index.IsInt64() {
			return nil, nil
		}
		idx := part.Index.Int64()
		if idx < -int64(n) || idx >= int64(n) {
			return nil, nil
		}
		if idx < 0 {
			idx += int64(n)
		}
		switch r := resource.(type) {
		case []any:
			resource = r[idx]
		case string:
			resource = string([]rune(r)[idx])
		}
	}
	return resource, nil
}

// keyPartValue is the Python value of a key part, for comparing it with
// a field: a name is a str, an index an int, a slice None.
func keyPartValue(p Part) any {
	switch {
	case p.Slice:
		return nil
	case p.Index != nil:
		return p.Index
	}
	return p.Name
}

// matchingName is resource_property.GetMatchingIndex for a name: the name
// itself, its camelCase form, or its snake_case form, whichever exists
// first.
func matchingName(name string, exists func(string) bool) (string, bool) {
	for _, n := range []string{name, CamelCase(name), SnakeCase(name)} {
		if exists(n) {
			return n, true
		}
	}
	return "", false
}

// metadataValueMatching is the "items" branch of Get on an object: the
// metadata value of part (or of its camelCase or snake_case form) in
// items, decoded from JSON when deserialize is set.
func metadataValueMatching(items any, part Part, deserialize bool) (any, error) {
	if !part.IsName() {
		// gcloud's GetMatchingIndexValue tries the index as it is, and
		// keeps only a value that is not empty.
		v, err := metaDataValueDecoded(items, keyPartValue(part), deserialize)
		if err != nil || !Truthy(v) {
			return nil, err
		}
		return v, nil
	}
	for _, n := range []string{part.Name, CamelCase(part.Name), SnakeCase(part.Name)} {
		v, err := metaDataValueDecoded(items, n, deserialize)
		if err != nil {
			return nil, err
		}
		if Truthy(v) {
			return v, nil
		}
	}
	return nil, nil
}

// metaDict is resource_property._GetMetaDict: the first object in items
// whose field key equals value. Iteration stops (with nil) at the first
// element that is not an object, as gcloud's does.
func metaDict(items any, key string, value any) *Object {
	list, ok := items.([]any)
	if !ok {
		return nil
	}
	for _, e := range list {
		o, ok := e.(*Object)
		if !ok {
			return nil
		}
		if Equal(o.vals[key], value) {
			return o
		}
	}
	return nil
}

// metaDataValue is resource_property._GetMetaDataValue without
// deserialization: the "value" field of the object whose "key" is name.
func metaDataValue(items any, name any) any {
	item := metaDict(items, "key", name)
	if item == nil {
		return nil
	}
	return item.vals["value"]
}

// metaDataValueDecoded is _GetMetaDataValue: with deserialize, a string
// value that json.loads reads is returned decoded.
func metaDataValueDecoded(items any, name any, deserialize bool) (any, error) {
	v := metaDataValue(items, name)
	if !deserialize {
		return v, nil
	}
	if s, ok := v.(string); ok {
		if d, err := decodePython([]byte(s)); err == nil {
			return d, nil
		}
	}
	return v, nil
}

// attribute looks name (or its camelCase or snake_case form) up as a
// Python attribute of a value that is not a dict, as Get's hasattr branch
// does. Of the attributes that are not methods, only those of numbers
// exist here; a name that starts with "__" could name a Python internal
// such as __doc__, and is an *UnsupportedError.
func attribute(v any, name string) (any, bool, error) {
	var found any
	ok := false
	if _, isNum := NumberOf(v); isNum {
		n, exists := matchingName(name, func(n string) bool {
			switch n {
			case "real", "imag":
				return true
			case "numerator", "denominator":
				_, isFloat := v.(float64)
				return !isFloat
			}
			return false
		})
		if exists {
			ok = true
			switch n {
			case "real":
				found = realPart(v)
			case "imag":
				if _, isFloat := v.(float64); isFloat {
					found = 0.0
				} else {
					found = big.NewInt(0)
				}
			case "numerator":
				found = realPart(v)
			case "denominator":
				found = big.NewInt(1)
			}
		}
	}
	if !ok && (strings.HasPrefix(name, "__") || strings.HasPrefix(CamelCase(name), "__") || strings.HasPrefix(SnakeCase(name), "__")) {
		return nil, false, Unsupportedf("the key name %q on a value that is not an object", name)
	}
	return found, ok, nil
}

// realPart is x.real: the number itself, with a bool as its int.
func realPart(v any) any {
	if b, ok := v.(bool); ok {
		if b {
			return big.NewInt(1)
		}
		return big.NewInt(0)
	}
	return v
}
