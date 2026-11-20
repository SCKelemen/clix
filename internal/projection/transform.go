package projection

import (
	"strings"

	"github.com/SCKelemen/clix/v2/internal/resource"
)

// transform is one call of a supported transform function.
type transform struct {
	name      string
	sep       string // join
	undefined string // basename, join
}

// transformParams lists the parameters of the supported transforms, in
// order, as gcloud's Python functions declare them after the resource.
var transformParams = map[string][]string{
	"basename": {"undefined"},
	"join":     {"sep", "undefined"},
	"len":      nil,
}

// transforms parses a chain of transform calls starting with fn, whose
// "(" has been consumed (resource_lex.Lexer.Transform), binding the
// arguments as Python would: positional ones in order, then name=value
// ones.
func (p *parser) transforms(fn string) ([]transform, error) {
	l := p.l
	var out []transform
	for {
		params, ok := transformParams[fn]
		if !ok {
			return nil, resource.Unsupportedf("the transform %s()", fn)
		}
		args, err := l.Args(",")
		if err != nil {
			return nil, err
		}
		t := transform{name: fn, sep: "/"}
		set := map[string]bool{}
		positional := 0
		for _, a := range args {
			name, value, isKw := strings.Cut(a.Text, "=")
			if !isKw || len(params) == 0 {
				if positional >= len(params) {
					return nil, resource.Unsupportedf("%s() with %d arguments", fn, len(args))
				}
				name, value = params[positional], a.Text
				positional++
			}
			if set[name] {
				return nil, resource.Unsupportedf("%s() with %s given twice", fn, name)
			}
			set[name] = true
			switch name {
			case "sep":
				t.sep = value
			case "undefined":
				t.undefined = value
			default:
				return nil, resource.Unsupportedf("the argument %s of %s()", name, fn)
			}
		}
		out = append(out, t)
		if c, _ := l.IsCharacter(".", false, true); c == 0 {
			return out, nil
		}
		here := l.Position()
		call, err := l.Key()
		if err != nil {
			return nil, err
		}
		if c, _ := l.IsCharacter("(", false, true); c == 0 {
			return nil, l.ErrorAt(here, "Transform function expected")
		}
		if len(call) != 1 || !call[0].IsName() {
			return nil, resource.Unsupportedf("the transform %s()", call)
		}
		fn = call[0].Name
	}
}

// apply runs the transforms on v in order.
func applyTransforms(ts []transform, v any) any {
	for _, t := range ts {
		v = t.apply(v)
	}
	return v
}

// apply is gcloud's TransformLen, TransformBaseName or TransformJoin.
func (t transform) apply(v any) any {
	switch t.name {
	case "len":
		switch v := v.(type) {
		case string:
			return pyInt(runeLen(v))
		case []any:
			return pyInt(len(v))
		case *resource.Object:
			return pyInt(v.Len())
		}
		return pyInt(0)
	case "basename":
		return basename(v, t.undefined)
	case "join":
		var parts []string
		switch v := v.(type) {
		case string:
			for _, r := range v {
				parts = append(parts, string(r))
			}
		case []any:
			for _, e := range v {
				parts = append(parts, resource.Str(e))
			}
		case *resource.Object:
			parts = append(parts, v.Keys()...)
		default:
			return t.undefined
		}
		if s := strings.Join(parts, t.sep); s != "" {
			return s
		}
		return t.undefined
	}
	return v
}

// basename is TransformBaseName: the part after the last "/" (or, when
// there is none, the last "\"), mapped over a list.
func basename(v any, undefined string) any {
	if !resource.Truthy(v) {
		return undefined
	}
	if list, ok := v.([]any); ok {
		out := make([]any, len(list))
		for i, e := range list {
			out[i] = basename(e, "")
		}
		return out
	}
	s := resource.Str(v)
	for _, sep := range []string{"/", `\`} {
		if i := strings.LastIndex(s, sep); i >= 0 {
			return s[i+1:]
		}
	}
	if s == "" {
		return undefined
	}
	return s
}
