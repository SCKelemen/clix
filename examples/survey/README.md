# Survey Extension Example

This example demonstrates the `survey` extension with two different surveys:

### Simple Survey (`simple` command)
- Uses `TextPrompter` (core, no extensions required)
- Only text input and confirm prompts
- Demonstrates:
  - Basic survey structure
  - Input validation (name, email)
  - Undo/back functionality
  - End card with styled summary

### Advanced Survey (`advanced` command)
- Uses `TerminalPrompter` (requires `ext/prompt` extension)
- Text input, select, multi-select, and confirm prompts
- Demonstrates:
  - Advanced prompt types (select, multi-select)
  - Input validation (name, email, age, interests)
  - Undo/back functionality
  - End card with styled summary
  - Multiple prompt types working together

Both surveys demonstrate:
- **Static survey definitions** using struct-based API
- **Undo/back functionality** - type `back` at any prompt to undo previous answers
- **End card with styled summary** - shows a formatted summary of all answers
- **Input validation** using the validation extension

## Building

```bash
cd examples/survey
go build -o demo cmd/demo/main.go
```

## Running

Run the simple survey (text + confirm only, works with TextPrompter):
```bash
./demo simple
```

Run the advanced survey (text + select + multiselect + confirm, requires TerminalPrompter):
```bash
./demo advanced
```

## Features Demonstrated

1. **Undo Stack**: Type `back` at any prompt to return to the previous question
2. **End Card**: After completing all questions, you'll see a styled summary and be asked to confirm
3. **Styled Output**: Uses lipgloss for beautiful terminal styling
4. **Multiple Prompt Types**: Mixes text, select, multi-select, and confirm prompts

## Simple Survey Flow

1. Enter your name (validated: minimum 2 characters)
2. Enter your email (validated: must be a valid email address)
3. Confirm newsletter subscription (yes/no)
4. Review the summary in the end card
5. Confirm or go back to edit answers

## Advanced Survey Flow

1. Enter your name (validated: minimum 2 characters)
2. Enter your email (validated: must be a valid email address)
3. Enter your age (validated: must be a number between 13 and 120)
4. Select multiple interests (validated: must select at least one; use space/enter to toggle, navigate to "Finish" to continue)
5. Select your experience level (use arrow keys, enter to select)
6. Confirm newsletter subscription (yes/no)
7. Review the summary in the end card
8. Confirm or go back to edit answers

Try typing `back` at any prompt to see the undo functionality in action!

## Validation Features

**Simple Survey:**
- **Name**: Must be at least 2 characters and not empty
- **Email**: Must be a valid RFC-compliant email address

**Advanced Survey:**
- **Name**: Must be at least 2 characters and not empty
- **Email**: Must be a valid RFC-compliant email address
- **Age**: Must be a number between 13 and 120
- **Interests**: Must select at least one interest

Invalid inputs will show an error message and prompt you to try again.

## Differences

- **Simple survey**: Works with basic `TextPrompter` (core feature, no extensions needed)
- **Advanced survey**: Requires `TerminalPrompter` (from `ext/prompt` extension) for select/multiselect prompts

