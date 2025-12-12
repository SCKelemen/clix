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
