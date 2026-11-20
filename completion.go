package clix

import "context"

// Completion is one candidate offered by shell completion for the word
// being completed.
type Completion struct {
	// Value is the text inserted on the command line.
	Value string

	// Description is shown next to the value by shells that support it
	// (zsh, fish and PowerShell).
	Description string
}

// CompletionRequest describes the word a CompletionFunc is asked to
// complete.
type CompletionRequest struct {
	// Context is the context of the completion run.
	Context context.Context

	// App is the application being completed.
	App *App

	// Command is the command the word belongs to.
	Command *Command

	// Flag is the flag or positional argument whose value is being
	// completed.
	Flag *Flag

	// Args holds the positional arguments already typed for Command, before
	// the word being completed.
	Args []string

	// ToComplete is the part of the word typed so far.
	ToComplete string
}

// CompletionFunc returns the candidate values for a flag or positional
// argument. It may return every value: candidates that do not start with
// req.ToComplete are dropped by the caller.
type CompletionFunc func(req CompletionRequest) []Completion

// SetCompletion registers fn to complete the values of the named flag or
// positional argument in shell completion (see the autocomplete
// extension). It reports whether fs has a flag with that name.
//
// Example:
//
//	cmd.Flags.SetCompletion("region", clix.CompleteValues("eu-west-1", "us-east-1"))
func (fs *FlagSet) SetCompletion(name string, fn CompletionFunc) bool {
	f := fs.lookup(name)
	if f == nil {
		return false
	}
	f.completion = fn
	return true
}

// Completer returns the completion function registered for f with
// SetCompletion, or nil. TimeVar and ISODurationVar flags start with one
// that offers nothing, so shell completion suggests no file names for
// them.
func (f *Flag) Completer() CompletionFunc {
	if f == nil {
		return nil
	}
	return f.completion
}

// CompleteValues returns a CompletionFunc that offers the given values.
func CompleteValues(values ...string) CompletionFunc {
	return func(CompletionRequest) []Completion {
		out := make([]Completion, len(values))
		for i, v := range values {
			out[i] = Completion{Value: v}
		}
		return out
	}
}

// CompleteOptions returns a CompletionFunc that offers the values of
// select-style options, the same list a select prompt would show. Each
// option's Label (and Description, when set) becomes the description.
func CompleteOptions(options ...SelectOption) CompletionFunc {
	return func(CompletionRequest) []Completion {
		out := make([]Completion, 0, len(options))
		for _, o := range options {
			value := o.Value
			if value == "" {
				value = o.Label
			}
			desc := o.Label
			if desc == value {
				desc = ""
			}
			if o.Description != "" {
				if desc != "" {
					desc += " - " + o.Description
				} else {
					desc = o.Description
				}
			}
			out = append(out, Completion{Value: value, Description: desc})
		}
		return out
	}
}
