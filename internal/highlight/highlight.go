package highlight

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// HighlightFile returns syntax-highlighted content for a file
func HighlightFile(path string, startLine, endLine int) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Get the lexer based on file extension
	lexer := lexers.Match(filepath.Base(path))
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Use a muted style (Nord-like)
	style := styles.Get("nord")
	if style == nil {
		style = styles.Fallback
	}

	// Use terminal256 formatter for ANSI output
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	// Get the lines we want
	lines := strings.Split(string(content), "\n")
	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > len(lines) {
		return "", nil
	}

	// Extract the range
	selectedLines := lines[startLine-1 : endLine]
	selectedContent := strings.Join(selectedLines, "\n")

	// Tokenize
	iterator, err := lexer.Tokenise(nil, selectedContent)
	if err != nil {
		return selectedContent, nil // Fall back to plain text
	}

	// Format
	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return selectedContent, nil
	}

	return buf.String(), nil
}

// LineCount returns the number of lines in a file
func LineCount(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return len(strings.Split(string(content), "\n")), nil
}
