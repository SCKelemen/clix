#!/bin/bash
# Alternative recording script with customizable minimal prompt
# This version allows you to set a custom prompt for recordings
# Usage: PROMPT_TEXT="> " ./record_minimal.sh <name>

set -e

NAME="$1"
if [ -z "$NAME" ]; then
    echo "Usage: PROMPT_TEXT='> ' $0 <name> [cast_file]"
    echo "Example: PROMPT_TEXT='$ ' $0 commands_0"
    exit 1
fi

CAST_FILE="${2:-${NAME}.cast}"
GIF_FILE="${NAME}.gif"
WEBP_FILE="${NAME}.webp"

# Use custom prompt if provided, otherwise use simple prompt
PROMPT_TEXT="${PROMPT_TEXT:-$ }"

echo "Recording terminal session with prompt: '$PROMPT_TEXT'"
echo "Type your commands, then press Ctrl+D when finished"

# Set minimal prompt to hide username and hostname
export PS1="$PROMPT_TEXT"
export PROMPT="$PROMPT_TEXT"

asciinema rec "$CAST_FILE"

echo "Converting to GIF..."
agg "$CAST_FILE" "$GIF_FILE" --theme asciinema

echo "Converting to WebP with maximum quality..."
ffmpeg -i "$GIF_FILE" -vcodec libwebp -quality 100 -loop 0 "$WEBP_FILE" -y

echo "Cleaning up intermediate files..."
rm "$CAST_FILE" "$GIF_FILE"

echo "✅ Recording saved as: $WEBP_FILE"

