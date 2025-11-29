#!/bin/bash
# Script to discover examples and generate VHS demo GIFs
# This script walks the examples directory and creates/updates tape files in each example's directory

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
EXAMPLES_DIR="$PROJECT_ROOT/examples"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Discovering examples in $EXAMPLES_DIR..."

# Function to generate a tape file for an example
generate_tape() {
    local example_name=$1
    local example_path=$2
    local main_cmd=$3
    local binary_name=$4
    local tape_file="$example_path/${binary_name}.tape"
    
    echo -e "${GREEN}Generating tape for: ${example_name} -> ${tape_file}${NC}"
    
    cat > "$tape_file" <<EOF
Output ${binary_name}.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"
Env TERM "xterm-256color"

# Build the example
Type "go build -o ${binary_name} ./cmd/${main_cmd}"
Enter
Sleep 2s

EOF

    # Add example-specific commands
    case "$example_name" in
        "basic")
            cat >> "$tape_file" <<'BASICEOF'
# Show help
Type "./demo"
Enter
Sleep 1s

Type "./demo --help"
Enter
Sleep 1.5s

Type "./demo greet --help"
Enter
Sleep 1.5s

Type "./demo greet Alice"
Enter
Sleep 1s

Type "./demo greet"
Enter
Sleep 500ms
Type "Bob"
Enter
Sleep 1s
BASICEOF
            ;;
        "survey")
            cat >> "$tape_file" <<'SURVEYEOF'
# Run simple survey
Type "./demo simple"
Enter
Sleep 1s
# Wait for first prompt to appear
Type "John Doe"
Enter
Sleep 1s
# Wait for email prompt
Type "john@example.com"
Enter
Sleep 1s
# Wait for country prompt (accept default with Enter)
Enter
Sleep 1s
# Wait for newsletter prompt
Type "y"
Enter
Sleep 1s
# Wait for end card confirmation
Type "y"
Enter
Sleep 2s

# Run advanced survey
Type "./demo advanced"
Enter
Sleep 1.5s
# Wait for name prompt
Type "Jane Smith"
Enter
Sleep 1s
# Wait for email prompt
Type "jane@example.com"
Enter
Sleep 1s
# Wait for country prompt (accept default with Enter)
Enter
Sleep 1s
# Wait for age prompt
Type "25"
Enter
Sleep 1s
# Wait for language prompt (accept default with Enter)
Enter
Sleep 1.5s
# Wait for interests multi-select prompt to appear, then navigate
Type "\033[B"
Sleep 300ms
Type "\033[B"
Sleep 300ms
# Select first item
Type " "
Sleep 300ms
# Navigate down
Type "\033[B"
Sleep 300ms
# Select second item
Type " "
Sleep 300ms
# Confirm selection
Enter
Sleep 1.5s
# Wait for experience select prompt to appear, then navigate
Type "\033[B"
Sleep 300ms
Type "\033[B"
Sleep 300ms
# Select option
Enter
Sleep 1s
# Wait for newsletter confirm prompt
Type "y"
Enter
Sleep 1s
# Wait for end card confirmation
Type "y"
Enter
Sleep 2s
SURVEYEOF
            ;;
        "lipgloss")
            cat >> "$tape_file" <<'LIPGLOSSEOF'
# Show styled help
Type "./styled"
Enter
Sleep 1s

Type "./styled --help"
Enter
Sleep 2s

# Run style command with interactive prompt
Type "./styled style"
Enter
Sleep 1.5s
# Wait for name prompt to appear, then type name
Type "Test User"
Enter
Sleep 1.5s
# Wait for confirmation prompt
Type "y"
Enter
Sleep 1.5s

# Show format option (non-interactive)
Type "./styled style --format json"
Enter
Sleep 1s
LIPGLOSSEOF
            ;;
        "gh")
            cat >> "$tape_file" <<'GHEOF'
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
GHEOF
            ;;
        "multicli")
            cat >> "$tape_file" <<'MULTICLIEOF'
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
MULTICLIEOF
            ;;
        "gcloud")
            cat >> "$tape_file" <<'GCLOUDEOF'
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
GCLOUDEOF
            ;;
        "bubbles")
            cat >> "$tape_file" <<'BUBBLESEOF'
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
BUBBLESEOF
            ;;
        *)
            # Generic example - just show help
            cat >> "$tape_file" <<GENERICEOF
# Show help
Type "./${binary_name}"
Enter
Sleep 1s

Type "./${binary_name} --help"
Enter
Sleep 2s
GENERICEOF
            ;;
    esac
}

# Discover examples by looking for cmd/ directories
discover_examples() {
    local examples_found=0
    
    for example_dir in "$EXAMPLES_DIR"/*; do
        if [[ ! -d "$example_dir" ]]; then
            continue
        fi
        
        local example_name=$(basename "$example_dir")
        
        # Look for cmd/ subdirectory
        if [[ -d "$example_dir/cmd" ]]; then
            # Find the first cmd subdirectory (usually cmd/<name>/)
            local cmd_dir=$(find "$example_dir/cmd" -mindepth 1 -maxdepth 1 -type d | head -1)
            if [[ -n "$cmd_dir" ]]; then
                local main_cmd=$(basename "$cmd_dir")
                local binary_name="$example_name"
                
                # Special cases for binary names
                case "$example_name" in
                    "basic") binary_name="demo" ;;
                    "survey") binary_name="demo" ;;
                    "lipgloss") binary_name="styled" ;;
                    "bubbles") binary_name="demo" ;;
                    "multicli")
                        # Skip multicli - it has multiple binaries, handled separately
                        continue
                        ;;
                esac
                
                generate_tape "$example_name" "$example_dir" "$main_cmd" "$binary_name"
                examples_found=$((examples_found + 1))
            fi
        fi
    done
    
    # Handle multicli separately (has multiple binaries)
    if [[ -d "$EXAMPLES_DIR/multicli/cmd" ]]; then
        # Create multicli tape in multicli directory
        local multicli_tape="$EXAMPLES_DIR/multicli/multicli.tape"
        cat > "$multicli_tape" <<'MULTICLIHEAD'
Output multicli.gif
Set Width 1200
Set Height 800
Set Theme "Monokai Pro"
Set FontSize 18
Set Shell "bash"

# Build all CLIs
Type "go build -o dev ./cmd/dev && go build -o db ./cmd/db && go build -o sec ./cmd/sec && go build -o bq ./cmd/bq"
Enter
Sleep 3s
MULTICLIHEAD
        # Add multicli-specific commands
        cat >> "$multicli_tape" <<'MULTICLIEOF'
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
MULTICLIEOF
        examples_found=$((examples_found + 1))
    fi
    
    echo -e "${GREEN}Found ${examples_found} example(s)${NC}"
    return 0
}

# Generate GIFs from tape files
generate_gifs() {
    local vhs_cmd="vhs"
    if ! command -v vhs &> /dev/null; then
        # Try common locations
        if [[ -f "$HOME/go/bin/vhs" ]]; then
            vhs_cmd="$HOME/go/bin/vhs"
        elif [[ -f "/opt/homebrew/bin/vhs" ]]; then
            vhs_cmd="/opt/homebrew/bin/vhs"
        else
            echo -e "${YELLOW}Warning: vhs not found. Skipping GIF generation.${NC}"
            echo "Install vhs: brew install vhs or go install github.com/charmbracelet/vhs@latest"
            return 1
        fi
    fi
    
    echo -e "${GREEN}Generating GIFs...${NC}"
    
    # Find all tape files in examples directories
    for example_dir in "$EXAMPLES_DIR"/*; do
        if [[ ! -d "$example_dir" ]]; then
            continue
        fi
        
        # Look for .tape files in this example directory
        for tape_file in "$example_dir"/*.tape; do
            if [[ ! -f "$tape_file" ]]; then
                continue
            fi
            
            local example_name=$(basename "$example_dir")
            local tape_name=$(basename "$tape_file")
            echo -e "${GREEN}Generating GIF from ${example_name}/${tape_name}...${NC}"
            
            # Run VHS from the example directory
            cd "$example_dir"
            if "$vhs_cmd" < "$tape_name"; then
                echo -e "${GREEN}✓ Generated GIF in ${example_name}/${NC}"
            else
                echo -e "${YELLOW}⚠ Failed to generate GIF from ${tape_name}${NC}"
            fi
        done
    done
    
    cd "$PROJECT_ROOT"
}

# Main execution
main() {
    echo "=== Discovering Examples ==="
    discover_examples
    
    if [[ "${GENERATE_GIFS:-1}" == "1" ]]; then
        echo ""
        echo "=== Generating GIFs ==="
        generate_gifs
    else
        echo ""
        echo "Skipping GIF generation (set GENERATE_GIFS=1 to enable)"
    fi
}

main "$@"

