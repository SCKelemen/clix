package clix

import "fmt"

// Extension is the interface that optional "batteries-included" features implement.
// Extensions allow features to be added to an App without requiring imports in
// the core package, keeping simple applications lightweight.
//
// This design is inspired by goldmark's extension system:
// https://github.com/yuin/goldmark
//
// Example:
//
//	type MyExtension struct{}
//
//	func (e MyExtension) Extend(app *clix.App) error {
//		if app.Root != nil {
//			cmd := clix.NewCommand("custom")
//			cmd.Short = "Custom command"
//			app.Root.AddCommand(cmd)
//		}
//		return nil
//	}
//
//	app.AddExtension(MyExtension{})
type Extension interface {
	// Extend is called during app initialization to register commands, hooks,
	// or modify app behavior. Extensions are applied in the order they are added.
	Extend(app *App) error
}

// AddExtension registers an extension with the application. Extensions are
// applied lazily when the app runs, or can be applied immediately by calling
// ApplyExtensions(). An extension added after a Run is applied by the next
// Run, and one added by another extension's Extend is applied right after
// it.
func (a *App) AddExtension(ext Extension) {
	a.extensions = append(a.extensions, ext)
}

// ApplyExtensions applies, in order, every registered extension that has
// not been applied yet. Run calls it automatically, but it can be called
// manually for testing or early initialization. Each extension is applied
// exactly once, and ApplyExtensions is safe for concurrent use.
//
// Every flag and command registered while an extension's Extend runs
// belongs to that extension; everything else belongs to the app. When
// names collide, the app's definitions win over an extension's, and
// between extensions the one added first wins, whatever the order of
// registration (see FlagSet.Err and App.Validate, which report each
// override).
//
// If an extension returns an error, the extensions after it are not
// applied and the error is returned by this and every later call (and so
// by every later Run): the App is left partly extended and must not run.
func (a *App) ApplyExtensions() error {
	a.extMu.Lock()
	defer a.extMu.Unlock()
	if a.extErr != nil {
		return a.extErr
	}
	for a.extApplied < len(a.extensions) {
		i := a.extApplied
		ext := a.extensions[i]
		a.extApplied++
		if ext == nil {
			continue
		}
		// What exists now was defined by the app (or by earlier
		// extensions, which already claimed theirs); what is new after
		// Extend belongs to this extension.
		a.claimDefinitions(0, "")
		a.extending = true
		err := ext.Extend(a)
		a.extending = false
		a.claimDefinitions(i+1, fmt.Sprintf("#%d (%T)", i+1, ext))
		if err != nil {
			a.extErr = err
			return err
		}
	}
	return nil
}

// claimDefinitions records rank and name as the owner of every command and
// flag in the tree (and in a root flag set waiting to move to a replaced
// Root) that has no owner yet, then re-resolves name conflicts.
func (a *App) claimDefinitions(rank int, name string) {
	if a.boundFlags != nil && a.boundFlags != a.rootFlagSet() {
		claimFlags(a, a.boundFlags, rank, name)
	}
	claimTree(a, a.Root, rank, name)
}

func claimTree(a *App, cmd *Command, rank int, name string) {
	if cmd == nil {
		return
	}
	cmd.claim(rank, name)
	if cmd.Flags != nil {
		// The flag set belongs to the command's owner.
		cmd.Flags.owner = cmd.owner
	}
	claimFlags(a, cmd.Flags, rank, name)
	for _, child := range cmd.Children {
		claimTree(a, child, rank, name)
	}
}

func claimFlags(a *App, fs *FlagSet, rank int, name string) {
	if fs == nil {
		return
	}
	fs.app = a
	changed := false
	for _, f := range fs.all {
		if !f.claimed {
			f.claim(rank, name)
			changed = changed || rank != 0
		}
	}
	if changed {
		fs.reindex()
	}
}
