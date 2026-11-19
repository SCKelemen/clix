package clix

import (
	"errors"
	"fmt"
)

// A Value may also have a Reset method:
//
//	Reset()
//
// Reset returns the value to its zero state, as if Set had never been
// called. Run uses it to undo, before the next Run of the same App, what a
// Run did to a custom Value (see FlagSet.Var); the built-in values have it
// too.

// startRun prepares every flag in the tree for a Run. A value that the
// previous Run changed is put back as it was before that Run, unless the
// program changed it since (then the program's value is kept). Then each
// value is recorded as it is now, so the next Run can do the same.
func (a *App) startRun() error {
	var errs []error
	eachFlagOnce(a.Root, func(fs *FlagSet, f *Flag) {
		if f.runChanged && f.Value.String() == f.afterRun {
			switch {
			case f.undoRun != nil:
				if err := f.undoRun(); err != nil {
					errs = append(errs, fmt.Errorf("clix: flag --%s: cannot restore the value it had before the previous Run: %w", f.Name, err))
				}
			case !f.hasDefault() && !f.leakNoted:
				// The default puts back a flag that has one. Anything else
				// carries over; say so where Validate and Err will see it.
				f.leakNoted = true
				fs.errs = append(fs.errs, fmt.Errorf("clix: flag --%s in flag set %q: its value (%T) has no Reset method, so what one Run set carried over to the next; implement Reset() to run the App more than once", f.Name, fs.name, f.Value))
			}
		}
		f.set, f.cliSet, f.source, f.runChanged = false, false, SourceDefault, false
		if b, ok := f.Value.(interface{ beginRun() }); ok {
			b.beginRun()
		}
		f.beforeRun = f.Value.String()
		f.undoRun = undoFunc(f.Value)
	})
	return errors.Join(errs...)
}

// finishRun records which flags the Run changed.
func (a *App) finishRun() {
	eachFlagOnce(a.Root, func(_ *FlagSet, f *Flag) {
		f.afterRun = f.Value.String()
		f.runChanged = f.afterRun != f.beforeRun
	})
}

// eachFlagOnce calls fn for every flag in effect under root, visiting a
// command shared by several groups once.
func eachFlagOnce(root *Command, fn func(*FlagSet, *Flag)) {
	seen := map[*Command]bool{}
	var walk func(*Command)
	walk = func(c *Command) {
		if c == nil || seen[c] {
			return
		}
		seen[c] = true
		if c.Flags != nil {
			for _, f := range c.Flags.flags {
				fn(c.Flags, f)
			}
		}
		for _, child := range c.Children {
			walk(child)
		}
	}
	walk(root)
}

// valueSnapshotter is implemented by the built-in values, which can be put
// back exactly as they are now.
type valueSnapshotter interface {
	snapshot() func()
}

// undoFunc returns a function that puts v back to its current value, or
// nil when v cannot be put back: a custom Value without Reset. A Value
// with Reset is reset and then, when its zero state differs from the
// current text, given that text again with Set.
func undoFunc(v Value) func() error {
	if s, ok := v.(valueSnapshotter); ok {
		restore := s.snapshot()
		return func() error { restore(); return nil }
	}
	r, ok := v.(interface{ Reset() })
	if !ok {
		return nil
	}
	saved := v.String()
	return func() error {
		r.Reset()
		if v.String() == saved {
			return nil
		}
		return v.Set(saved)
	}
}

// snapshotTarget returns a function that puts *p back to its current
// value, allocating p first when it is nil.
func snapshotTarget[T any](p **T) func() {
	if *p == nil {
		*p = new(T)
	}
	target, saved := *p, **p
	return func() { *target = saved }
}

func (s *StringValue) snapshot() func()   { return snapshotTarget(&s.target) }
func (b *BoolValue) snapshot() func()     { return snapshotTarget(&b.target) }
func (d *DurationValue) snapshot() func() { return snapshotTarget(&d.target) }
func (i *IntValue) snapshot() func()      { return snapshotTarget(&i.target) }
func (i *Int64Value) snapshot() func()    { return snapshotTarget(&i.target) }
func (f *Float64Value) snapshot() func()  { return snapshotTarget(&f.target) }

// Reset sets the value to "".
func (s *StringValue) Reset() { *orNewTarget(&s.target) = "" }

// Reset sets the value to false.
func (b *BoolValue) Reset() { *orNewTarget(&b.target) = false }

// Reset sets the value to 0.
func (d *DurationValue) Reset() { *orNewTarget(&d.target) = 0 }

// Reset sets the value to 0.
func (i *IntValue) Reset() { *orNewTarget(&i.target) = 0 }

// Reset sets the value to 0.
func (i *Int64Value) Reset() { *orNewTarget(&i.target) = 0 }

// Reset sets the value to 0.
func (f *Float64Value) Reset() { *orNewTarget(&f.target) = 0 }

// Reset empties the slice, and the next Set replaces it.
func (s *StringSliceValue) Reset() {
	*orNewTarget(&s.target) = nil
	s.changed = false
}

// beginRun makes the first Set of a Run replace the value.
func (s *StringSliceValue) beginRun() { s.changed = false }

// orNewTarget returns *p, allocating it first when it is nil.
func orNewTarget[T any](p **T) *T {
	if *p == nil {
		*p = new(T)
	}
	return *p
}
