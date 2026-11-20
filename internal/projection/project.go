package projection

import (
	"github.com/SCKelemen/clix/v2/internal/resource"
)

// projector is gcloud's resource_projector.Projector for JSON values.
type projector struct {
	columns bool // the projection has columns
	retain  bool // keep object entries whose value is None
	empty   *node
}

func newProjector(columns, retain bool) *projector {
	return &projector{columns: columns, retain: retain, empty: newNode(flagProject)}
}

// project is Projector._Project.
func (p *projector) project(obj any, n *node, flag int, leaf bool) any {
	switch obj.(type) {
	case *resource.Object, []any:
		if n != nil && len(n.transforms) > 0 {
			return applyTransforms(n.transforms, obj)
		}
		if flag >= flagProject || (n != nil && !n.empty()) {
			if o, ok := obj.(*resource.Object); ok {
				return p.projectDict(o, n, flag)
			}
			return p.projectList(obj.([]any), n, flag)
		}
		return obj
	}
	if leaf {
		return obj
	}
	return p.projectAttribute(obj, n, flag)
}

// projectAttribute is Projector._ProjectAttribute.
func (p *projector) projectAttribute(obj any, n *node, flag int) any {
	if flag < flagProject {
		return nil
	}
	if n != nil && len(n.transforms) > 0 {
		return applyTransforms(n.transforms, obj)
	}
	return p.project(obj, n, flag, true)
}

// projectDict is Projector._ProjectDict. An empty object stays as it is;
// one left with no entries is None.
func (p *projector) projectDict(obj *resource.Object, n *node, flag int) any {
	if obj.Len() == 0 {
		return obj
	}
	res := resource.NewObject()
	for _, key := range obj.Keys() {
		val, _ := obj.Get(key)
		f := flag
		if c := n.names[key]; c != nil {
			f |= c.flag
			if f < flagInner {
				continue
			}
			val = p.project(val, c, f, false)
		} else {
			val = p.projectAttribute(val, p.empty, f)
		}
		if val != nil || p.retain || (f >= flagProject && p.columns) {
			res.Set(key, val)
		}
	}
	if res.Len() == 0 {
		return nil
	}
	return res
}

// projectList is Projector._ProjectList: the elements a slice or index
// in the projection selects (all of them when projecting everything),
// with trailing None elements dropped; None when none is left.
func (p *projector) projectList(obj []any, n *node, flag int) any {
	if len(obj) == 0 {
		return []any{}
	}
	var sliced *node
	var indices []int64
	if n.empty() {
		if flag < flagProject {
			return nil
		}
	} else {
		if n.slice != nil && (flag >= flagProject || n.slice.flag != 0) {
			sliced = n.slice
		}
		for i := range n.indices {
			if i >= -int64(len(obj)) && i < int64(len(obj)) {
				indices = append(indices, i)
			}
		}
	}
	if flag >= flagProject && sliced == nil {
		sliced = p.empty
	}
	if len(indices) == 0 && sliced == nil {
		return nil
	}
	var res []any
	var order []int64
	if sliced != nil {
		res = make([]any, len(obj))
		for i := range obj {
			order = append(order, int64(i))
		}
	} else {
		top := int64(-1)
		for _, i := range indices {
			if i < 0 {
				i += int64(len(obj))
			}
			top = max(top, i)
		}
		res = make([]any, top+1)
		order = indices
	}
	maxIndex := int64(-1)
	for _, index := range order {
		pos := index
		if pos < 0 {
			pos += int64(len(obj))
		}
		val := obj[pos]
		if val == nil {
			continue
		}
		f := flag
		child := sliced
		if c, ok := n.indices[index]; ok {
			child = c
			if sliced != nil {
				f |= sliced.flag
			}
		}
		if child != nil {
			f |= child.flag
			if f >= flagInner {
				val = p.project(val, child, f, false)
			} else {
				val = nil
			}
		}
		if val == nil {
			continue
		}
		maxIndex = max(maxIndex, pos)
		res[pos] = val
	}
	if maxIndex < 0 {
		return nil
	}
	if sliced != nil {
		return res[:maxIndex+1]
	}
	return res
}

// evaluate is Projector.Evaluate for formats that print projected
// resources (json, yaml, flattened, and list without a projection).
func (f *Format) evaluate(obj any, retain bool) any {
	if len(f.Columns) > 0 {
		return newProjector(true, false).project(obj, f.root, flagDefault, false)
	}
	return newProjector(false, retain).project(obj, f.root, flagProject, false)
}

// row is Projector.Evaluate for formats that print columns: the value of
// each column's key in the resource (serialized first, which drops
// trailing None elements of lists), with the column's transforms applied.
func (f *Format) row(obj any) ([]any, error) {
	p := newProjector(true, false)
	serialized := p.project(obj, p.empty, flagProject, false)
	out := make([]any, len(f.Columns))
	for i, c := range f.Columns {
		v, err := resource.Get(serialized, c.Key)
		if err != nil {
			return nil, err
		}
		out[i] = applyTransforms(c.Transforms, v)
	}
	return out, nil
}
