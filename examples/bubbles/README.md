# Bubbles Integration Example

This example demonstrates how to integrate [Bubbles](https://github.com/charmbracelet/bubbles) components with clix's prompter system.

## Overview

The example shows how to create a custom `clix.Prompter` implementation using Bubbles components:
- **textinput**: For text input prompts
- **list**: For select and multi-select prompts

## Key Features

1. **Custom Prompter**: `BubblesPrompter` implements `clix.Prompter` using Bubbles components
2. **Text Input**: Uses `bubbles/textinput` for text prompts
3. **Select**: Uses `bubbles/list` for single-select prompts
4. **Multi-Select**: Uses `bubbles/list` with custom selection tracking for multi-select
5. **Confirm**: Uses `bubbles/textinput` for yes/no prompts

## Usage

```bash
cd examples/bubbles
go run ./cmd/demo greet
```

The example will prompt you for:
1. Your name (text input)
2. A greeting type (select from list)
3. Confirmation (yes/no)

## Implementation Details

The `BubblesPrompter` wraps Bubbles components in `bubbletea` models to integrate with clix's prompt system. Each prompt type (text, select, multi-select, confirm) uses the appropriate Bubbles component while maintaining compatibility with clix's `PromptRequest` API.

This demonstrates how you can:
- Use Bubbles components for rich terminal UI
- Maintain compatibility with clix's standard prompt API
- Customize the prompt experience while keeping clix's ergonomics

## Dependencies

- `github.com/charmbracelet/bubbles` - Bubbles components
- `github.com/charmbracelet/bubbletea` - Bubble Tea framework
- `github.com/SCKelemen/clix` - clix CLI framework

