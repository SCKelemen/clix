Output interactive-prompts.gif
Width 1200
Height 800
Theme "Monokai Pro"
FontSize 18
Shell "bash"

Set CWD "../../.."

# Navigate to survey example
Type "cd examples/survey"
Enter
Sleep 500ms

# Run simple survey
Type "./demo simple"
Enter
Sleep 500ms
Type "John Doe"
Enter
Sleep 500ms
Type "john@example.com"
Enter
Sleep 500ms
Type "y"
Enter
Sleep 1s
Type "y"
Enter
Sleep 2s

# Show advanced survey (with select/multiselect)
Type "./demo advanced"
Enter
Sleep 500ms
Type "Jane Smith"
Enter
Sleep 500ms
Type "jane@example.com"
Enter
Sleep 500ms
Type "25"
Enter
Sleep 500ms
# Select country (use arrow keys)
Type "\033[B" # Down arrow
Sleep 300ms
Type "\033[B" # Down arrow
Sleep 300ms
Enter
Sleep 500ms
# Multi-select interests
Type "\033[B" # Down arrow
Sleep 300ms
Type " " # Select
Sleep 300ms
Type "\033[B" # Down arrow
Sleep 300ms
Type " " # Select
Sleep 300ms
Enter
Sleep 500ms
Type "y"
Enter
Sleep 2s

