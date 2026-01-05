#!/bin/bash
# Drift POC - Proof of concept before building the full TUI
# Validates: file watching, git integration, syntax highlighting
#
# Dependencies: fswatch, bat
# Install: brew install fswatch bat

set -e

DIR="${1:-.}"
cd "$DIR"

# Colors (muted, zen palette)
DIM='\033[2m'
GREEN='\033[38;5;108m'
RED='\033[38;5;131m'
CYAN='\033[38;5;109m'
RESET='\033[0m'

get_modified_files() {
    git status --porcelain 2>/dev/null | head -10
}

get_most_recent() {
    # Get the most recently modified tracked file
    git status --porcelain 2>/dev/null | head -1 | awk '{print $2}'
}

render() {
    clear

    # Header
    echo -e "${DIM}─── drift ───${RESET}"
    echo ""

    # File list
    while IFS= read -r line; do
        status="${line:0:2}"
        file="${line:3}"

        case "$status" in
            " M"|"M "|"MM") echo -e "  ${GREEN}M${RESET} $file" ;;
            " A"|"A "|"AM") echo -e "  ${GREEN}A${RESET} $file" ;;
            " D"|"D ")      echo -e "  ${RED}D${RESET} $file" ;;
            "??")           echo -e "  ${DIM}?${RESET} $file" ;;
            *)              echo -e "  ${DIM}${status}${RESET} $file" ;;
        esac
    done <<< "$(get_modified_files)"

    echo ""
    echo -e "${DIM}───${RESET}"
    echo ""

    # Preview most recent file
    recent=$(get_most_recent)
    if [[ -n "$recent" && -f "$recent" ]]; then
        echo -e "${DIM}$recent${RESET}"
        echo ""
        # Show file with diff highlighting, limit to terminal height
        bat --style=plain --color=always --diff --paging=never --line-range=:30 "$recent" 2>/dev/null || cat "$recent" | head -30
    fi
}

# Initial render
render

# Watch for changes and re-render
fswatch -o . 2>/dev/null | while read; do
    render
done
