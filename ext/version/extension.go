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

	// Only add if not already present
	if findSubcommand(app.Root, "version") == nil {
		app.Root.AddCommand(NewVersionCommand(app, e.Version, e.Commit, e.Date))
	}

	return nil
}

func findSubcommand(cmd *clix.Command, name string) *clix.Command {
	for _, sub := range cmd.Subcommands {
		if sub.Name == name {
			return sub
		}
		if found := findSubcommand(sub, name); found != nil {
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

	fmt.Fprintf(app.Out, "%s version %s", app.Name, version)

	if commit != "" {
		fmt.Fprintf(app.Out, " (commit: %s)", commit)
	}

	if date != "" {
		fmt.Fprintf(app.Out, " (built: %s)", date)
	}

	fmt.Fprintf(app.Out, "\n")
	fmt.Fprintf(app.Out, "go version %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	return nil
}
