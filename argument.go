package clix

import "strings"

// ArgumentOption configures an argument using the functional options pattern.
// Options can be used to build arguments:
//
//	// Using functional options
//	cmd.Arguments = []*clix.Argument{
//		clix.NewArgument(
//			WithArgName("name"),
//			WithArgPrompt("Enter your name"),
//			WithArgRequired(),
//			WithArgValidate(validation.NotEmpty),
//		),
//	}
//
//	// Using struct (primary API)
//	cmd.Arguments = []*clix.Argument{{
//		Name:     "name",
//		Prompt:   "Enter your name",
//		Required: true,
//		Validate: validation.NotEmpty,
//	}}
type ArgumentOption interface {
	// ApplyArgument configures an argument struct.
	// Exported so extension packages can implement ArgumentOption.
	ApplyArgument(*Argument)
}

// Argument describes a positional argument for a command.
// This struct implements ArgumentOption, so it can be used alongside functional options.
//
// Example:
//
//	// Struct-based (primary API)
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
//
//	// Functional options
//	cmd.Arguments = []*clix.Argument{
//		clix.NewArgument(
//			WithArgName("name"),
//			WithArgPrompt("Enter your name"),
//			WithArgRequired(),
//			WithArgValidate(validation.NotEmpty),
//		),
//		clix.NewArgument(
//			WithArgName("email"),
//			WithArgPrompt("Enter your email"),
//			WithArgDefault("user@example.com"),
//			WithArgValidate(validation.Email),
//		),
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

// ApplyArgument implements ArgumentOption so Argument can be used directly.
func (a *Argument) ApplyArgument(arg *Argument) {
	if a.Name != "" {
		arg.Name = a.Name
	}
	if a.Prompt != "" {
		arg.Prompt = a.Prompt
	}
	if a.Default != "" {
		arg.Default = a.Default
	}
	if a.Required {
		arg.Required = true
	}
	if a.Validate != nil {
		arg.Validate = a.Validate
	}
}

// NewArgument creates a new Argument using functional options.
// Supports three API styles:
//
//	// 1. Struct-based (primary API)
//	arg := &clix.Argument{
//		Name:     "name",
//		Prompt:   "Enter your name",
//		Required: true,
//	}
//
//	// 2. Functional options
//	arg := clix.NewArgument(
//		WithArgName("name"),
//		WithArgPrompt("Enter your name"),
//		WithArgRequired(),
//	)
//
//	// 3. Builder-style (fluent API)
//	arg := clix.NewArgument().
//		SetName("name").
//		SetPrompt("Enter your name").
//		SetRequired()
func NewArgument(opts ...ArgumentOption) *Argument {
	arg := &Argument{}
	for _, opt := range opts {
		opt.ApplyArgument(arg)
	}
	return arg
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

// Builder-style methods for fluent API (similar to lipgloss styles)
// These methods allow method chaining for a more fluent API:
//
//	cmd.Arguments = []*clix.Argument{
//		clix.NewArgument().
//			SetName("name").
//			SetPrompt("Enter your name").
//			SetRequired().
//			SetValidate(validation.NotEmpty),
//	}
//
// Note: These are convenience methods. The struct fields can still be set directly.

// SetName sets the argument name and returns the argument for method chaining.
func (a *Argument) SetName(name string) *Argument {
	a.Name = name
	return a
}

// SetPrompt sets the argument prompt text and returns the argument for method chaining.
func (a *Argument) SetPrompt(prompt string) *Argument {
	a.Prompt = prompt
	return a
}

// SetDefault sets the argument default value and returns the argument for method chaining.
func (a *Argument) SetDefault(defaultValue string) *Argument {
	a.Default = defaultValue
	return a
}

// SetRequired marks the argument as required and returns the argument for method chaining.
func (a *Argument) SetRequired() *Argument {
	a.Required = true
	return a
}

// SetValidate sets the argument validation function and returns the argument for method chaining.
func (a *Argument) SetValidate(validate func(string) error) *Argument {
	a.Validate = validate
	return a
}

// Functional option helpers for arguments

// WithArgName sets the argument name.
func WithArgName(name string) ArgumentOption {
	return argNameOption(name)
}

// WithArgPrompt sets the argument prompt text.
func WithArgPrompt(prompt string) ArgumentOption {
	return argPromptOption(prompt)
}

// WithArgDefault sets the argument default value.
func WithArgDefault(defaultValue string) ArgumentOption {
	return argDefaultOption(defaultValue)
}

// WithArgRequired marks the argument as required.
func WithArgRequired() ArgumentOption {
	return argRequiredOption(true)
}

// WithArgValidate sets the argument validation function.
func WithArgValidate(validate func(string) error) ArgumentOption {
	return argValidateOption{validate: validate}
}

// Internal option types

type argNameOption string

func (o argNameOption) ApplyArgument(arg *Argument) {
	arg.Name = string(o)
}

type argPromptOption string

func (o argPromptOption) ApplyArgument(arg *Argument) {
	arg.Prompt = string(o)
}

type argDefaultOption string

func (o argDefaultOption) ApplyArgument(arg *Argument) {
	arg.Default = string(o)
}

type argRequiredOption bool

func (o argRequiredOption) ApplyArgument(arg *Argument) {
	arg.Required = bool(o)
}

type argValidateOption struct {
	validate func(string) error
}

func (o argValidateOption) ApplyArgument(arg *Argument) {
	arg.Validate = o.validate
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
