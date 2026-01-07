package ui

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	// Theme-coherent colors (matching model.go: 109=cyan, 241=dim, 252=bright)
	mdH1Style      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("109"))
	mdH2Style      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("109"))
	mdH3Style      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	mdBoldStyle    = lipgloss.NewStyle().Bold(true)
	mdItalStyle    = lipgloss.NewStyle().Italic(true)
	mdCodeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	mdLinkText     = lipgloss.NewStyle().Foreground(lipgloss.Color("109")).Underline(true)
	mdLinkURL      = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
	mdBullet       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	mdTableBorder  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	mdTableHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))

	fenceRegex     = regexp.MustCompile("^[ \t]*```([A-Za-z0-9_+-]*)")
	tableSepRegex  = regexp.MustCompile(`^\|?[\s:-]+\|[\s|:-]*$`)
)

type mdRenderer struct {
	inCodeBlock bool
	codeLang    string
	// Table buffering for proper column alignment
	tableBuffer [][]string // buffered rows (each row is slice of cells)
	tableStart  int        // line index where table started
}

func highlightMarkdownLines(lines []string, filename string) []string {
	r := &mdRenderer{}
	result := make([]string, len(lines))
	for i, line := range lines {
		rendered, flush := r.renderLine(line, i)
		if flush && len(r.tableBuffer) > 0 {
			// Flush the buffered table into result
			// Save tableStart before flush clears it
			startIdx := r.tableStart
			tableLines := r.flushTable()
			for j, tl := range tableLines {
				idx := startIdx + j
				if idx < len(result) {
					result[idx] = tl
				}
			}
		}
		result[i] = rendered
	}
	// Flush any remaining table at end of file
	if len(r.tableBuffer) > 0 {
		startIdx := r.tableStart
		tableLines := r.flushTable()
		for j, tl := range tableLines {
			idx := startIdx + j
			if idx < len(result) {
				result[idx] = tl
			}
		}
	}
	return result
}

func (r *mdRenderer) renderLine(line string, lineIdx int) (rendered string, flushTable bool) {
	trimmed := strings.TrimSpace(line)

	if r.inCodeBlock {
		if strings.HasPrefix(trimmed, "```") {
			r.inCodeBlock = false
			return "", false
		}
		return highlightCodeFenceLine(line, r.codeLang), false
	}

	if matches := fenceRegex.FindStringSubmatch(line); matches != nil {
		r.inCodeBlock = true
		r.codeLang = matches[1]
		return "", false
	}

	// Table handling - buffer rows until table ends
	if isTableRow(trimmed) {
		if len(r.tableBuffer) == 0 {
			r.tableStart = lineIdx
		}
		// Skip separator rows but keep them in buffer for line count
		if isTableSeparator(trimmed) {
			r.tableBuffer = append(r.tableBuffer, nil) // nil = separator
		} else {
			cells := parseTableCells(trimmed)
			r.tableBuffer = append(r.tableBuffer, cells)
		}
		return "", false // placeholder, will be filled on flush
	}

	// Not a table row - flush any buffered table
	if len(r.tableBuffer) > 0 {
		return renderMarkdownTextLine(line), true
	}

	return renderMarkdownTextLine(line), false
}

func isTableRow(s string) bool {
	return strings.Contains(s, "|")
}

func isTableSeparator(s string) bool {
	return tableSepRegex.MatchString(s)
}

func parseTableCells(s string) []string {
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	parts := strings.Split(s, "|")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func (r *mdRenderer) flushTable() []string {
	if len(r.tableBuffer) == 0 {
		return nil
	}

	// Separate header (first row) from body, skip separator rows (nil)
	var headerCells []string
	var bodyRows [][]string
	isFirst := true

	for _, row := range r.tableBuffer {
		if row == nil {
			continue // separator row
		}
		if isFirst {
			headerCells = row
			isFirst = false
		} else {
			bodyRows = append(bodyRows, row)
		}
	}

	// Build lipgloss table - no top/bottom borders to fit original line count
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(mdTableBorder).
		BorderTop(false).
		BorderBottom(false).
		Headers(headerCells...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return mdTableHeader
			}
			return lipgloss.NewStyle()
		})

	for _, row := range bodyRows {
		// Apply inline markdown to each cell
		styledRow := make([]string, len(row))
		for i, cell := range row {
			styledRow[i] = renderInlineMarkdown(cell)
		}
		t.Row(styledRow...)
	}

	// Render table and split into lines
	rendered := t.Render()
	outputLines := strings.Split(rendered, "\n")

	// We need to map outputLines back to buffer slots (1:1 line mapping)
	// The lipgloss table may have different line count, so we pad/truncate
	result := make([]string, len(r.tableBuffer))
	for i := range result {
		if i < len(outputLines) {
			result[i] = outputLines[i]
		} else {
			result[i] = "" // extra lines become empty
		}
	}

	// Clear buffer
	r.tableBuffer = nil
	r.tableStart = 0

	return result
}

func highlightCodeFenceLine(line, lang string) string {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	styleName := "algol"
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, line)
	if err != nil {
		return line
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return line
	}

	result := buf.String()
	result = strings.ReplaceAll(result, "\n", "")
	if !strings.HasSuffix(result, "\x1b[0m") {
		result += "\x1b[0m"
	}
	return result
}

func renderMarkdownTextLine(line string) string {
	leading := leadingWhitespace(line)
	content := strings.TrimLeft(line, " \t")

	if content == "" {
		return ""
	}

	if lvl, body := parseHeading(content); lvl > 0 {
		return leading + styleHeading(lvl, body)
	}

	if bullet, body, ok := parseListItem(content); ok {
		return leading + mdBullet.Render(bullet) + " " + renderInlineMarkdown(body)
	}

	if strings.HasPrefix(content, ">") {
		body := strings.TrimPrefix(content, ">")
		body = strings.TrimPrefix(body, " ")
		return leading + mdItalStyle.Render("▌ ") + renderInlineMarkdown(body)
	}

	return leading + renderInlineMarkdown(content)
}

func leadingWhitespace(s string) string {
	for i, c := range s {
		if c != ' ' && c != '\t' {
			return s[:i]
		}
	}
	return s
}

func parseHeading(s string) (level int, body string) {
	if len(s) == 0 || s[0] != '#' {
		return 0, ""
	}

	level = 0
	for i := 0; i < len(s) && i < 6; i++ {
		if s[i] == '#' {
			level++
		} else {
			break
		}
	}

	if level == 0 || level > 6 {
		return 0, ""
	}

	if level >= len(s) {
		return level, ""
	}

	rest := s[level:]
	if len(rest) > 0 && rest[0] != ' ' && rest[0] != '\t' {
		return 0, ""
	}

	return level, strings.TrimSpace(rest)
}

func styleHeading(level int, body string) string {
	rendered := renderInlineMarkdown(body)
	switch level {
	case 1:
		return mdH1Style.Render(rendered)
	case 2:
		return mdH2Style.Render(rendered)
	default:
		return mdH3Style.Render(rendered)
	}
}

func parseListItem(s string) (bullet, body string, ok bool) {
	if len(s) == 0 {
		return "", "", false
	}

	if s[0] == '-' || s[0] == '*' || s[0] == '+' {
		if len(s) > 1 && (s[1] == ' ' || s[1] == '\t') {
			return string(s[0]), strings.TrimSpace(s[2:]), true
		}
		return "", "", false
	}

	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && s[i] == '.' {
		if i+1 < len(s) && (s[i+1] == ' ' || s[i+1] == '\t') {
			return s[:i+1], strings.TrimSpace(s[i+2:]), true
		}
	}

	return "", "", false
}

func renderInlineMarkdown(s string) string {
	var out strings.Builder
	i := 0

	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			if next == '*' || next == '_' || next == '`' || next == '[' || next == ']' {
				out.WriteByte(next)
				i += 2
				continue
			}
		}

		if s[i] == '`' {
			end := findClosingByte(s, i+1, '`')
			if end > 0 {
				code := s[i+1 : end]
				out.WriteString(mdCodeStyle.Render(code))
				i = end + 1
				continue
			}
		}

		if s[i] == '[' {
			text, url, consumed, ok := parseLink(s[i:])
			if ok {
				out.WriteString(mdLinkText.Render(text))
				out.WriteString(mdLinkURL.Render(" (" + url + ")"))
				i += consumed
				continue
			}
		}

		if s[i] == '*' || s[i] == '_' {
			marker := s[i]

			if i+1 < len(s) && s[i+1] == marker {
				end := findClosing(s, i+2, string([]byte{marker, marker}))
				if end > 0 {
					inner := s[i+2 : end]
					out.WriteString(mdBoldStyle.Render(renderInlineMarkdown(inner)))
					i = end + 2
					continue
				}
			}

			end := findClosingByte(s, i+1, marker)
			if end > 0 && end > i+1 {
				inner := s[i+1 : end]
				out.WriteString(mdItalStyle.Render(renderInlineMarkdown(inner)))
				i = end + 1
				continue
			}
		}

		out.WriteByte(s[i])
		i++
	}

	return out.String()
}

func parseLink(s string) (text, url string, consumed int, ok bool) {
	if len(s) == 0 || s[0] != '[' {
		return "", "", 0, false
	}

	closeBracket := findClosingByte(s, 1, ']')
	if closeBracket < 0 {
		return "", "", 0, false
	}

	if closeBracket+1 >= len(s) || s[closeBracket+1] != '(' {
		return "", "", 0, false
	}

	closeParen := findClosingByte(s, closeBracket+2, ')')
	if closeParen < 0 {
		return "", "", 0, false
	}

	text = s[1:closeBracket]
	url = s[closeBracket+2 : closeParen]
	consumed = closeParen + 1
	return text, url, consumed, true
}

func findClosing(s string, start int, marker string) int {
	if start >= len(s) {
		return -1
	}
	idx := strings.Index(s[start:], marker)
	if idx < 0 {
		return -1
	}
	return start + idx
}

func findClosingByte(s string, start int, b byte) int {
	for i := start; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
