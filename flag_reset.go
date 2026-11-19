package clix

import "fmt"

// A Value may also have a Reset method:
//
//	Reset()
//
// Reset returns the value to its zero state, as if Set had never been
// called. Before the next Run of the same App, Run calls it on a custom
// Value (see FlagSet.Var) that a Run called Set on, whatever the value's
// String returns; the built-in values have it too. Run does not pass the
// old String back to Set afterwards, because String need not be text that
// Set accepts (a secret may print as <redacted>), so a value that should
// start from something other than its zero state gets that from a Default,
// which every Run applies again, or from Reset itself.

// startRun prepares every flag in the tree for a Run and returns the
// function that ends the Run.
//
// A Run started from outside the App first undoes what the previous one
// changed: every value that Run called Set on, or that was different when
// it ended, is put back as it was before that Run, unless the program
// changed it since (then the program's value is kept). Then each value is
// recorded as it is now, so the next Run can do the same. A Run started
// by a handler of a running Run is prepared by startNestedRun instead.
func (a *App) startRun() (end func()) {
	eachFlagSetOnce(a.Root, func(fs *FlagSet) { fs.app = a })
	if a.running {
		return a.startNestedRun()
	}
	eachFlagOnce(a.Root, func(fs *FlagSet, f *Flag) {
		if f.runChanged && f.Value.String() == f.afterRun {
			switch {
			case f.undoRun != nil:
				f.undoRun()
			case !f.hasDefault() && !f.leakNoted:
				// The default puts back a flag that has one. Anything else
				// carries over; say so where Validate and Err will see it.
				f.leakNoted = true
				fs.errs = append(fs.errs, fmt.Errorf("clix: flag --%s in flag set %q: its value (%T) has no Reset method, so what one Run set carried over to the next; implement Reset() to run the App more than once", f.Name, fs.name, f.Value))
			}
		}
		f.runChanged = false
		f.beginRun()
		f.beforeRun = f.Value.String()
		f.undoRun = undoFunc(f.Value)
		f.tracked = true
	})
	a.running = true
	return func() {
		a.finishRun()
		a.running = false
	}
}

// startNestedRun prepares the flags for a Run that a handler started while
// another Run of the App is running, and returns the function that ends
// it. The nested Run starts from the values the outer Run left, with no
// flag set yet. When it ends, it puts back every value it changed, what
// the outer Run recorded about each flag (Changed, Source, ...) and the
// command path, so the handler that started it continues as before. A
// built-in value is put back exactly. A custom Value the nested Run set is
// Reset (see Value), and one without Reset keeps what the nested Run set.
func (a *App) startNestedRun() (end func()) {
	type outer struct {
		set, cliSet, touched, shadowed bool
		source                         Source
		groupSkip                      fillSkip
		value                          string // Value.String() when the nested Run started
		restore                        func()
	}
	saved := make(map[*Flag]outer)
	eachFlagOnce(a.Root, func(_ *FlagSet, f *Flag) {
		saved[f] = outer{f.set, f.cliSet, f.touched, f.shadowed, f.source, f.groupSkip, f.Value.String(), undoFunc(f.Value)}
		f.beginRun()
	})
	parents := make(map[*Command]*Command)
	eachCommandOnce(a.Root, func(c *Command) { parents[c] = c.parent })
	return func() {
		eachFlagOnce(a.Root, func(_ *FlagSet, f *Flag) {
			o, ok := saved[f]
			if !ok {
				return // registered during the nested Run
			}
			changed := f.touched || f.Value.String() != o.value
			if changed && o.restore != nil {
				o.restore()
			}
			// A value that could not be put back counts as changed by the
			// outer Run, so the next Run reports or undoes it.
			f.set, f.cliSet, f.touched, f.shadowed, f.source, f.groupSkip = o.set, o.cliSet, o.touched || (changed && o.restore == nil), o.shadowed, o.source, o.groupSkip
		})
		for c, p := range parents {
			c.parent = p
		}
	}
}

// beginRun clears what a Run records about the flag: whether, where from
// and how it was set.
func (f *Flag) beginRun() {
	f.set, f.cliSet, f.source, f.touched, f.shadowed, f.groupSkip = false, false, SourceDefault, false, false, 0
	if b, ok := f.Value.(interface{ beginRun() }); ok {
		b.beginRun()
	}
}

// setValue sets the flag's value from text with Value.Set, and records
// that the Run did, so the next Run undoes it.
func (f *Flag) setValue(text string) error {
	f.touched = true
	return f.Value.Set(text)
}

// finishRun records which flags the Run changed: those it called Set on,
// and those whose value is different now, such as a bound variable that a
// handler assigned.
func (a *App) finishRun() {
	eachFlagOnce(a.Root, func(_ *FlagSet, f *Flag) {
		// A flag registered during the Run has no value from before it,
		// so there is nothing to undo.
		f.afterRun = f.Value.String()
		f.runChanged = f.tracked && (f.touched || f.afterRun != f.beforeRun)
	})
}

// eachFlagOnce calls fn for every flag in effect under root, visiting a
// command shared by several groups once.
func eachFlagOnce(root *Command, fn func(*FlagSet, *Flag)) {
	eachFlagSetOnce(root, func(fs *FlagSet) {
		for _, f := range fs.flags {
			fn(fs, f)
		}
	})
}

// eachFlagSetOnce calls fn for the flag set of every command under root,
// visiting a command shared by several groups once.
func eachFlagSetOnce(root *Command, fn func(*FlagSet)) {
	eachCommandOnce(root, func(c *Command) {
		if c.Flags != nil {
			fn(c.Flags)
		}
	})
}

// eachCommandOnce calls fn for root and every command under it, visiting
// a command shared by several groups once.
func eachCommandOnce(root *Command, fn func(*Command)) {
	seen := map[*Command]bool{}
	var walk func(*Command)
	walk = func(c *Command) {
		if c == nil || seen[c] {
			return
		}
		seen[c] = true
		fn(c)
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

// undoFunc returns a function that puts v back: a built-in value exactly
// as it is now, and a custom Value with Reset by calling Reset (see
// Value). It returns nil for a custom Value without Reset, which cannot
// be put back.
func undoFunc(v Value) func() {
	if s, ok := v.(valueSnapshotter); ok {
		return s.snapshot()
	}
	if r, ok := v.(interface{ Reset() }); ok {
		return r.Reset
	}
	return nil
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
