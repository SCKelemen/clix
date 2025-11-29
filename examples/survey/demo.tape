Output demo.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"
Env TERM "xterm-256color"
Env COLORTERM "truecolor"

# Ensure NO_COLOR is not set and build
Type "unset NO_COLOR && go build -o demo ./cmd/demo"
Enter
Sleep 2s

# Run simple survey
Type "./demo simple"
Enter
Sleep 1s
# Wait for first prompt to appear
Type "John Doe"
Enter
Sleep 1s
# Wait for email prompt
Type "john@example.com"
Enter
Sleep 1s
# Wait for country prompt (accept default with Enter)
Enter
Sleep 1s
# Wait for newsletter prompt
Type "y"
Enter
Sleep 1s
# Wait for end card confirmation
Type "y"
Enter
Sleep 2s

# Run advanced survey
Type "./demo advanced"
Enter
Sleep 1.5s
# Wait for name prompt
Type "Jane Smith"
Enter
Sleep 1s
# Wait for email prompt
Type "jane@example.com"
Enter
Sleep 1s
# Wait for country prompt (accept default with Enter)
Enter
Sleep 1s
# Wait for age prompt
Type "25"
Enter
Sleep 1s
# Wait for language prompt (accept default with Enter)
Enter
Sleep 1.5s
# Wait for interests multi-select prompt to appear, then navigate
Type "\033[B"
Sleep 300ms
Type "\033[B"
Sleep 300ms
# Select first item
Type " "
Sleep 300ms
# Navigate down
Type "\033[B"
Sleep 300ms
# Select second item
Type " "
Sleep 300ms
# Confirm selection
Enter
Sleep 1.5s
# Wait for experience select prompt to appear, then navigate
Type "\033[B"
Sleep 300ms
Type "\033[B"
Sleep 300ms
# Select option
Enter
Sleep 1s
# Wait for newsletter confirm prompt
Type "y"
Enter
Sleep 1s
# Wait for end card confirmation
Type "y"
Enter
Sleep 2s
