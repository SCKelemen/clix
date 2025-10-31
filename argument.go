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

// parseNamedArguments parses positional arguments that may include named
// parameters in the form key=value. Returns a map of argument names to values
// and a slice of positional arguments in order.
func parseNamedArguments(args []string, commandArgs []*Argument) (map[string]string, []string) {
	named := make(map[string]string)
	positional := make([]string, 0)
	
	// Build a map of argument names for quick lookup
	argNames := make(map[string]*Argument)
	for _, arg := range commandArgs {
		if arg.Name != "" {
			argNames[arg.Name] = arg
			// Also support with hyphens converted
			argNames[strings.ReplaceAll(arg.Name, "-", "_")] = arg
		}
	}

	for _, arg := range args {
		// Check if this is a named parameter (key=value format)
		if key, value, ok := strings.Cut(arg, "="); ok && !strings.HasPrefix(key, "-") {
			// Normalize key name (handle both hyphens and underscores)
			normalizedKey := strings.ReplaceAll(key, "-", "_")
			if cmdArg, exists := argNames[normalizedKey]; exists {
				// Use the original argument name from the definition
				named[cmdArg.Name] = value
				continue
			}
			// If not recognized, treat as positional
		}
		// Not a named parameter or not recognized, treat as positional
		positional = append(positional, arg)
	}

	return named, positional
}
