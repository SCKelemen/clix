# Manual Prompt Override for Recordings

If the automatic prompt override doesn't work, you can manually set a minimal prompt before recording.

## For zsh (macOS default):

Before starting `asciinema rec`, run:

```bash
PROMPT='$ '
```

Or for a slightly fancier minimal prompt:

```bash
PROMPT='%# '  # Shows $ for regular user, # for root
```

This will override your current prompt for the session. When you exit the shell, it will revert to your normal prompt.

## Quick Recording Workflow:

1. Open your terminal
2. Type: `PROMPT='$ '`
3. Run: `asciinema rec animation.cast`
4. Record your commands
5. Press Ctrl+D to finish

## To Reset:

After recording, either:
- Close the terminal window, or
- Run: `source ~/.zshrc` (or whatever your prompt config file is)

## Alternative: Create a Recording Alias

Add this to your `~/.zshrc`:

```bash
alias record='PROMPT="$ " asciinema rec'
```

Then you can just run:
```bash
record animation.cast
```

