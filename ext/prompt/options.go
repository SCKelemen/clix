package prompt

import "github.com/SCKelemen/clix/v2"

// Select creates a select prompt option.
// This option is only valid when using the prompt extension (TerminalPrompter).
// Usage:
//
//	prompter.Prompt(ctx, clix.WithLabel("Choose"), Select([]clix.SelectOption{...}))
func Select(options []clix.SelectOption) clix.PromptOption {
	return selectOption{options: options}
}

// MultiSelect creates a multi-select prompt option.
// This option is only valid when using the prompt extension (TerminalPrompter).
// Usage:
//
//	prompter.Prompt(ctx, clix.WithLabel("Select"), MultiSelect([]clix.SelectOption{...}))
func MultiSelect(options []clix.SelectOption) clix.PromptOption {
	return multiSelectOption{options: options}
}

// Confirm creates a confirmation prompt option.
// Note: This is now also available in core as clix.WithConfirm(), but kept here
// for backward compatibility and as a convenience alias.
// Usage:
//
//	prompter.Prompt(ctx, clix.WithLabel("Continue?"), Confirm())
//	// or use clix.WithConfirm() directly
func Confirm() clix.PromptOption {
	return clix.WithConfirm()
}

// WithContinueText sets the text shown for the continue action in multi-select prompts.
// Usage:
//
//	prompter.Prompt(ctx, clix.WithLabel("Select"), MultiSelect([]clix.SelectOption{...}), WithContinueText("Finish"))
func WithContinueText(text string) clix.PromptOption {
	return continueTextOption{text: text}
}

// selectOption implements PromptOption for select prompts.
type selectOption struct {
	options []clix.SelectOption
}

func (o selectOption) Apply(cfg *clix.PromptConfig) {
	cfg.Options = o.options
}

// multiSelectOption implements PromptOption for multi-select prompts.
type multiSelectOption struct {
	options []clix.SelectOption
}

func (o multiSelectOption) Apply(cfg *clix.PromptConfig) {
	cfg.Options = o.options
	cfg.MultiSelect = true
}

// confirmOption implements PromptOption for confirmation prompts.
type confirmOption struct{}

func (o confirmOption) Apply(cfg *clix.PromptConfig) {
	cfg.Confirm = true
}

// continueTextOption implements PromptOption for setting continue text.
type continueTextOption struct {
	text string
}

func (o continueTextOption) Apply(cfg *clix.PromptConfig) {
	cfg.ContinueText = o.text
}
