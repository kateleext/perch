package ui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// VisualLine represents one physical line in the viewport
type VisualLine struct {
	LogicalIndex int    // index into HighlightedLines
	SegmentIndex int    // 0 = first segment, 1+ = continuations
	Gutter       string // "· ", "+ ", "- ", or "  " for continuations
	Text         string // ANSI-highlighted content slice
	DiffStatus   string // "added", "deleted", or "" for styling
}

const gutterWidth = 4 // "  · " or "  + " etc

// InjectBackground replaces all ANSI resets with reset+background to maintain bg color
func InjectBackground(s string, bgCode string) string {
	if bgCode == "" {
		return s
	}
	// Replace resets with reset+background, and prepend background
	return bgCode + strings.ReplaceAll(s, ansiReset, ansiReset+bgCode)
}

// countLeadingSpaces returns the number of leading space characters (not tabs)
func countLeadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r == ' ' {
			count++
		} else if r == '\t' {
			count += 4 // treat tab as 4 spaces
		} else {
			break
		}
	}
	return count
}

// runeVisualWidth returns the visual width of a rune, handling tabs specially
func runeVisualWidth(r rune) int {
	if r == '\t' {
		return 4 // treat tab as 4 spaces for consistency
	}
	return runewidth.RuneWidth(r)
}

// VisibleWidth returns visual column width, ignoring ANSI sequences
func VisibleWidth(s string) int {
	width := 0
	i := 0
	for i < len(s) {
		if isANSIStart(s, i) {
			i = skipANSI(s, i)
			continue
		}
		r, size := decodeRune(s, i)
		width += runeVisualWidth(r)
		i += size
	}
	return width
}

// decodeRune extracts a rune from string at position i
func decodeRune(s string, i int) (rune, int) {
	if i >= len(s) {
		return 0, 0
	}
	r := rune(s[i])
	if r < 0x80 {
		return r, 1
	}
	// UTF-8 multi-byte
	var size int
	if r&0xE0 == 0xC0 {
		size = 2
	} else if r&0xF0 == 0xE0 {
		size = 3
	} else if r&0xF8 == 0xF0 {
		size = 4
	} else {
		return r, 1
	}
	if i+size > len(s) {
		return r, 1
	}
	runes := []rune(s[i : i+size])
	if len(runes) > 0 {
		return runes[0], size
	}
	return r, 1
}

func isANSIStart(s string, i int) bool {
	if i+1 >= len(s) {
		return false
	}
	return s[i] == 0x1b && s[i+1] == '['
}

func skipANSI(s string, i int) int {
	if !isANSIStart(s, i) {
		return i + 1
	}
	j := i + 2
	for j < len(s) {
		b := s[j]
		if b >= 0x40 && b <= 0x7E {
			return j + 1
		}
		j++
	}
	return j
}

// sliceANSIAware slices a string to fit within maxWidth visible columns
// Returns the sliced content and any active ANSI codes that need to be preserved
func sliceANSIAware(s string, maxWidth int) (content string, remainder string, activeANSI string) {
	if maxWidth <= 0 {
		return "", s, ""
	}

	var result strings.Builder
	var currentANSI strings.Builder
	width := 0
	i := 0
	cutPoint := -1

	for i < len(s) && width < maxWidth {
		if isANSIStart(s, i) {
			// Capture ANSI sequence
			start := i
			i = skipANSI(s, i)
			ansi := s[start:i]
			result.WriteString(ansi)
			// Track active ANSI (reset clears it)
			if ansi == "\033[0m" {
				currentANSI.Reset()
			} else {
				currentANSI.WriteString(ansi)
			}
			continue
		}

		r, size := decodeRune(s, i)
		rw := runeVisualWidth(r)

		if width+rw > maxWidth {
			cutPoint = i
			break
		}

		result.WriteString(s[i : i+size])
		width += rw
		i += size
	}

	if cutPoint == -1 {
		cutPoint = i
	}

	// Ensure we close any open ANSI sequences
	content = result.String()
	if currentANSI.Len() > 0 {
		content += "\033[0m"
		activeANSI = currentANSI.String()
	}

	if cutPoint < len(s) {
		remainder = s[cutPoint:]
	}

	return content, remainder, activeANSI
}

// wrapHighlightedLine splits one highlighted line into VisualLine segments
// with hanging indent support - continuation lines preserve leading whitespace
func wrapHighlightedLine(line string, logicalIndex int, maxWidth int, diffStatus string, rawLine string) []VisualLine {
	if maxWidth <= gutterWidth {
		maxWidth = gutterWidth + 10
	}
	contentWidth := maxWidth - gutterWidth

	// Determine gutter symbol based on diff status
	var firstGutter string
	switch diffStatus {
	case "added":
		firstGutter = "+ "
	case "deleted":
		firstGutter = "- "
	default:
		firstGutter = "· "
	}
	contGutter := "  "

	// Calculate hanging indent from raw line's leading whitespace
	hangingIndent := countLeadingSpaces(rawLine)
	// Cap at reasonable max (half the content width)
	maxIndent := contentWidth / 2
	if hangingIndent > maxIndent {
		hangingIndent = maxIndent
	}
	hangingIndentStr := strings.Repeat(" ", hangingIndent)

	var result []VisualLine
	remaining := line
	segmentIndex := 0
	activeANSI := ""

	for {
		// Prepend active ANSI codes to continuation
		if activeANSI != "" && segmentIndex > 0 {
			remaining = activeANSI + remaining
		}

		// For continuation lines, reduce available width by hanging indent
		availWidth := contentWidth
		if segmentIndex > 0 && hangingIndent > 0 {
			availWidth = contentWidth - hangingIndent
			if availWidth < 10 {
				availWidth = 10 // minimum content width
			}
		}

		content, rest, newActiveANSI := sliceANSIAware(remaining, availWidth)

		gutter := contGutter
		if segmentIndex == 0 {
			gutter = firstGutter
		}

		// Add hanging indent to continuation lines
		text := content
		if segmentIndex > 0 && hangingIndent > 0 {
			text = hangingIndentStr + content
		}

		result = append(result, VisualLine{
			LogicalIndex: logicalIndex,
			SegmentIndex: segmentIndex,
			Gutter:       gutter,
			Text:         text,
			DiffStatus:   diffStatus,
		})

		if rest == "" {
			break
		}

		remaining = rest
		activeANSI = newActiveANSI
		segmentIndex++
	}

	return result
}

// wrapAllLines wraps all highlighted lines for a given width
func wrapAllLines(highlighted []string, rawLines []string, diffLines map[int]string, maxWidth int) []VisualLine {
	var result []VisualLine

	for i, line := range highlighted {
		lineNum := i + 1
		diffStatus := ""
		if status, ok := diffLines[lineNum]; ok {
			diffStatus = status
		}

		// Get raw line for indent calculation
		rawLine := ""
		if i < len(rawLines) {
			rawLine = rawLines[i]
		}

		wrapped := wrapHighlightedLine(line, i, maxWidth, diffStatus, rawLine)
		result = append(result, wrapped...)
	}

	return result
}
