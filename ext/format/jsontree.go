package format

import (
	"bytes"
	"encoding/json/jsontext"
)

// jsonObject is a JSON object decoded with its members in order. Names
// may repeat (a MarshalJSON method can write them twice); lookups find
// the first.
type jsonObject struct {
	members []jsonMember
}

// jsonMember is one member of a jsonObject. value is a *jsonObject, a
// []any for an array, or a jsontext.Value holding a string, number,
// boolean or null exactly as it was written, so numbers keep their text.
type jsonMember struct {
	name  string
	value any
}

// index returns the index of the first member named name, or -1.
func (o *jsonObject) index(name string) int {
	for i, m := range o.members {
		if m.name == name {
			return i
		}
	}
	return -1
}

// insert adds a member at position i.
func (o *jsonObject) insert(i int, name string, value any) {
	o.members = append(o.members, jsonMember{})
	copy(o.members[i+1:], o.members[i:])
	o.members[i] = jsonMember{name, value}
}

var jsonNull = jsontext.Value("null")

// decodeTree decodes doc, JSON as clix.FormatData writes it, into a tree
// of *jsonObject, []any and jsontext.Value.
func decodeTree(doc []byte) (any, error) {
	dec := jsontext.NewDecoder(bytes.NewReader(doc), decodeOptions)
	return readTree(dec)
}

func readTree(dec *jsontext.Decoder) (any, error) {
	switch dec.PeekKind() {
	case '{':
		if _, err := dec.ReadToken(); err != nil {
			return nil, err
		}
		o := &jsonObject{}
		for dec.PeekKind() != '}' {
			name, err := dec.ReadToken()
			if err != nil {
				return nil, err
			}
			key := name.String()
			v, err := readTree(dec)
			if err != nil {
				return nil, err
			}
			o.members = append(o.members, jsonMember{key, v})
		}
		_, err := dec.ReadToken()
		return o, err
	case '[':
		if _, err := dec.ReadToken(); err != nil {
			return nil, err
		}
		list := []any{}
		for dec.PeekKind() != ']' {
			v, err := readTree(dec)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		_, err := dec.ReadToken()
		return list, err
	default:
		v, err := dec.ReadValue()
		if err != nil {
			return nil, err
		}
		return v.Clone(), nil
	}
}

// encodeTree writes v as clix.FormatData writes JSON: indented by two
// spaces and followed by a newline.
func encodeTree(v any) ([]byte, error) {
	var out bytes.Buffer
	enc := jsontext.NewEncoder(&out, encodeOptions)
	if err := writeTree(enc, v); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeTree(enc *jsontext.Encoder, v any) error {
	switch x := v.(type) {
	case *jsonObject:
		if err := enc.WriteToken(jsontext.BeginObject); err != nil {
			return err
		}
		for _, m := range x.members {
			if err := enc.WriteToken(jsontext.String(m.name)); err != nil {
				return err
			}
			if err := writeTree(enc, m.value); err != nil {
				return err
			}
		}
		return enc.WriteToken(jsontext.EndObject)
	case []any:
		if err := enc.WriteToken(jsontext.BeginArray); err != nil {
			return err
		}
		for _, e := range x {
			if err := writeTree(enc, e); err != nil {
				return err
			}
		}
		return enc.WriteToken(jsontext.EndArray)
	case jsontext.Value:
		return enc.WriteValue(x)
	case nil:
		return enc.WriteValue(jsonNull)
	}
	panic("format: unexpected JSON tree node")
}

// cloneTree returns a deep copy of v, so that filling one copy does not
// change another.
func cloneTree(v any) any {
	switch x := v.(type) {
	case *jsonObject:
		o := &jsonObject{members: make([]jsonMember, len(x.members))}
		for i, m := range x.members {
			o.members[i] = jsonMember{m.name, cloneTree(m.value)}
		}
		return o
	case []any:
		list := make([]any, len(x))
		for i, e := range x {
			list[i] = cloneTree(e)
		}
		return list
	}
	return v // a jsontext.Value is never modified in place
}
