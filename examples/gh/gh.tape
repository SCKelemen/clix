Output gh.gif
Set Width 1200
Set Height 800
Set Theme "nord"
Set FontSize 18
Set Shell "zsh"
Env TERM "xterm-256color"
Env COLORTERM "truecolor"
Env PATH "/usr/bin:/bin:/usr/local/bin:$HOME/go/bin:$PATH"

# Build the example (unset NO_COLOR and clear while hidden)
Hide
Type "unset NO_COLOR && clear"
Enter
Sleep 100ms
Show
Type "go build -o gh ./cmd/gh"
Enter
Sleep 2s

# Show help
Type "./gh"
Enter
Sleep 1s

Type "./gh auth --help"
Enter
Sleep 1.5s

Type "./gh repo --help"
Enter
Sleep 1.5s

Type "./gh pr --help"
Enter
Sleep 1.5s

Type "./gh org --help"
Enter
Sleep 1.5s

Type "./gh --version"
Enter
Sleep 1s
