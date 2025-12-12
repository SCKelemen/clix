package version

import (
	"fmt"
	"runtime"

	"github.com/SCKelemen/clix"
)

// Extension adds the version command and --version flag to a clix app.
// This provides version information in multiple formats (text, json, yaml).
//
// The extension adds:
//   - cli version - Show detailed version information (includes commit, date, Go version)
//   - cli --version / -v - Show simple version info inline
//
// Example:
//
//	import (
//		"github.com/SCKelemen/clix"
//		"github.com/SCKelemen/clix/ext/version"
//	)
//
//	app := clix.NewApp("myapp")
//	app.AddExtension(version.Extension{
//		Version: "1.0.0",
//		Commit:  "abc123",  // optional
//		Date:    "2024-01-01", // optional
//	})
//	// Now your app has: myapp version and myapp --version
type Extension struct {
	// Version is the semantic version (e.g., "1.0.0").
	// Required. If empty, defaults to "dev" in the version command.
	Version string

	// Commit is the git commit hash (optional).
	// Only shown in the "version" command, not in the --version flag.
	Commit string

	// Date is the build date (optional).
	// Only shown in the "version" command, not in the --version flag.
	Date string
}

// Extend implements clix.Extension.
func (e Extension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}

	// Add global --version flag that shows version info
	app.Flags().BoolVar(clix.BoolVarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "version",
			Short: "v",
			Usage: "Show version information",
		},
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
	// Use ResolvePath for consistent behavior with core library
	if resolved := cmd.ResolvePath([]string{name}); resolved != nil {
		return resolved
	}
	return nil
}

// NewVersionCommand creates a version command.
func NewVersionCommand(app *clix.App, version, commit, date string) *clix.Command {
	cmd := clix.NewCommand("version")
	cmd.Short = "Show version information"
	cmd.Usage = fmt.Sprintf("%s version", app.Name)
	cmd.IsExtensionCommand = true
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
