Output styled.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "zsh"
Env TERM "xterm-256color"
Env COLORTERM "truecolor"

# Ensure NO_COLOR is not set and build
Type "unset NO_COLOR && go build -o styled ./cmd/styled"
Enter
Sleep 2s

# Run styled (prompts for name) - ensure colors are enabled
Type "unset NO_COLOR && ./styled"
Enter
Sleep 2s
# Wait for banner and name prompt to appear, then type name
Type "Test User"
Enter
Sleep 2s
# Command completes and shows output, wait for it to finish
Sleep 1s

# Show help
Type "unset NO_COLOR && ./styled --help"
Enter
Sleep 2s

# Show format option (non-interactive)
Type "unset NO_COLOR && ./styled style --format json"
Enter
Sleep 1s
