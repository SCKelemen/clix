package version

import (
	"fmt"
	"runtime"

	"clix"
)

// Extension adds the version command to a clix app.
// This provides version information:
//
//   - cli version - Show version information
//
// Usage:
//
//	import (
//		"clix"
//		"clix/ext/version"
//	)
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(version.Extension{
//		Version: "1.0.0",
//		Commit:  "abc123",  // optional
//		Date:    "2024-01-01", // optional
//	})
//	// Now your app has: myapp version
type Extension struct {
	// Version is the semantic version (e.g., "1.0.0")
	Version string
	// Commit is the git commit hash (optional)
	Commit string
	// Date is the build date (optional)
	Date string
}

// Extend implements clix.Extension.
func (e Extension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}

	// Add global --version flag that shows version info
	app.GlobalFlags.BoolVar(&clix.BoolVarOptions{
		Name:  "version",
		Short: "v",
		Usage: "Show version information",
	})

	// Store version info in app so Run can access it for --version flag
	app.Version = e.Version
	// Note: Commit and Date are only shown in the "version" command, not --version flag
	// This keeps --version simple and consistent with common CLI patterns

	// Only add command if not already present
	if findChild(app.Root, "version") == nil {
		app.Root.AddCommand(NewVersionCommand(app, e.Version, e.Commit, e.Date))
	}

	return nil
}

func findChild(cmd *clix.Command, name string) *clix.Command {
	for _, child := range cmd.Children {
		if child.Name == name {
			return child
		}
		if found := findChild(child, name); found != nil {
			return found
		}
	}
	return nil
}

// NewVersionCommand creates a version command.
func NewVersionCommand(app *clix.App, version, commit, date string) *clix.Command {
	cmd := clix.NewCommand("version")
	cmd.Short = "Show version information"
	cmd.Usage = fmt.Sprintf("%s version", app.Name)
	cmd.Run = func(ctx *clix.Context) error {
		return renderVersion(ctx.App, version, commit, date)
	}
	return cmd
}

func renderVersion(app *clix.App, version, commit, date string) error {
	if version == "" {
		version = "dev"
	}

	format := app.OutputFormat()

	// Build version data structure
	versionData := map[string]interface{}{
		"name":    app.Name,
		"version": version,
		"go": map[string]string{
			"version": runtime.Version(),
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
		},
	}

	if commit != "" {
		versionData["commit"] = commit
	}

	if date != "" {
		versionData["date"] = date
	}

	// Format according to --format flag
	return clix.FormatData(app.Out, versionData, format)
}
