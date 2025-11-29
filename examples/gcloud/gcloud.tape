Output gcloud.gif
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
Type "go build -o gcloud ./cmd/gcloud"
Enter
Sleep 2s

# Show help
Type "./gcloud"
Enter
Sleep 1s

Type "./gcloud auth --help"
Enter
Sleep 1.5s

Type "./gcloud config --help"
Enter
Sleep 1.5s

Type "./gcloud projects --help"
Enter
Sleep 1.5s
