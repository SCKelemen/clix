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

// StyleOption configures Styles using the functional options pattern.
type StyleOption interface {
	ApplyStyle(*Styles)
}

// WithAppTitle sets the app title style.
func WithAppTitle(style TextStyle) StyleOption {
	return styleAppTitleOption{style: style}
}

// WithStyleAppDescription sets the app description style.
func WithStyleAppDescription(style TextStyle) StyleOption {
	return styleAppDescriptionOption{style: style}
}

// WithCommandTitle sets the command title style.
func WithCommandTitle(style TextStyle) StyleOption {
	return styleCommandTitleOption{style: style}
}

// WithSectionHeading sets the section heading style.
func WithSectionHeading(style TextStyle) StyleOption {
	return styleSectionHeadingOption{style: style}
}

// WithUsage sets the usage string style.
func WithUsage(style TextStyle) StyleOption {
	return styleUsageOption{style: style}
}

// WithStyleFlagName sets the flag name style (default for both app and command flags).
func WithStyleFlagName(style TextStyle) StyleOption {
	return styleFlagNameOption{style: style}
}

// WithStyleFlagUsage sets the flag usage style (default for both app and command flags).
func WithStyleFlagUsage(style TextStyle) StyleOption {
	return styleFlagUsageOption{style: style}
}

// WithAppFlagName sets the app-level flag name style.
func WithAppFlagName(style TextStyle) StyleOption {
	return styleAppFlagNameOption{style: style}
}

// WithAppFlagUsage sets the app-level flag usage style.
func WithAppFlagUsage(style TextStyle) StyleOption {
	return styleAppFlagUsageOption{style: style}
}

// WithCommandFlagName sets the command-level flag name style.
func WithCommandFlagName(style TextStyle) StyleOption {
	return styleCommandFlagNameOption{style: style}
}

// WithCommandFlagUsage sets the command-level flag usage style.
func WithCommandFlagUsage(style TextStyle) StyleOption {
	return styleCommandFlagUsageOption{style: style}
}

// WithArgumentName sets the argument name style.
func WithArgumentName(style TextStyle) StyleOption {
	return styleArgumentNameOption{style: style}
}

// WithArgumentMarker sets the argument marker style.
func WithArgumentMarker(style TextStyle) StyleOption {
	return styleArgumentMarkerOption{style: style}
}

// WithChildName sets the child name style.
func WithChildName(style TextStyle) StyleOption {
	return styleChildNameOption{style: style}
}

// WithChildDesc sets the child description style.
func WithChildDesc(style TextStyle) StyleOption {
	return styleChildDescOption{style: style}
}

// WithExample sets the example style.
func WithExample(style TextStyle) StyleOption {
	return styleExampleOption{style: style}
}

// Internal option types

type styleAppTitleOption struct{ style TextStyle }

func (o styleAppTitleOption) ApplyStyle(s *Styles) { s.AppTitle = o.style }

type styleAppDescriptionOption struct{ style TextStyle }

func (o styleAppDescriptionOption) ApplyStyle(s *Styles) { s.AppDescription = o.style }

type styleCommandTitleOption struct{ style TextStyle }

func (o styleCommandTitleOption) ApplyStyle(s *Styles) { s.CommandTitle = o.style }

type styleSectionHeadingOption struct{ style TextStyle }

func (o styleSectionHeadingOption) ApplyStyle(s *Styles) { s.SectionHeading = o.style }

type styleUsageOption struct{ style TextStyle }

func (o styleUsageOption) ApplyStyle(s *Styles) { s.Usage = o.style }

type styleFlagNameOption struct{ style TextStyle }

func (o styleFlagNameOption) ApplyStyle(s *Styles) { s.FlagName = o.style }

type styleFlagUsageOption struct{ style TextStyle }

func (o styleFlagUsageOption) ApplyStyle(s *Styles) { s.FlagUsage = o.style }

type styleAppFlagNameOption struct{ style TextStyle }

func (o styleAppFlagNameOption) ApplyStyle(s *Styles) { s.AppFlagName = o.style }

type styleAppFlagUsageOption struct{ style TextStyle }

func (o styleAppFlagUsageOption) ApplyStyle(s *Styles) { s.AppFlagUsage = o.style }

type styleCommandFlagNameOption struct{ style TextStyle }

func (o styleCommandFlagNameOption) ApplyStyle(s *Styles) { s.CommandFlagName = o.style }

type styleCommandFlagUsageOption struct{ style TextStyle }

func (o styleCommandFlagUsageOption) ApplyStyle(s *Styles) { s.CommandFlagUsage = o.style }

type styleArgumentNameOption struct{ style TextStyle }

func (o styleArgumentNameOption) ApplyStyle(s *Styles) { s.ArgumentName = o.style }

type styleArgumentMarkerOption struct{ style TextStyle }

func (o styleArgumentMarkerOption) ApplyStyle(s *Styles) { s.ArgumentMarker = o.style }

type styleChildNameOption struct{ style TextStyle }

func (o styleChildNameOption) ApplyStyle(s *Styles) { s.ChildName = o.style }

type styleChildDescOption struct{ style TextStyle }

func (o styleChildDescOption) ApplyStyle(s *Styles) { s.ChildDesc = o.style }

type styleExampleOption struct{ style TextStyle }

func (o styleExampleOption) ApplyStyle(s *Styles) { s.Example = o.style }
