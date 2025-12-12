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
// ApplyExtensions().
func (a *App) AddExtension(ext Extension) {
	if a.extensions == nil {
		a.extensions = make([]Extension, 0)
	}
	a.extensions = append(a.extensions, ext)
}

// ApplyExtensions processes all registered extensions in order. This is
// typically called automatically during Run(), but can be called manually
// for testing or early initialization.
// Extensions are applied exactly once using sync.Once for thread-safety.
func (a *App) ApplyExtensions() error {
	if len(a.extensions) == 0 {
		return nil
	}

	var extErr error
	a.extensionsOnce.Do(func() {
		for _, ext := range a.extensions {
			if err := ext.Extend(a); err != nil {
				extErr = err
				return
			}
		}
	})

	return extErr
}
