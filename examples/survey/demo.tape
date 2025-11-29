Output demo.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"

# Build the example
Type "go build -o demo ./cmd/demo"
Enter
Sleep 2s

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

# Run advanced survey
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
Type "\033[B"
Sleep 300ms
Type "\033[B"
Sleep 300ms
Enter
Sleep 500ms
Type "\033[B"
Sleep 300ms
Type " "
Sleep 300ms
Type "\033[B"
Sleep 300ms
Type " "
Sleep 300ms
Enter
Sleep 500ms
Type "y"
Enter
Sleep 2s
