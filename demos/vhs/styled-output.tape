Output styled-output.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"

# Navigate to styled example
Type "cd examples/lipgloss"
Enter
Sleep 500ms

# Build the demo
Type "go build -o styled ./cmd/styled"
Enter
Sleep 2s

# Show styled help
Type "./styled"
Enter
Sleep 1s

Type "./styled --help"
Enter
Sleep 2s

# Show styled command
Type "./styled style"
Enter
Sleep 500ms
Type "Test User"
Enter
Sleep 1s

# Show with different format
Type "./styled style --format json"
Enter
Sleep 1s

