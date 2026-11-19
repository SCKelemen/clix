package clix

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
		ext := a.extensions[a.extApplied]
		a.extApplied++
		if ext == nil {
			continue
		}
		if err := ext.Extend(a); err != nil {
			a.extErr = err
			return err
		}
	}
	return nil
}
