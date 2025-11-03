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

Each example is a standalone Go program. To build and run:

```bash
# Navigate to the example directory
cd docs/code/commands

# Build the example
go build example1_basic.go

# Run it
./example1_basic

# Or run directly
go run example1_basic.go
```

## Examples Index

### Commands (`commands/`)
- `example1_basic.go` - Basic command execution
- `example2_subcommands.go` - Subcommands demonstration
- `example3_help.go` - Automatic help display

### Arguments (`arguments/`)
- `example1_basic.go` - Basic argument with prompting
- `example2_multiple.go` - Multiple arguments

### Flags (`flags/`)
- `example1_basic.go` - Basic flag usage

### Config (`config/`)
- `example1_precedence.go` - Configuration precedence demonstration

### Text Prompts (`text_prompts/`)
- `example1_basic.go` - Basic text prompt
- `example2_default.go` - Default values in prompts
- `example3_confirm.go` - Confirmation prompt

### Validation (`validation/`)
- `example1_argument.go` - Argument validation
- `example2_prompt.go` - Prompt validation with re-prompting

### Terminal Prompts (`terminal_prompts/`)
- `example1_select.go` - Select prompt with arrow navigation
- `example2_multiselect.go` - Multi-select prompt
- `example3_tab_completion.go` - Tab completion in text input

## Recording Animations

Use these examples to record animations for the documentation:

1. Navigate to the example directory
2. Build/run the example to verify it works
3. Use the recording tools in `../assets/`:
   ```bash
   cd ../assets
   ./record.sh <animation_name>
   ```

See `../assets/RECORDING_CHECKLIST.md` for specific commands to run for each animation.

## Notes

- All examples use relative imports assuming they're run from within the clix repository
- Some examples require extensions (like `terminal_prompts/`) - they will show how to enable them
- Examples are minimal and focused on demonstrating specific features

