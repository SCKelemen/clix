Output multicli.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "zsh"
Env TERM "xterm-256color"
Env COLORTERM "truecolor"

# Build all CLIs
Type "go build -o dev ./cmd/dev && go build -o db ./cmd/db && go build -o sec ./cmd/sec && go build -o bq ./cmd/bq"
Enter
Sleep 3s
# Show dev CLI
Type "./dev --help"
Enter
Sleep 1.5s

Type "./dev database list"
Enter
Sleep 1s

# Show db CLI (focused)
Type "./db --help"
Enter
Sleep 1.5s

Type "./db list"
Enter
Sleep 1s

# Show sec CLI with aliases
Type "./sec --help"
Enter
Sleep 1.5s

Type "./sec vulns list"
Enter
Sleep 1s

# Show bq CLI with versioning
Type "./bq --help"
Enter
Sleep 1.5s

Type "./bq dataset list"
Enter
Sleep 1s
