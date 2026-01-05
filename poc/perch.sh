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
FILES=()        # Array of "status|file|commit|time"
PAGE_SIZE=8

# Colors (muted, zen palette)
DIM='\033[2m'
GREEN='\033[38;5;108m'
CYAN='\033[38;5;109m'
RESET='\033[0m'

cleanup() {
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
    tput ed

    # Get terminal width
    local cols=$(tput cols)
    local fog_width=$((cols - 4))
    local line_width=$((cols - 2))

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

        local prefix="  "
        local icon="✓ "
        [[ "$status" == "uncommitted" ]] && icon="○ "

        if [[ $i -eq $SELECTED ]]; then
            prefix="${CYAN}›${RESET} "
            local file_len=${#file}
            local trail_len=$((cols - file_len - 8))
            (( trail_len < 3 )) && trail_len=3
            local trail=""
            for ((t=0; t<trail_len; t++)); do trail+="─"; done
            echo -e "${prefix}${icon} ${file} ${DIM}${trail}${RESET}"
        else
            echo -e "${prefix}${DIM}${icon}${RESET} ${file}"
        fi
        ((i++))
    done

    (( end < total )) && echo -e "  ${DIM}↓${RESET}"

    # Empty state
    if (( total == 0 )); then
        echo -e "  ${DIM}no changes${RESET}"
    fi

    echo ""
    # Responsive fog divider
    local fog=""
    for ((f=0; f<fog_width/4; f++)); do fog+="·  "; done
    echo -e "${DIM}${fog}${RESET}"
    echo ""

    # Preview selected item
    if [[ ${#FILES[@]} -gt 0 ]]; then
        IFS='|' read -r status file commit time <<< "${FILES[$SELECTED]}"

        if [[ -d "$file" ]]; then
            # Directory or submodule
            local basename="${file##*/}"
            echo -e "${basename}  ${DIM}directory${RESET}"
            echo ""
            echo -e "${DIM}contains:${RESET}"
            ls -1 "$file" 2>/dev/null | head -10 | while read item; do
                echo -e "  ${DIM}$item${RESET}"
            done
        elif [[ -n "$file" && -f "$file" ]]; then
            # File header with context
            local context="has changes"
            [[ "$status" == "committed" ]] && context="${time} · ${commit}"

            local basename="${file##*/}"
            echo -e "${basename}  ${DIM}${context}${RESET}"
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

            # Show with diff if uncommitted
            if [[ "$status" == "uncommitted" ]]; then
                bat $style_opts --color=always --theme="Nord" --diff --paging=never --line-range=$((SCROLL_OFFSET+1)):$((SCROLL_OFFSET+25)) "$file" 2>/dev/null || cat "$file" | head -25
            else
                bat $style_opts --color=always --theme="Nord" --paging=never --line-range=$((SCROLL_OFFSET+1)):$((SCROLL_OFFSET+25)) "$file" 2>/dev/null || cat "$file" | head -25
            fi

            # Show scroll indicator
            local line_count=$(wc -l < "$file" 2>/dev/null || echo 0)
            if (( line_count > 25 )); then
                local percent=$(( (SCROLL_OFFSET * 100) / (line_count - 25) ))
                (( percent > 100 )) && percent=100
                (( percent < 0 )) && percent=0
                if (( SCROLL_OFFSET == 0 )); then
                    echo -e "${DIM}top of file${RESET}"
                elif (( SCROLL_OFFSET >= line_count - 25 )); then
                    echo -e "${DIM}end of file${RESET}"
                else
                    echo -e "${DIM}scrolling ↓ ${percent}%${RESET}"
                fi
            fi
        fi
    fi

    echo ""
    # Responsive bottom line
    local line=""
    for ((l=0; l<line_width; l++)); do line+="─"; done
    echo -e "${DIM}${line}${RESET}"
    echo -e "${DIM}↑↓ browse files · j k scroll preview · g top · q quit${RESET}"
}

# Setup terminal
tput civis  # Hide cursor

# Initial load
load_files
render

# Track changes
LAST_STATUS=""
check_changes() {
    local current
    current=$(git status --porcelain 2>/dev/null; git log --oneline -n 1 2>/dev/null)
    if [[ "$current" != "$LAST_STATUS" ]]; then
        LAST_STATUS="$current"
        load_files
        render
    fi
}

# Main loop
while true; do
    check_changes

    if read -rsn1 -t 1 key; then
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
    fi
done
