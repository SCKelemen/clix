# Animation Recording Guide

This guide walks you through recording animations for the documentation.

## Step 1: Install Tools

Install the required tools on macOS:

```bash
brew install asciinema agg ffmpeg
```

Or on Linux:
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y asciinema agg ffmpeg

# Or via npm
npm install -g asciinema-cli agg
```

Verify installation:
```bash
asciinema --version
agg --version
ffmpeg -version
```

## Step 2: Prepare Terminal

Before recording:

1. **Set terminal size**: 100 columns × 30 rows (or similar consistent size)
2. **Use clean theme**: High contrast, clear fonts (Menlo, Monaco, JetBrains Mono at 14-16pt)
3. **Clear terminal**: Use `clear` before each recording
4. **Minimal prompt** (optional): The recording script automatically sets a minimal prompt (`$ `) to hide username and hostname. If you prefer a custom prompt, you can override it by setting `PS1` (bash) or `PROMPT` (zsh) before running the script.

## Step 3: Record an Animation

### Using the Script (Recommended)

1. Navigate to the assets directory:
   ```bash
   cd docs/assets
   ```

2. Start recording:
   ```bash
   ./record.sh commands_0
   ```
   This will:
   - Start asciinema recording
   - You'll see a prompt: "Type your commands, then press Ctrl+D when finished"
   - Type your commands in the terminal
   - Press **Ctrl+D** when done

3. The script automatically:
   - Converts `.cast` to `.gif` with agg
   - Converts `.gif` to `.webp` with maximum quality (100) and infinite looping
   - Cleans up intermediate files
   - Creates `commands_0.webp` in `docs/assets/`

### Manual Recording (Alternative)

If you prefer to do it manually:

```bash
# 1. Record
asciinema rec animation.cast

# 2. Convert to GIF
agg animation.cast animation.gif --theme asciinema

# 3. Convert to WebP with max quality and looping
ffmpeg -i animation.gif -vcodec libwebp -quality 100 -loop 0 animation.webp

# 4. Clean up
rm animation.cast animation.gif

# 5. Rename to match naming convention
mv animation.webp commands_0.webp
```

## Step 4: Recording Each Animation

### commands_0.webp - Basic command execution

```bash
cd docs/code/commands/example1_basic
clear
go run main.go
# Press Ctrl+D after output appears
```

### commands_1.webp - Subcommands demonstration

```bash
cd docs/code/commands/example2_subcommands
clear
go run main.go
go run main.go hello
go run main.go goodbye
# Press Ctrl+D
```

### commands_2.webp - Automatic help display

```bash
cd docs/code/commands/example3_help
clear
go run main.go invalid
# Press Ctrl+D
```

### arguments_0.webp - Interactive argument prompting

```bash
cd docs/code/arguments/example1_basic
clear
go run main.go greet
# Type: Alice
# Press Enter
# Press Ctrl+D
```

### arguments_1.webp - Multiple arguments

```bash
cd docs/code/arguments/example2_multiple
clear
go run main.go greet John Doe
# Press Ctrl+D
```

### flags_0.webp - Flag usage demonstration

```bash
cd docs/code/flags/example1_basic
clear
go run main.go greet --name Alice --age 30
go run main.go greet -n Bob -a 25
# Press Ctrl+D
```

### config_0.webp - Configuration precedence

```bash
cd docs/code/config/example1_precedence
clear
export MYAPP_API_KEY=env-value
export MYAPP_PORT=7000
go run main.go server --api-key flag-override
go run main.go server
# Press Ctrl+D
```

### text_prompts_0.webp - Basic text prompt

```bash
cd docs/code/text_prompts/example1_basic
clear
go run main.go greet
# Type: Alice
# Press Enter
# Press Ctrl+D
```

### text_prompts_1.webp - Default values in prompts

```bash
cd docs/code/text_prompts/example2_default
clear
go run main.go
# Just press Enter (accept default)
# Press Ctrl+D
```

### text_prompts_2.webp - Confirmation prompt

```bash
cd docs/code/text_prompts/example3_confirm
clear
go run main.go
# Type: y
# Press Enter
# Press Ctrl+D
```

### validation_0.webp - Argument validation

```bash
cd docs/code/validation/example1_argument
clear
go run main.go age 25
go run main.go age abc
go run main.go age -5
# Press Ctrl+D
```

### validation_1.webp - Prompt validation with re-prompting

```bash
cd docs/code/validation/example2_prompt
clear
go run main.go
# Type: invalid
# Wait for error, then type: user@example.com
# Press Enter
# Press Ctrl+D
```

### terminal_prompts_0.webp - Select prompt with arrow navigation

```bash
cd docs/code/terminal_prompts/example1_select
clear
go run main.go
# Use arrow keys (↑ ↓) to navigate
# Press Enter to select
# Press Ctrl+D
```

### terminal_prompts_1.webp - Multi-select prompt

```bash
cd docs/code/terminal_prompts/example2_multiselect
clear
go run main.go
# Use arrow keys to navigate
# Press Space to toggle items
# Navigate to Continue button
# Press Enter to finish
# Press Ctrl+D
```

### terminal_prompts_2.webp - Tab completion in text input

```bash
cd docs/code/terminal_prompts/example3_tab_completion
clear
go run main.go
# Type: John D
# Press Tab (should complete to "John Doe")
# Press Enter
# Press Ctrl+D
```

## Tips for Good Recordings

1. **Type slowly**: Make it easy to see what's being typed
2. **Wait between actions**: Pause 1-2 seconds after output appears
3. **Show the full flow**: Include the command being typed, not just the output
4. **Clear terminal**: Start fresh with `clear` before each recording
5. **Consistent size**: Keep terminal window the same size for all recordings
6. **Test first**: Run the example before recording to ensure it works

## Batch Recording

To record all animations in sequence, you can use this approach:

```bash
cd docs/assets

# Record each one
./record.sh commands_0
./record.sh commands_1
./record.sh commands_2
# ... etc
```

Or create a simple loop:

```bash
cd docs/assets

for name in commands_0 commands_1 commands_2 arguments_0 arguments_1 flags_0 config_0 \
            text_prompts_0 text_prompts_1 text_prompts_2 validation_0 validation_1 \
            terminal_prompts_0 terminal_prompts_1 terminal_prompts_2; do
    echo "Recording $name..."
    echo "Press Enter to start recording, or Ctrl+C to skip..."
    read
    ./record.sh "$name"
done
```

## Verifying Recordings

After recording, verify the file was created:

```bash
ls -lh docs/assets/*.webp
```

Open the `.webp` file in a browser or image viewer to check:
- Text is crisp and readable
- Animation loops smoothly
- Duration is appropriate (10-30 seconds)
- Content matches the intended demonstration

## File Size

The recordings should be reasonably sized (typically 500KB - 2MB). If files are too large:
- Reduce the terminal window size
- Shorten the recording duration
- Remove unnecessary pauses

The `-quality 100` setting ensures maximum quality, but if files are too large, you can adjust the quality in `record.sh` (though text clarity should remain priority).

## Troubleshooting

**"agg: command not found"**
- Install agg: `brew install agg` or `npm install -g agg`

**"ffmpeg: command not found"**
- Install ffmpeg: `brew install ffmpeg`

**Animation doesn't loop**
- Check the `-loop 0` flag is in the ffmpeg command

**Text is blurry**
- Ensure `-quality 100` is used
- Check terminal font size (14-16pt recommended)

**Recording doesn't capture keystrokes**
- Make sure you're typing in the terminal where asciinema is running
- Some terminal apps may need special settings for raw mode

