package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Matches ERB tags: <%, <%=, <%#, <%-, -%>, etc.
	erbTagRegex = regexp.MustCompile(`<%[#=-]?.*?-?%>`)

	// Muted purple/magenta for ERB tags - works well with Catppuccin
	erbTagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("139"))
)

// styleERBTags applies ERB tag styling to an already-highlighted line.
// It finds ERB patterns and wraps them with the ERB style, being careful
// to handle existing ANSI codes.
func styleERBTags(line string) string {
	// Find all ERB tag matches
	matches := erbTagRegex.FindAllStringIndex(line, -1)
	if len(matches) == 0 {
		return line
	}

	// Build result by replacing matches with styled versions
	var result strings.Builder
	lastEnd := 0

	for _, match := range matches {
		start, end := match[0], match[1]

		// Add text before this match
		result.WriteString(line[lastEnd:start])

		// Extract the ERB tag and apply styling
		erbTag := line[start:end]
		// Reset any existing styling, apply ERB style, then reset again
		result.WriteString("\033[0m")
		result.WriteString(erbTagStyle.Render(erbTag))
		result.WriteString("\033[0m")

		lastEnd = end
	}

	// Add remaining text after last match
	result.WriteString(line[lastEnd:])

	return result.String()
}

// isERBFile checks if the file is an ERB template
func isERBFile(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".erb")
}

// isMarkdownERBFile checks if the file is a markdown ERB template
func isMarkdownERBFile(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".md.erb") || strings.HasSuffix(lower, ".markdown.erb")
}

// applyERBStyling applies ERB tag styling to all lines
func applyERBStyling(lines []string) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = styleERBTags(line)
	}
	return result
}
