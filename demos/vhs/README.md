# VHS Demo Tapes

This directory contains [VHS](https://github.com/charmbracelet/vhs) tape files for creating animated GIFs that demonstrate clix features.

## Prerequisites

Install VHS:

```bash
# macOS
brew install vhs

# Linux
curl -L https://github.com/charmbracelet/vhs/releases/latest/download/vhs_linux_amd64.tar.gz | tar -xz
sudo mv vhs /usr/local/bin/

# Or via Go
go install github.com/charmbracelet/vhs@latest
```

## Generating GIFs

From the `demos/vhs` directory:

```bash
# Generate all GIFs
vhs < basic-usage.tape
vhs < interactive-prompts.tape
vhs < styled-output.tape
vhs < gh-example.tape
vhs < multicli-example.tape
```

Or from the project root:

```bash
cd demos/vhs
for tape in *.tape; do
    vhs < "$tape"
done
```

The GIFs will be generated in the `demos/vhs/` directory (same directory as the tape files).

## Available Demos

### `basic-usage.tape`
Demonstrates:
- Basic command structure
- Help output
- Command execution with arguments
- Interactive prompting for missing arguments

### `interactive-prompts.tape`
Demonstrates:
- Simple survey with text prompts
- Advanced survey with select and multi-select
- Input validation
- End card summary

### `styled-output.tape`
Demonstrates:
- Lipgloss styling integration
- Styled help output
- Format options (JSON, YAML, text)

### `gh-example.tape`
Demonstrates:
- Complex command hierarchy
- Group and command organization
- Version extension

### `multicli-example.tape`
Demonstrates:
- Shared command implementations
- Different CLI structures
- Aliases
- Versioning support

## Customizing

You can customize the tapes by editing the `.tape` files. Key settings:

- `Output`: Path to output GIF file
- `Width` / `Height`: Terminal dimensions
- `Theme`: Color theme (see VHS docs for options)
- `FontSize`: Terminal font size
- `Shell`: Shell to use

See [VHS documentation](https://github.com/charmbracelet/vhs) for more options.

## Adding to README

After generating GIFs, add them to the README:

```markdown
## Demo

![Basic Usage](demos/basic-usage.gif)
```

Make sure to commit the GIF files to the repository so they display on GitHub.

