# Recording Checklist

This file lists all animations that need to be recorded for the documentation.

## Installation

First, install the required tools:

```bash
make -C docs/assets install-tools
# or manually:
brew install asciinema agg ffmpeg
```

## Recording Workflow

For each animation, follow these steps:

1. **Navigate to the example directory** (if needed, create simple test files)
2. **Start recording**: `./record.sh <name>` or `make record NAME=<name>`
3. **Type commands slowly and clearly**
4. **Press Ctrl+D when finished**
5. **Verify the output file** was created in `docs/assets/`

## Animations Needed

### 1. Commands (`1_commands.md`)

#### commands_0.webp - Basic command execution
**What to show:**
- Simple "Hello, World!" command
- Terminal shows: `$ go run main.go` → `Hello, World!`

**Commands:**
```bash
cd /path/to/example
clear
go run main.go
```

#### commands_1.webp - Subcommands demonstration
**What to show:**
- Root command shows help
- `greet hello` command execution
- `greet goodbye` command execution

**Commands:**
```bash
clear
go run main.go
go run main.go hello
go run main.go goodbye
```

#### commands_2.webp - Automatic help display
**What to show:**
- Invalid subcommand triggers help
- Help shows available subcommands

**Commands:**
```bash
clear
go run main.go invalid
```

---

### 2. Arguments (`2_arguments.md`)

#### arguments_0.webp - Interactive argument prompting
**What to show:**
- Command without argument triggers prompt
- User types "Alice"
- Command executes with "Hello, Alice!"

**Commands:**
```bash
clear
go run main.go greet
# Type: Alice
```

#### arguments_1.webp - Multiple arguments
**What to show:**
- Command with two arguments
- Shows "Hello, John Doe!"

**Commands:**
```bash
clear
go run main.go greet John Doe
```

---

### 3. Flags (`3_flags.md`)

#### flags_0.webp - Flag usage demonstration
**What to show:**
- Using long flags: `--name Alice --age 30`
- Using short flags: `-n Bob -a 25`

**Commands:**
```bash
clear
go run main.go greet --name Alice --age 30
go run main.go greet -n Bob -a 25
```

---

### 4. Config (`4_config.md`)

#### config_0.webp - Configuration precedence
**What to show:**
- Flag overrides environment variable
- Environment variable is used when flag not provided

**Commands:**
```bash
clear
export MYAPP_API_KEY=env-value
export MYAPP_PORT=7000
go run main.go server --api-key flag-override
go run main.go server
```

---

### 5. Text Prompts (`6_text_prompts.md`)

#### text_prompts_0.webp - Basic text prompt
**What to show:**
- Prompt appears: "What is your name?:"
- User types "Alice"
- Output: "Hello, Alice!"

**Commands:**
```bash
clear
go run main.go greet
# Type: Alice
```

#### text_prompts_1.webp - Default values in prompts
**What to show:**
- Prompt shows: "Port number [8080]:"
- User presses Enter (accepts default)
- Shows using default value

**Commands:**
```bash
clear
go run main.go
# Just press Enter when prompted
```

#### text_prompts_2.webp - Confirmation prompt
**What to show:**
- Prompt: "Continue? [Y/n]:"
- User types "y"
- Shows "Proceeding..."

**Commands:**
```bash
clear
go run main.go
# Type: y
```

---

### 6. Validation (`7_validation.md`)

#### validation_0.webp - Argument validation
**What to show:**
- Valid input: `age 25` → success
- Invalid input: `age abc` → error message
- Another invalid: `age -5` → error message

**Commands:**
```bash
clear
go run main.go age 25
go run main.go age abc
go run main.go age -5
```

#### validation_1.webp - Prompt validation with re-prompting
**What to show:**
- User types "invalid" → error shown
- Prompt re-appears
- User types "user@example.com" → success

**Commands:**
```bash
clear
go run main.go
# Type: invalid
# Wait for error, then type: user@example.com
```

---

### 7. Terminal Prompts (`8_terminal_prompts.md`)

#### terminal_prompts_0.webp - Select prompt with arrow navigation
**What to show:**
- Select prompt appears with 3 options
- Use arrow keys to navigate (show cursor moving)
- Press Enter to select

**Setup:** Need terminal prompt extension enabled

**Commands:**
```bash
clear
go run main.go
# Use arrow keys to navigate, Enter to select
```

#### terminal_prompts_1.webp - Multi-select prompt
**What to show:**
- Multi-select prompt with checkboxes
- Use Space to toggle items
- Navigate to Continue button
- Press Enter to finish

**Setup:** Need terminal prompt extension enabled

**Commands:**
```bash
clear
go run main.go
# Use arrow keys, Space to toggle, Enter on Continue
```

#### terminal_prompts_2.webp - Tab completion in text input
**What to show:**
- Text prompt with default "John Doe"
- Start typing "John D"
- Press Tab to complete to "John Doe"

**Setup:** Need terminal prompt extension enabled

**Commands:**
```bash
clear
go run main.go
# Type: John D
# Press Tab
```

---

## Batch Recording

To record all animations, you can use this script:

```bash
#!/bin/bash
cd "$(dirname "$0")"

NAMES=(
    "commands_0" "commands_1" "commands_2"
    "arguments_0" "arguments_1"
    "flags_0"
    "config_0"
    "text_prompts_0" "text_prompts_1" "text_prompts_2"
    "validation_0" "validation_1"
    "terminal_prompts_0" "terminal_prompts_1" "terminal_prompts_2"
)

for name in "${NAMES[@]}"; do
    echo "Recording $name..."
    echo "Press Enter when ready to record, or Ctrl+C to skip..."
    read
    ./record.sh "$name"
done

echo "All animations recorded!"
```

Save as `record_all.sh` and run: `./record_all.sh`

## Tips

1. **Terminal Setup:**
   - Use terminal size: 100 columns × 30 rows
   - Use monospace font (Menlo, Monaco, JetBrains Mono) at 14-16pt
   - Use a clean, high-contrast theme

2. **Recording:**
   - Type slowly and deliberately
   - Wait 1-2 seconds before and after actions
   - Clear terminal before each recording
   - Keep recordings under 30 seconds

3. **Quality:**
   - Scripts automatically use `-quality 100` for maximum quality
   - Animations loop infinitely with `-loop 0`
   - Text should be crisp and readable

4. **Testing:**
   - Open the generated `.webp` file in a browser to verify
   - Check that the loop is seamless
   - Verify text is readable at normal zoom levels

