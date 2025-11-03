# Documentation Assets

This folder contains animated recordings (WebP format) that demonstrate CLIX features in action.

## File Naming Convention

Animations are named using the pattern: `{document}_{sequence}.webp`

Examples:
- `validation_0.webp` - First animation in validation.md
- `validation_1.webp` - Second animation in validation.md
- `terminal_prompts_0.webp` - First animation in terminal_prompts.md

## Recording Guidelines

### Tools

Recommended tools for recording terminal animations:

1. **asciinema + agg** (Recommended)
   ```bash
   # Record
   asciinema rec demo.cast
   
   # Convert to GIF
   agg demo.cast demo.gif
   
   # Convert GIF to WebP (using ffmpeg or ImageMagick)
   ffmpeg -i demo.gif demo.webp
   ```

2. **terminalizer** (Alternative)
   ```bash
   terminalizer record demo
   terminalizer render demo -o demo.gif
   ffmpeg -i demo.gif demo.webp
   ```

3. **ttygif** (Simple, for GIFs)
   ```bash
   ttygif recording.log
   ffmpeg -i tty.gif recording.webp
   ```

### Best Practices

1. **Keep recordings short**: 10-30 seconds maximum
2. **Use consistent terminal size**: 80x24 or 100x30
3. **Clear terminal before recording**: Use `clear` or `reset`
4. **Show the command being typed**: Type slowly and deliberately
5. **Highlight key interactions**: Show important prompts, selections, etc.
6. **Optimize file size**: Use WebP compression to keep files under 1MB when possible

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

After creating a GIF:

```bash
# Using ffmpeg
ffmpeg -i input.gif -vcodec libwebp -quality 80 -loop 0 output.webp

# Using ImageMagick
convert input.gif -quality 80 output.webp
```

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

