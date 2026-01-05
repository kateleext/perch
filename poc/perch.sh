#!/bin/bash
# Perch POC - Proof of concept before building the full TUI
#
# Dependencies: bat
# Install: brew install bat
#
# Controls: ↑↓ browse files · j k scroll preview · g top · q quit

DIR="${1:-.}"
cd "$DIR"

# State
SELECTED=0
SCROLL_OFFSET=0
FILES=()
PAGE_SIZE=8
LAST_STATUS=""

# Colors (muted, zen palette)
DIM='\033[2m'
CYAN='\033[38;5;109m'
RESET='\033[0m'

cleanup() {
    tput rmcup  # Exit alternate screen
    tput cnorm  # Show cursor
    exit 0
}
trap cleanup EXIT INT TERM

load_files() {
    FILES=()

    # Uncommitted items first (○ prefix)
    while IFS= read -r line; do
        [[ -z "$line" ]] && continue
        local file="${line:3}"
        FILES+=("uncommitted|$file||")
    done < <(git status --porcelain 2>/dev/null)

    # Recently committed files (✓ prefix) - last 5 commits
    while IFS= read -r line; do
        [[ -z "$line" ]] && continue
        local commit="${line%% *}"
        local rest="${line#* }"
        local time="${rest%% *}"
        local file="${rest#* }"

        # Skip if already in uncommitted
        local exists=0
        for f in "${FILES[@]}"; do
            [[ "$f" == *"|$file|"* ]] && exists=1 && break
        done
        [[ $exists -eq 0 ]] && FILES+=("committed|$file|$commit|$time")
    done < <(git log --name-only --pretty=format:"%h %ar" -n 5 2>/dev/null | awk '
        /^[a-f0-9]+ / { commit=$1; time=$2" "$3 }
        /^[^[:space:]]/ && !/^[a-f0-9]+ / { if(commit) print commit, time, $0 }
    ')

    # Clamp selection
    local max=${#FILES[@]}
    (( max > 0 )) && (( SELECTED >= max )) && SELECTED=$((max - 1))
    (( SELECTED < 0 )) && SELECTED=0
}

render() {
    tput cup 0 0

    # Get terminal dimensions
    local cols=$(tput cols)
    local rows=$(tput lines)
    local preview_lines=$((rows - PAGE_SIZE - 10))
    (( preview_lines < 5 )) && preview_lines=5

    # Header
    echo -e "${DIM}✦${RESET} perch"
    echo ""

    local total=${#FILES[@]}

    # Calculate visible window
    local start=0
    local end=$total

    if (( total > PAGE_SIZE )); then
        start=$((SELECTED - PAGE_SIZE / 2))
        (( start < 0 )) && start=0
        end=$((start + PAGE_SIZE))
        (( end > total )) && end=$total && start=$((end - PAGE_SIZE))
        (( start < 0 )) && start=0
    fi

    (( start > 0 )) && echo -e "  ${DIM}↑${RESET}"

    local i=$start
    while (( i < end )); do
        IFS='|' read -r status file commit time <<< "${FILES[$i]}"

        local icon="✓ "
        [[ "$status" == "uncommitted" ]] && icon="○ "

        if [[ $i -eq $SELECTED ]]; then
            # Selected: cyan arrow and filename
            echo -e "${CYAN}› ${icon}${file}${RESET}"
        else
            echo -e "  ${DIM}${icon}${RESET}${file}"
        fi
        ((i++))
    done

    (( end < total )) && echo -e "  ${DIM}↓${RESET}"

    # Empty state
    if (( total == 0 )); then
        echo -e "  ${DIM}no changes${RESET}"
    fi

    echo ""
    # Solid line divider
    local line=""
    for ((l=0; l<cols-2; l++)); do line+="─"; done
    echo -e "${DIM}${line}${RESET}"
    echo ""

    # Preview selected item
    if [[ ${#FILES[@]} -gt 0 ]]; then
        IFS='|' read -r status file commit time <<< "${FILES[$SELECTED]}"

        if [[ -d "$file" ]]; then
            # Directory or submodule
            local basename="${file##*/}"
            echo -e "${CYAN}${basename}${RESET}  ${DIM}directory${RESET}"
            echo ""
            echo -e "${DIM}contains:${RESET}"
            ls -1 "$file" 2>/dev/null | head -$((preview_lines - 3)) | while read item; do
                echo -e "  ${DIM}$item${RESET}"
            done
        elif [[ -n "$file" && -f "$file" ]]; then
            # File header with context
            local context="has changes"
            [[ "$status" == "committed" ]] && context="${time} · ${commit}"

            local basename="${file##*/}"
            echo -e "${CYAN}${basename}${RESET}  ${DIM}${context}${RESET}"
            echo ""

            # Smart preview - no line numbers for prose files
            local style_opts="--style=plain"
            case "$file" in
                *.md|*.txt|*.json|*.yml|*.yaml)
                    style_opts="--style=plain"
                    ;;
                *)
                    style_opts="--style=numbers"
                    ;;
            esac

            # Show file content
            local start_line=$((SCROLL_OFFSET + 1))
            local end_line=$((SCROLL_OFFSET + preview_lines))

            if [[ "$status" == "uncommitted" ]]; then
                bat $style_opts --color=always --theme="Nord" --diff --paging=never --line-range=${start_line}:${end_line} "$file" 2>/dev/null || head -n $end_line "$file" | tail -n +$start_line
            else
                bat $style_opts --color=always --theme="Nord" --paging=never --line-range=${start_line}:${end_line} "$file" 2>/dev/null || head -n $end_line "$file" | tail -n +$start_line
            fi

            # Show scroll indicator
            local line_count=$(wc -l < "$file" 2>/dev/null | tr -d ' ')
            line_count=${line_count:-0}
            if (( line_count > preview_lines )); then
                if (( SCROLL_OFFSET == 0 )); then
                    echo -e "${DIM}top of file${RESET}"
                elif (( SCROLL_OFFSET >= line_count - preview_lines )); then
                    echo -e "${DIM}end of file${RESET}"
                else
                    local percent=$(( (SCROLL_OFFSET * 100) / (line_count - preview_lines) ))
                    echo -e "${DIM}${percent}% ↓${RESET}"
                fi
            fi
        fi
    fi

    # Clear rest of screen
    tput ed

    # Bottom bar
    echo ""
    echo -e "${DIM}↑↓ browse · j k scroll · g top · q quit${RESET}"
}

# Setup terminal
tput smcup  # Enter alternate screen
tput civis  # Hide cursor
clear

# Initial load
load_files
LAST_STATUS=$(git status --porcelain 2>/dev/null; git log --oneline -n 1 2>/dev/null)
render

# Main loop
while true; do
    # Check for changes less frequently (only after key timeout)
    if read -rsn1 -t 2 key; then
        case "$key" in
            q) exit 0 ;;
            g) SELECTED=0; SCROLL_OFFSET=0; render ;;
            G) SELECTED=$((${#FILES[@]} - 1)); (( SELECTED < 0 )) && SELECTED=0; SCROLL_OFFSET=0; render ;;
            j) ((SCROLL_OFFSET++)); render ;;
            k) ((SCROLL_OFFSET--)); [[ $SCROLL_OFFSET -lt 0 ]] && SCROLL_OFFSET=0; render ;;
            $'\x1b')
                read -rsn2 -t 1 arrow
                case "$arrow" in
                    '[A') ((SELECTED--)); [[ $SELECTED -lt 0 ]] && SELECTED=0; SCROLL_OFFSET=0; render ;;
                    '[B') ((SELECTED++)); [[ $SELECTED -ge ${#FILES[@]} ]] && SELECTED=$((${#FILES[@]} - 1)); SCROLL_OFFSET=0; render ;;
                esac
                ;;
        esac
    else
        # Only check for file changes on timeout (every 2 sec)
        current=$(git status --porcelain 2>/dev/null; git log --oneline -n 1 2>/dev/null)
        if [[ "$current" != "$LAST_STATUS" ]]; then
            LAST_STATUS="$current"
            load_files
            render
        fi
    fi
done
