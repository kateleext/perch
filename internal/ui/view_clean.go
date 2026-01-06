package ui

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// === TYPES ===

// ViewSnapshot captures the immutable state needed to render a frame
type ViewSnapshot struct {
	Files         []FilePreview
	SelectedIdx   int
	ListScroll    int
	PreviewScroll int
	ListHeight    int
	Width         int
	Height        int
	SparkleOn     bool
	Dir           string
}

// FilePreview is the subset of FileStatus needed for rendering
type FilePreview struct {
	Path       string
	Status     string
	GitCode    string
	ChangeType string
	IsSelected bool
}

// === SNAPSHOT CREATION ===

// captureSnapshot creates an immutable view of current state
func (m Model) captureSnapshot() ViewSnapshot {
	fileSnapshots := make([]FilePreview, len(m.files))
	for i, f := range m.files {
		fileSnapshots[i] = FilePreview{
			Path:       f.Path,
			Status:     f.Status,
			GitCode:    f.GitCode,
			ChangeType: f.ChangeType(),
			IsSelected: (i == m.selected),
		}
	}

	return ViewSnapshot{
		Files:         fileSnapshots,
		SelectedIdx:   m.selected,
		ListScroll:    m.listScroll,
		PreviewScroll: m.previewScroll,
		ListHeight:    m.listHeight,
		Width:         m.width,
		Height:        m.height,
		SparkleOn:     m.sparkleOn,
		Dir:           m.dir,
	}
}

// === HELPER FUNCTIONS ===

// padRightString adds padding between left and right content for exact width
func padRightString(left, right string, width int) string {
	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	padding := width - leftLen - rightLen - 1
	if padding < 1 {
		padding = 1
	}
	return left + strings.Repeat(" ", padding) + right
}

// === RENDERING ===

// Clean view rendering with clear separation of concerns
// Layout is simple: TOP = file list, BOTTOM = preview, FOOTER = quit hint

// renderViewClean builds the screen fresh with a simple two-pane layout
func renderViewClean(snap ViewSnapshot, model Model) string {
	var b bytes.Buffer

	// Safety checks
	if snap.Width <= 0 || snap.Height <= 0 {
		return ""
	}

	// Empty state?
	if len(snap.Files) == 0 {
		return renderEmptyStateClean(snap)
	}

	// === LAYOUT DECISION ===
	// Footer always takes 1 line at bottom
	// FileList takes snap.ListHeight lines from top
	// Preview gets whatever's left in middle
	footerHeight := 1
	fileListHeight := snap.ListHeight
	dividerHeight := 1
	previewHeaderHeight := 1
	previewUnderlineHeight := 1

	// Calculate space available
	totalContentHeight := snap.Height - footerHeight
	previewSpaceAvailable := totalContentHeight - fileListHeight - dividerHeight - previewHeaderHeight - previewUnderlineHeight

	if previewSpaceAvailable < 1 {
		previewSpaceAvailable = 1
	}

	// === FILE LIST SECTION ===
	renderFileListClean(&b, snap)

	// === DIVIDER ===
	b.WriteString(dividerStyle.Render(strings.Repeat("─", snap.Width)) + "\n")

	// === PREVIEW SECTION ===
	if snap.SelectedIdx >= 0 && snap.SelectedIdx < len(snap.Files) && model.preview.Valid {
		renderPreviewHeaderClean(&b, snap)
		b.WriteString(dividerStyle.Render(strings.Repeat("─", snap.Width)) + "\n")
		renderPreviewContentClean(&b, snap, model, previewSpaceAvailable)
	} else {
		// No file selected or preview not ready - fill preview space
		for i := 0; i < previewHeaderHeight+previewUnderlineHeight+previewSpaceAvailable; i++ {
			b.WriteString("\n")
		}
	}

	// === FOOTER ===
	renderFooterClean(&b, snap)

	return b.String()
}

// renderEmptyStateClean shows centered message when no files
func renderEmptyStateClean(snap ViewSnapshot) string {
	var b bytes.Buffer

	totalHeight := snap.Height
	emptyMsg := "no files found"
	emptyHint := "is this inside a git directory?"

	// Center vertically
	topPad := (totalHeight - 3) / 2
	for i := 0; i < topPad; i++ {
		b.WriteString("\n")
	}

	// Center message horizontally
	msgWidth := lipgloss.Width(emptyMsg)
	msgPad := (snap.Width - msgWidth) / 2
	if msgPad < 0 {
		msgPad = 0
	}
	b.WriteString(strings.Repeat(" ", msgPad) + emptyMsg + "\n\n")

	// Center hint horizontally
	hintWidth := lipgloss.Width(emptyHint)
	hintPad := (snap.Width - hintWidth) / 2
	if hintPad < 0 {
		hintPad = 0
	}
	b.WriteString(strings.Repeat(" ", hintPad) + emptyHint + "\n")

	// Fill rest
	for i := topPad + 3; i < totalHeight; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

// renderFileListClean renders exactly listHeight lines of the file list
func renderFileListClean(b *bytes.Buffer, snap ViewSnapshot) {
	if len(snap.Files) == 0 {
		// Pad to listHeight
		for i := 0; i < snap.ListHeight; i++ {
			b.WriteString("\n")
		}
		return
	}

	// Sparkle character
	sparkle := "✨"
	if snap.SparkleOn {
		sparkle = "✦"
	}

	// Truncate path for header
	shortPath := truncatePath(snap.Dir, 2)

	// Calculate what's visible with scroll
	showUpArrow := snap.ListScroll > 0

	// How many file slots do we have?
	fileSlots := snap.ListHeight - 1 // -1 for header

	if showUpArrow {
		fileSlots-- // -1 for up arrow
	}

	// Check if we need down arrow
	potentialEnd := snap.ListScroll + fileSlots
	showDownArrow := potentialEnd < len(snap.Files)

	if showDownArrow {
		fileSlots-- // -1 for down arrow
	}

	if fileSlots < 1 {
		fileSlots = 1
	}

	visibleStart := snap.ListScroll
	visibleEnd := snap.ListScroll + fileSlots
	if visibleEnd > len(snap.Files) {
		visibleEnd = len(snap.Files)
	}

	linesWritten := 0

	// Header line
	header := " " + sparkle + dimStyle.Render(" perched on "+shortPath)
	hint := keyStyle.Render("↑↓") + dimStyle.Render(" browse")
	b.WriteString(padRightString(header, hint, snap.Width) + "\n")
	linesWritten++

	// Up arrow if needed
	if showUpArrow {
		b.WriteString(dimStyle.Render("  ↑ more") + "\n")
		linesWritten++
	}

	// File items
	for i := visibleStart; i < visibleEnd; i++ {
		if i < 0 || i >= len(snap.Files) {
			break
		}

		f := snap.Files[i]

		// Icon for file status
		icon := "✓ "
		if f.Status == "uncommitted" {
			if f.GitCode == "??" || f.GitCode == "A " || f.GitCode == "AM" {
				icon = "✦ "
			} else {
				icon = "- "
			}
		}

		// Render with selection highlight
		var line string
		if f.IsSelected {
			line = selectedStyle.Render("› " + icon + f.Path)
		} else {
			line = "  " + dimStyle.Render(icon) + f.Path
		}

		b.WriteString(line + "\n")
		linesWritten++
	}

	// Down arrow if needed
	if showDownArrow {
		b.WriteString(dimStyle.Render("  ↓ more") + "\n")
		linesWritten++
	}

	// Pad to exact listHeight
	for linesWritten < snap.ListHeight {
		b.WriteString("\n")
		linesWritten++
	}
}

// renderPreviewHeaderClean renders the preview file header line
func renderPreviewHeaderClean(b *bytes.Buffer, snap ViewSnapshot) {
	if snap.SelectedIdx < 0 || snap.SelectedIdx >= len(snap.Files) {
		return
	}

	f := snap.Files[snap.SelectedIdx]
	basename := filepath.Base(f.Path)

	// Build the context/metadata string
	context := f.ChangeType

	// Add diff stats if uncommitted with changes
	if f.Status == "uncommitted" && f.Status == "uncommitted" {
		// Note: we'd need to pass diff stats through - for now just show change type
	}

	// Render header
	header := "  " + cyanStyle.Render(basename) + "  " + dimStyle.Render(context)
	hint := keyStyle.Render("j k") + dimStyle.Render(" scroll  ")
	b.WriteString(padRightString(header, hint, snap.Width) + "\n")
}

// renderPreviewContentClean renders the actual file content lines
func renderPreviewContentClean(b *bytes.Buffer, snap ViewSnapshot, model Model, contentHeight int) {
	preview := model.preview

	// No content to show?
	if !preview.Valid {
		for i := 0; i < contentHeight; i++ {
			b.WriteString("\n")
		}
		return
	}

	// Show message (deleted, unsupported, read error)
	if preview.Message != "" {
		b.WriteString("\n  " + dimStyle.Render(preview.Message) + "\n")
		for i := 1; i < contentHeight; i++ {
			b.WriteString("\n")
		}
		return
	}

	// Show file content - use raw lines for now to avoid syntax highlighting issues
	if len(preview.RawLines) > 0 {
		// Clamp scroll position
		startLine := snap.PreviewScroll
		if startLine < 0 {
			startLine = 0
		}
		if startLine >= len(preview.RawLines) {
			startLine = len(preview.RawLines) - 1
		}

		// Calculate end line
		endLine := startLine + contentHeight
		if endLine > len(preview.RawLines) {
			endLine = len(preview.RawLines)
		}

		// Safety: ensure we don't render negative or out of order ranges
		if startLine >= endLine {
			startLine = endLine - 1
			if startLine < 0 {
				startLine = 0
			}
		}

		linesRendered := 0

		for i := startLine; i < endLine && i >= 0 && i < len(preview.RawLines); i++ {
			lineNum := i + 1 // 1-indexed

			// Get diff status
			status, isDiff := preview.DiffLines[lineNum]

			// Get the line content - use raw for now
			lineContent := preview.RawLines[i]

			// Override with diff coloring if this line is a diff
			if isDiff {
				if status == "added" {
					lineContent = lineAddStyle.Render(lineContent)
				} else {
					lineContent = lineDelStyle.Render(lineContent)
				}
			}

			// Render with gutter
			if preview.IsMarkdown {
				b.WriteString("  " + lineContent + "\n")
			} else {
				gutter := dimStyle.Render("·")
				if isDiff {
					if status == "added" {
						gutter = lineAddStyle.Render("+")
					} else {
						gutter = lineDelStyle.Render("-")
					}
				}
				b.WriteString("  " + gutter + " " + lineContent + "\n")
			}

			linesRendered++
		}

		// Fill remaining space
		for i := linesRendered; i < contentHeight; i++ {
			b.WriteString("\n")
		}
	} else {
		// No lines - fill space
		for i := 0; i < contentHeight; i++ {
			b.WriteString("\n")
		}
	}
}

// renderFooterClean renders the bottom quit hint
func renderFooterClean(b *bytes.Buffer, snap ViewSnapshot) {
	quitHint := keyStyle.Render("q") + dimStyle.Render(" quit  ")
	b.WriteString(padRightString("", quitHint, snap.Width))
}
