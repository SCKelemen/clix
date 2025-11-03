# Documentation Assets

This folder contains animated recordings (WebP format) that demonstrate CLIX features in action.

## File Naming Convention

Animations are named using the pattern: `{document}_{sequence}.webp`

Examples:
- `validation_0.webp` - First animation in validation.md
- `validation_1.webp` - Second animation in validation.md
- `terminal_prompts_0.webp` - First animation in terminal_prompts.md

## Quick Start

If you have the tools installed, you can use the helper scripts:

```bash
# Record a new animation
./record.sh commands_0

# Or use make
make record NAME=commands_0

# Convert existing GIF to WebP
./convert_to_webp.sh animation.gif

# Or use make
make convert FILE=animation.gif
```

The scripts automatically:
- Record with asciinema
- Convert to GIF with agg
- Convert to WebP with maximum quality (100) and infinite looping
- Clean up intermediate files

## Recording Guidelines

### Tools

Recommended tools for recording terminal animations:

1. **asciinema + agg** (Recommended - Best quality)
   ```bash
   # Install
   brew install asciinema agg  # macOS
   # or
   npm install -g asciinema agg  # via npm
   
   # Record (type your commands, then Ctrl+D to finish)
   asciinema rec demo.cast
   
   # Convert to GIF with high quality
   agg demo.cast demo.gif --theme asciinema
   
   # Convert GIF to WebP with maximum quality and looping
   ffmpeg -i demo.gif -vcodec libwebp -quality 100 -loop 0 -preset default demo.webp
   ```

2. **terminalizer** (Alternative)
   ```bash
   # Install
   npm install -g terminalizer
   
   # Record (create config, then record)
   terminalizer record demo
   # Edit demo.yml to set loop: true
   
   # Render with high quality
   terminalizer render demo -o demo.gif
   
   # Convert to WebP with maximum quality and looping
   ffmpeg -i demo.gif -vcodec libwebp -quality 100 -loop 0 demo.webp
   ```

3. **vhs** (Modern alternative - generates high-quality videos)
   ```bash
   # Install
   brew install vhs  # macOS
   
   # Create .tape file with commands, then:
   vhs demo.tape  # Generates demo.gif directly
   
   # Convert to WebP with maximum quality and looping
   ffmpeg -i demo.gif -vcodec libwebp -quality 100 -loop 0 demo.webp
   ```

### Best Practices

1. **Keep recordings short**: 10-30 seconds maximum
2. **Use consistent terminal size**: 80x24 or 100x30
3. **Clear terminal before recording**: Use `clear` or `reset`
4. **Show the command being typed**: Type slowly and deliberately
5. **Highlight key interactions**: Show important prompts, selections, etc.
6. **Maximum quality**: Use `-quality 100` for WebP to ensure text is crisp and readable
7. **Always loop**: Use `-loop 0` flag to create seamless looping animations
8. **Font clarity**: Use monospace fonts like Menlo, Monaco, JetBrains Mono, or Fira Code at 14-16pt

### Terminal Setup

For consistent recordings:

```bash
# Use a clean terminal theme
export TERM=xterm-256color

# Set terminal size
# In iTerm2/Terminal.app, set window size to 100x30

# Use clear fonts (e.g., Menlo, Monaco, or JetBrains Mono)
```

### Conversion to WebP

After creating a GIF, convert to WebP with maximum quality and looping:

```bash
# Using ffmpeg (Recommended - best quality)
ffmpeg -i input.gif -vcodec libwebp -quality 100 -loop 0 -preset default output.webp

# Alternative: Using ImageMagick (may have slightly lower quality)
convert input.gif -quality 100 output.webp
```

**Quality settings:**
- `-quality 100`: Maximum quality for crisp text (recommended)
- `-loop 0`: Infinite looping
- `-preset default`: Good balance of quality and encoding speed

## Adding Animations to Documentation

In the markdown files, reference animations like this:

```markdown
### Example Interaction

```bash
$ go run main.go greet
What is your name?: Alice
Hello, Alice!
```

![Interactive prompt demonstration](assets/commands_0.webp)
```

## Current Assets

(Animations will be added here as they are recorded)

