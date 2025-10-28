package clix

import "strings"

// Argument describes a positional argument for a command.
type Argument struct {
	Name     string
	Prompt   string
	Default  string
	Required bool
	Validate func(string) error
}

// PromptLabel returns the prompt to display for this argument.
func (a *Argument) PromptLabel() string {
	if a.Prompt != "" {
		return a.Prompt
	}
	if a.Name != "" {
		return strings.Title(strings.ReplaceAll(a.Name, "-", " "))
	}
	return "Value"
}
