# CLIX Documentation

Welcome to the CLIX documentation! This guide provides a progressive introduction to building CLI applications with CLIX, from simple commands to advanced interactive features.

## Table of Contents

1. [Commands](tutorial/1_commands/README.md) - Building your first CLI application
1.5. [Styling with Lipgloss](tutorial/1.5_styling/README.md) - Beautiful terminal styling
2. [Arguments](tutorial/2_arguments/README.md) - Handling command arguments
3. [Flags](tutorial/3_flags/README.md) - Using flags and global flags
4. [Configuration](tutorial/4_config/README.md) - Configuration system (flags, env vars, files)
5. [Help System](tutorial/5_help/README.md) - Built-in help rendering
6. [Text Prompts](tutorial/6_text_prompts/README.md) - Basic interactive prompting
7. [Validation](tutorial/7_validation/README.md) - Input validation
8. [Terminal Prompts](tutorial/8_terminal_prompts/README.md) - Advanced prompts (select, multi-select, confirm)
9. [Surveys](tutorial/9_surveys/README.md) - Chaining prompts together
10. [Extensions](tutorial/10_extensions/README.md) - Extension system architecture

## How to Use This Documentation

This documentation is designed to be read sequentially. Each chapter builds on the previous ones:

- **Start with Commands** to understand the basic structure
- **Progress through Arguments and Flags** to handle user input
- **Learn about Configuration** to make your CLI configurable
- **Explore Prompts** starting with simple text, then validation, then advanced features
- **Discover Surveys** to create complex interactive flows
- **Understand Extensions** to customize and extend CLIX

## Running Examples

Each tutorial section includes working code examples. To run an example:

```bash
cd docs/tutorial/1_commands
go run main.go
```

Each tutorial directory contains:
- `README.md` - The documentation
- `main.go` - The example code
- `go.mod` - Go module configuration

## Prerequisites

- Go 1.21 or later
- Basic understanding of Go programming
- Familiarity with command-line interfaces

Let's get started with [Commands](tutorial/1_commands/README.md)!
