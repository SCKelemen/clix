package clix

// Styles defines styling hooks for textual CLI output such as help screens.
// All fields are optional - unset styles produce plain text output.
// Styles are compatible with lipgloss and can use any TextStyle implementation.
//
// Example:
//
//	app.Styles = clix.Styles{
//		AppTitle:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")),
//		CommandTitle: lipgloss.NewStyle().Bold(true),
//		FlagName:     lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
//	}
type Styles struct {
	// AppTitle styles the application title in help output.
	AppTitle TextStyle

	// AppDescription styles the application description in help output.
	AppDescription TextStyle

	// CommandTitle styles command names in help output.
	CommandTitle TextStyle

	// SectionHeading styles section headings (e.g., "FLAGS", "ARGUMENTS") in help output.
	SectionHeading TextStyle

	// Usage styles usage strings in help output.
	Usage TextStyle

	// FlagName styles flag names (e.g., "--project") in help output.
	// Used as the default for both global and local flag names.
	FlagName TextStyle

	// FlagUsage styles flag usage text in help output.
	// Used as the default for both global and local flag usage.
	FlagUsage TextStyle

	// AppFlagName styles app-level flag names (root command flags shown everywhere).
	// Falls back to FlagName when unset.
	AppFlagName TextStyle

	// AppFlagUsage styles app-level flag usage text.
	// Falls back to FlagUsage when unset.
	AppFlagUsage TextStyle

	// CommandFlagName styles command-level flag names.
	// Falls back to FlagName when unset.
	CommandFlagName TextStyle

	// CommandFlagUsage styles command-level flag usage text.
	// Falls back to FlagUsage when unset.
	CommandFlagUsage TextStyle

	// ArgumentName styles argument names in help output.
	ArgumentName TextStyle

	// ArgumentMarker styles argument markers (e.g., "<name>", "[name]") in help output.
	ArgumentMarker TextStyle

	// ChildName styles child command and group names in help output.
	// Used for both groups and commands in the GROUPS and COMMANDS sections.
	ChildName TextStyle

	// ChildDesc styles child command and group descriptions in help output.
	// Used for both groups and commands in the GROUPS and COMMANDS sections.
	ChildDesc TextStyle

	// Example styles example text in help output.
	Example TextStyle
}

// DefaultStyles leaves all styles unset, producing plain text output.
var DefaultStyles = Styles{}
