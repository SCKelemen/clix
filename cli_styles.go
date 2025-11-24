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
	FlagName TextStyle

	// FlagUsage styles flag usage text in help output.
	FlagUsage TextStyle

	// ArgumentName styles argument names in help output.
	ArgumentName TextStyle

	// ArgumentMarker styles argument markers (e.g., "<name>", "[name]") in help output.
	ArgumentMarker TextStyle

	// SubcommandName styles subcommand names in help output.
	SubcommandName TextStyle

	// SubcommandDesc styles subcommand descriptions in help output.
	SubcommandDesc TextStyle

	// Example styles example text in help output.
	Example TextStyle
}

// DefaultStyles leaves all styles unset, producing plain text output.
var DefaultStyles = Styles{}
