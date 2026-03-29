#!/bin/bash
# Quick test script to verify prompt works
# Run: ./test_prompt.sh

SHELL_NAME=$(basename "$SHELL" 2>/dev/null || echo "bash")

echo "Testing prompt override for: $SHELL_NAME"
echo ""

if [ "$SHELL_NAME" = "zsh" ]; then
    echo "Testing zsh prompt..."
    zsh -f -c "PROMPT='$ ' exec zsh"
elif [ "$SHELL_NAME" = "bash" ]; then
    echo "Testing bash prompt..."
    bash --norc -c "PS1='$ ' exec bash"
else
    echo "Unknown shell: $SHELL_NAME"
fi

