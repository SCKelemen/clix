# Documentation Code Examples

This directory contains standalone, runnable code examples for each documentation section.

## Structure

```
code/
├── commands/          # Examples for 1_commands.md
├── arguments/          # Examples for 2_arguments.md
├── flags/             # Examples for 3_flags.md
├── config/            # Examples for 4_config.md
├── help/              # Examples for 5_help.md
├── text_prompts/      # Examples for 6_text_prompts.md
├── validation/        # Examples for 7_validation.md
├── terminal_prompts/  # Examples for 8_terminal_prompts.md
└── surveys/           # Examples for 9_surveys.md
```

## Building and Running

Each example is a standalone Go program in its own directory. To build and run:

```bash
# Navigate to the example directory
cd docs/code/commands/example1_basic

# Build the example
go build

# Run it
./example1_basic

# Or run directly
go run main.go

# From the repo root, you can use:
go run ./docs/code/commands/example1_basic
```

## Examples Index

### Commands (`commands/`)
- `example1_basic/` - Basic command execution
- `example2_subcommands/` - Subcommands demonstration
- `example3_help/` - Automatic help display

### Arguments (`arguments/`)
- `example1_basic/` - Basic argument with prompting
- `example2_multiple/` - Multiple arguments

### Flags (`flags/`)
- `example1_basic/` - Basic flag usage

### Config (`config/`)
- `example1_precedence/` - Configuration precedence demonstration

### Text Prompts (`text_prompts/`)
- `example1_basic/` - Basic text prompt
- `example2_default/` - Default values in prompts
- `example3_confirm/` - Confirmation prompt

### Validation (`validation/`)
- `example1_argument/` - Argument validation
- `example2_prompt/` - Prompt validation with re-prompting

### Terminal Prompts (`terminal_prompts/`)
- `example1_select/` - Select prompt with arrow navigation
- `example2_multiselect/` - Multi-select prompt
- `example3_tab_completion/` - Tab completion in text input

## Recording Animations

Use these examples to record animations for the documentation:

1. Navigate to the example directory (e.g., `cd commands/example1_basic`)
2. Build/run the example to verify it works:
   ```bash
   go run main.go
   ```
3. Use the recording tools in `../../assets/`:
   ```bash
   cd ../../assets
   ./record.sh <animation_name>
   ```

See `../assets/RECORDING_CHECKLIST.md` for specific commands to run for each animation.

## Setup

Before running examples, set up their `go.mod` files:

```bash
cd docs/code
make setup
```

This creates a `go.mod` file in each example directory with the correct `replace` directive to point to the clix repository.

## Notes

- Each example has its own `go.mod` file with a replace directive
- Examples can be run independently from their directories
- Some examples require extensions (like `terminal_prompts/`) - they will show how to enable them
- Examples are minimal and focused on demonstrating specific features

