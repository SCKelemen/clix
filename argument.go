package clix

import "strings"

// Argument describes a positional argument for a command.
//
// Example:
//
//	cmd.Arguments = []*clix.Argument{
//		{
//			Name:     "name",
//			Prompt:   "Enter your name",
//			Required: true,
//			Validate: validation.NotEmpty,
//		},
//		{
//			Name:    "email",
//			Prompt:  "Enter your email",
//			Default: "user@example.com",
//			Validate: validation.Email,
//		},
//	}
type Argument struct {
	// Name is the argument name, used for named argument access (key=value format)
	// and for accessing via ctx.ArgNamed(name).
	Name string

	// Prompt is the text shown when prompting for this argument if it's missing.
	// If empty, a default prompt is generated from the Name field.
	Prompt string

	// Default is the default value if the argument is not provided.
	// Only used if Required is false.
	Default string

	// Required indicates whether this argument must be provided.
	// Missing required arguments trigger interactive prompts (if available).
	Required bool

	// Validate is an optional validation function called when the argument is provided.
	// Return an error if the value is invalid.
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
