Output demo.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "zsh"
Env TERM "xterm-256color"
Env COLORTERM "truecolor"

# Ensure NO_COLOR is not set and build
Type "unset NO_COLOR && go build -o demo ./cmd/demo"
Enter
Sleep 2s

# Show help
Type "./demo"
Enter
Sleep 1s

Type "./demo greet"
Enter
Sleep 500ms
Type "Alice"
Enter
Sleep 500ms
Type "\033[B"
Sleep 300ms
Enter
Sleep 500ms
Type "y"
Enter
Sleep 1s
