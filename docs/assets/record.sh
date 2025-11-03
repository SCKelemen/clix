#!/bin/bash
# Helper script to record terminal animations and convert to WebP
# Usage: ./record.sh <name> [cast_file]
# Example: ./record.sh commands_0

set -e

NAME="$1"
if [ -z "$NAME" ]; then
    echo "Usage: $0 <name> [cast_file]"
    echo "Example: $0 commands_0"
    exit 1
fi

CAST_FILE="${2:-${NAME}.cast}"
GIF_FILE="${NAME}.gif"
WEBP_FILE="${NAME}.webp"

echo "Recording terminal session..."
echo "Type your commands, then press Ctrl+D when finished"
echo ""
echo "Note: Using minimal prompt to hide username/hostname"

# Detect shell and use -c flag to run shell with custom prompt
# This bypasses .zshrc/.bashrc which would override the prompt
SHELL_NAME=$(basename "$SHELL" 2>/dev/null || echo "bash")

if [ "$SHELL_NAME" = "zsh" ]; then
    # For zsh: use -f to skip .zshrc, set PROMPT inline, then exec zsh
    asciinema rec -c "zsh -f -c 'PROMPT=\"\$ \" exec zsh'" "$CAST_FILE"
elif [ "$SHELL_NAME" = "bash" ]; then
    # For bash: use --norc to skip .bashrc, set PS1 inline, then exec bash
    asciinema rec -c "bash --norc -c 'PS1=\"\$ \" exec bash'" "$CAST_FILE"
else
    # Fallback: try environment variables (may not work)
    export PS1='$ '
    export PROMPT='$ '
    asciinema rec "$CAST_FILE"
fi

echo ""
echo "Converting to GIF..."

# Convert to GIF using agg
if command -v agg &> /dev/null; then
    agg "$CAST_FILE" "$GIF_FILE" --theme asciinema
else
    echo "Error: agg not found. Install with: brew install agg"
    exit 1
fi

echo "Converting to WebP with maximum quality..."

# Convert GIF to WebP with maximum quality and looping
if command -v ffmpeg &> /dev/null; then
    ffmpeg -i "$GIF_FILE" -vcodec libwebp -quality 100 -loop 0 -preset default "$WEBP_FILE" -y
    echo ""
    echo "✓ Created: $WEBP_FILE"
    echo "  File size: $(du -h "$WEBP_FILE" | cut -f1)"
else
    echo "Error: ffmpeg not found. Install with: brew install ffmpeg"
    exit 1
fi

# Clean up intermediate files
rm -f "$CAST_FILE" "$GIF_FILE"

echo ""
echo "Done! Add this to your markdown:"
echo "![Description](assets/${WEBP_FILE})"

