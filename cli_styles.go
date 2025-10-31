package clix

// Styles defines styling hooks for textual CLI output such as help screens.
type Styles struct {
	AppTitle       TextStyle
	AppDescription TextStyle
	CommandTitle   TextStyle
	SectionHeading TextStyle
	Usage          TextStyle
	FlagName       TextStyle
	FlagUsage      TextStyle
	ArgumentName   TextStyle
	ArgumentMarker TextStyle
	SubcommandName TextStyle
	SubcommandDesc TextStyle
	Example        TextStyle
}

// DefaultStyles leaves all styles unset, producing plain text output.
var DefaultStyles = Styles{}
