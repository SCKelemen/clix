package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"

	"github.com/SCKelemen/clix/v2"
)

// Extension adds the version command and --version flag to a clix app.
// This provides version information in multiple formats (text, json, yaml).
//
// The extension adds:
//   - cli version - Show detailed version information (includes commit, date, Go version)
//   - cli --version / -v - Show simple version info inline ("cli version 1.0.0")
//
// The app's own definitions win: when the app uses -v (for example, for
// --verbose), the extension's flag is --version only, and an app command
// named "version" replaces the extension's. Adding the extension twice
// registers the flag and command once.
//
// When Version is empty, App.Version is used if set; otherwise the version
// comes from the build information Go embeds in the binary
// (runtime/debug.ReadBuildInfo): the main module's version, as set by
// "go install example.com/cmd@v1.2.3" or, since Go 1.24, by "go build" in a
// tagged checkout, or "dev" when there is none. Commit and Date then
// default to the VCS revision and commit time, and a build with
// uncommitted changes is reported with modified = true.
//
// Example:
//
//	import (
//		"github.com/SCKelemen/clix/v2"
//		"github.com/SCKelemen/clix/v2/ext/version"
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
	// If empty, App.Version or the binary's build information is used (see
	// Extension), and "dev" when neither has one.
	Version string

	// Commit is the git commit hash (optional).
	// Only shown in the "version" command, not in the --version flag.
	Commit string

	// Date is the build date (optional).
	// Only shown in the "version" command, not in the --version flag.
	Date string
}

// readBuildInfo is runtime/debug.ReadBuildInfo; tests replace it.
var readBuildInfo = debug.ReadBuildInfo

// info is the version information an Extension shows.
type info struct {
	version, commit, date string
	modified              bool // built from a checkout with uncommitted changes
}

// resolve fills in what e leaves empty, from appVersion and the build
// information.
func (e Extension) resolve(appVersion string) info {
	v := info{version: e.Version, commit: e.Commit, date: e.Date}
	if v.version == "" {
		v.version = appVersion
	}
	if v.version != "" {
		return v
	}
	if bi, ok := readBuildInfo(); ok && bi != nil {
		if mv := bi.Main.Version; mv != "" && mv != "(devel)" {
			v.version = mv
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if v.commit == "" {
					v.commit = s.Value
				}
			case "vcs.time":
				if v.date == "" {
					v.date = s.Value
				}
			case "vcs.modified":
				v.modified = s.Value == "true"
			}
		}
	}
	if v.version == "" {
		v.version = "dev"
	}
	return v
}

// versionSwitch is the value of the --version flag. Its type marks the flag
// as registered by this extension.
type versionSwitch struct{ on bool }

func (s *versionSwitch) Set(v string) error {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return err
	}
	s.on = b
	return nil
}

func (s *versionSwitch) SetBool(b bool) error {
	s.on = b
	return nil
}

func (s *versionSwitch) String() string { return strconv.FormatBool(s.on) }

// Reset turns the switch off before the next Run of the same App, so a
// --version in one Run does not carry over (see clix.FlagSet.Var).
func (s *versionSwitch) Reset() { s.on = false }

// Extend implements clix.Extension.
func (e Extension) Extend(app *clix.App) error {
	if app.Root == nil {
		return nil
	}
	if f := app.Flags().Lookup("version"); f != nil {
		if _, ours := f.Value.(*versionSwitch); ours {
			return nil // already added
		}
	}

	v := e.resolve(app.Version)

	// Add global --version flag that shows version info
	app.Flags().Var(clix.VarOptions{
		FlagOptions: clix.FlagOptions{
			Name:  "version",
			Short: "v",
			Usage: "Show version information",
		},
		Value: &versionSwitch{},
	})

	// Store version info in app so Run can access it for --version flag
	app.Version = v.version
	// Note: Commit and Date are only shown in the "version" command, not --version flag
	// This keeps --version simple and consistent with common CLI patterns

	// Only add command if not already present
	if findChild(app.Root, "version") == nil {
		app.Root.AddCommand(newVersionCommand(app, v))
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

// NewVersionCommand creates a version command that shows the given
// version ("dev" when empty), commit and date.
func NewVersionCommand(app *clix.App, version, commit, date string) *clix.Command {
	return newVersionCommand(app, info{version: version, commit: commit, date: date})
}

func newVersionCommand(app *clix.App, v info) *clix.Command {
	cmd := clix.NewCommand("version")
	cmd.Short = "Show version information"
	cmd.Usage = fmt.Sprintf("%s version", app.Name)
	cmd.IsExtensionCommand = true
	cmd.Run = func(ctx *clix.Context) error {
		return renderVersion(ctx.App, v)
	}
	return cmd
}

func renderVersion(app *clix.App, v info) error {
	if v.version == "" {
		v.version = "dev"
	}

	format := clix.FormatText
	if f, ok := app.Flags().String("format"); ok && f != "" {
		format = f
	}

	// Build version data structure
	versionData := map[string]interface{}{
		"name":    app.Name,
		"version": v.version,
		"go": map[string]string{
			"version": runtime.Version(),
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
		},
	}

	if v.commit != "" {
		versionData["commit"] = v.commit
	}

	if v.date != "" {
		versionData["date"] = v.date
	}

	if v.modified {
		versionData["modified"] = true
	}

	// Format according to --format flag
	return clix.FormatData(app.Out, versionData, format)
}
