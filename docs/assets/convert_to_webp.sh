#!/bin/bash
# Convert existing GIF to high-quality looping WebP
# Usage: ./convert_to_webp.sh input.gif [output.webp]

set -e

INPUT="$1"
if [ -z "$INPUT" ]; then
    echo "Usage: $0 <input.gif> [output.webp]"
    exit 1
fi

OUTPUT="${2:-${INPUT%.*}.webp}"

if [ ! -f "$INPUT" ]; then
    echo "Error: File not found: $INPUT"
    exit 1
fi

echo "Converting $INPUT to WebP with maximum quality..."
ffmpeg -i "$INPUT" -vcodec libwebp -quality 100 -loop 0 -preset default "$OUTPUT" -y

echo ""
echo "✓ Created: $OUTPUT"
echo "  Input size:  $(du -h "$INPUT" | cut -f1)"
echo "  Output size: $(du -h "$OUTPUT" | cut -f1)"

