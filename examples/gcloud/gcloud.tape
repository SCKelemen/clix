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

# A private config directory for the configurations (hidden)
Hide
Type "export XDG_CONFIG_HOME=$(mktemp -d) && clear"
Enter
Sleep 100ms
Show

# A table on a terminal
Type "./gcloud projects list"
Enter
Sleep 1.5s

# Named configurations
Type "./gcloud config configurations create staging"
Enter
Sleep 1s

Type "./gcloud config set project demo-staging"
Enter
Sleep 1s

Type "./gcloud config configurations list"
Enter
Sleep 1.5s

Type "./gcloud projects describe"
Enter
Sleep 2s

# A flag group: --clear-labels and --remove-labels exclude each other
Type "./gcloud projects update --clear-labels --remove-labels env"
Enter
Sleep 1.5s

Type "./gcloud projects update --update-labels env=prod,owner=ana"
Enter
Sleep 1.5s

# --format is Ambient, so the missing project ID is still prompted for;
# then a progress indicator runs on stderr while JSON goes to stdout
Type "./gcloud --format json projects create"
Enter
Sleep 1s
Type "my-new-project"
Enter
Sleep 4s

# A help topic
Type "./gcloud help configurations"
Enter
Sleep 2s
