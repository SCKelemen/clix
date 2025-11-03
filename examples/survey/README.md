# Survey Extension Example

This example demonstrates the `survey` extension with two different surveys:

### Simple Survey (`simple` command)
- Uses `TextPrompter` (core, no extensions required)
- Only text input and confirm prompts
- Demonstrates:
  - Basic survey structure
  - Input validation (name, email)
  - **Autocomplete with defaults** (country field with default "United States")
  - **Tab completion** - Press Tab to auto-complete default values
  - **Suggestions** - Type part of a default value to see suggestions
  - Undo/back functionality
  - End card with styled summary

### Advanced Survey (`advanced` command)
- Uses `TerminalPrompter` (requires `ext/prompt` extension)
- Text input, select, multi-select, and confirm prompts
- Demonstrates:
  - Advanced prompt types (select, multi-select)
  - Input validation (name, email, age, interests)
  - **Autocomplete with defaults** (country, age, language fields)
  - **Tab completion** - Press Tab to auto-complete default values
  - **Suggestions** - Type part of a default value to see inline suggestions
  - Undo/back functionality
  - End card with styled summary
  - Multiple prompt types working together

Both surveys demonstrate:
- **Static survey definitions** using struct-based API
- **Autocomplete features**:
  - Default values that can be accepted by pressing Enter
  - Tab key to auto-complete default values
  - Inline suggestions that appear as you type (if your input matches the start of a default)
- **Undo/back functionality** - Press Escape or F12 to undo previous answers
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

1. **Autocomplete & Defaults**:
   - Set default values with the `Default` field
   - Press **Tab** to auto-complete to the default value
   - Type part of a default value to see inline suggestions appear
   - Press **Enter** with empty input to accept the default
   - Key hints show `[ Tab ] Autocomplete` when a default is available

2. **Undo Stack**: Press **Escape** or **F12** at any prompt to return to the previous question

3. **End Card**: After completing all questions, you'll see a styled summary and be asked to confirm

4. **Styled Output**: Uses lipgloss for beautiful terminal styling

5. **Multiple Prompt Types**: Mixes text, select, multi-select, and confirm prompts

## Simple Survey Flow

1. Enter your name (validated: minimum 2 characters)
2. Enter your email (validated: must be a valid email address)
3. Enter your country (default: "United States")
   - Press **Tab** to auto-complete to "United States"
   - Or type part of the default (e.g., "United") to see suggestions
   - Press **Enter** to accept the default or your typed value
4. Confirm newsletter subscription (yes/no)
5. Review the summary in the end card
6. Confirm or go back to edit answers

## Advanced Survey Flow

1. Enter your name (validated: minimum 2 characters)
2. Enter your email (validated: must be a valid email address)
3. Enter your country (default: "United States")
   - Press **Tab** to auto-complete, or type part of "United States" to see suggestions
4. Enter your age (default: "25", validated: must be a number between 13 and 120)
   - Press **Tab** to auto-complete to "25", or just press **Enter** to accept the default
5. Enter your favorite programming language (default: "Go")
   - Try typing "Go" or just "G" to see the suggestion appear inline
   - Press **Tab** to auto-complete
6. Select multiple interests (validated: must select at least one; use space/enter to toggle, navigate to "Finish" to continue)
7. Select your experience level (use arrow keys, enter to select)
8. Confirm newsletter subscription (yes/no)
9. Review the summary in the end card
10. Confirm or go back to edit answers

Try these autocomplete features:
- Press **Tab** to auto-complete default values
- Type part of a default (e.g., "United" when default is "United States") to see suggestions
- Press **Escape** or **F12** at any prompt to undo and go back to previous questions

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

