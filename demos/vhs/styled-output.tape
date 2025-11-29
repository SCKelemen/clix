Output styled-output.gif
Width 1200
Height 800
Theme "Monokai Pro"
FontSize 18
Shell "bash"

Set CWD "../../.."

# Navigate to styled example
Type "cd examples/lipgloss"
Enter
Sleep 500ms

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

