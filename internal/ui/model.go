package ui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kateleext/perch/internal/git"
)

// Styles
var (
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cyanStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	dividerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // Bright keys
	addedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("71"))  // Softer green
	deletedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("167")) // Darker red
	lineAddStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("71"))  // Softer green for added lines
	lineDelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("167")) // Darker red for deleted lines
	sparkleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("109")) // Cyan sparkle
)

// TickMsg for sparkle animation
type TickMsg time.Time

// Model is the main bubbletea model
type Model struct {
	files          []git.FileStatus
	selected       int
	listScroll     int
	dir            string
	gitRoot        string
	width          int
	height         int
	listHeight     int
	viewport       viewport.Model
	previewReady   bool
	dragging       bool
	dividerY       int
	diffLines      map[int]string   // line number -> "added" or "deleted"
	diffStats      git.DiffStats
	highlightLines []string         // syntax highlighted lines (raw content for diff lines)
	rawLines       []string         // original unhighlighted lines
	sparkleOn      bool             // sparkle animation state
}

// New creates a new UI model
func New(dir string) Model {
	gitRoot, _ := git.GetGitRoot(dir)
	return Model{
		dir:        dir,
		gitRoot:    gitRoot,
		listHeight: 8,
		diffLines:  make(map[int]string),
	}
}

// RefreshMsg tells the model to refresh files
type RefreshMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadFiles, tickCmd())
}

func (m Model) loadFiles() tea.Msg {
	files, _ := git.GetStatus(m.dir)
	return filesLoadedMsg{files: files}
}

type filesLoadedMsg struct {
	files []git.FileStatus
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.selected > 0 {
				m.selected--
				// Keep selector 1 line below top when scrolling up
				topBuffer := 1
				if m.selected < m.listScroll+topBuffer {
					m.listScroll = m.selected - topBuffer
					if m.listScroll < 0 {
						m.listScroll = 0
					}
				}
				m.viewport.GotoTop()
				m.updatePreview()
			}
		case "down":
			if m.selected < len(m.files)-1 {
				m.selected++
				// Visible capacity is listHeight - 3 (header + arrows)
				visibleCapacity := m.listHeight - 3
				if visibleCapacity < 1 {
					visibleCapacity = 1
				}
				// Keep selector 2 lines above bottom of visible area
				bottomBuffer := 2
				if visibleCapacity <= bottomBuffer {
					bottomBuffer = 0
				}
				if m.selected >= m.listScroll+visibleCapacity-bottomBuffer {
					m.listScroll = m.selected - visibleCapacity + bottomBuffer + 1
				}
				m.viewport.GotoTop()
				m.updatePreview()
			}
		case "j":
			m.viewport, cmd = m.viewport.Update(tea.KeyMsg{Type: tea.KeyDown})
			return m, cmd
		case "k":
			m.viewport, cmd = m.viewport.Update(tea.KeyMsg{Type: tea.KeyUp})
			return m, cmd
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		case "+", "=":
			if m.listHeight < m.height-10 {
				m.listHeight++
				m.recalculateViewport()
			}
		case "-", "_":
			if m.listHeight > 3 {
				m.listHeight--
				m.recalculateViewport()
			}
		}

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			if msg.Y > m.dividerY {
				m.viewport, cmd = m.viewport.Update(tea.KeyMsg{Type: tea.KeyUp})
				return m, cmd
			}
		case tea.MouseWheelDown:
			if msg.Y > m.dividerY {
				m.viewport, cmd = m.viewport.Update(tea.KeyMsg{Type: tea.KeyDown})
				return m, cmd
			}
		case tea.MouseLeft:
			if msg.Y >= m.dividerY-1 && msg.Y <= m.dividerY+1 {
				m.dragging = true
			}
		case tea.MouseRelease:
			m.dragging = false
		case tea.MouseMotion:
			if m.dragging {
				newListHeight := msg.Y - 2
				if newListHeight < 3 {
					newListHeight = 3
				}
				if newListHeight > m.height-10 {
					newListHeight = m.height - 10
				}
				if newListHeight != m.listHeight {
					m.listHeight = newListHeight
					m.recalculateViewport()
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateViewport()

	case filesLoadedMsg:
		m.files = msg.files
		if m.selected >= len(m.files) {
			m.selected = len(m.files) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
		m.updatePreview()

	case RefreshMsg:
		return m, m.loadFiles

	case TickMsg:
		m.sparkleOn = !m.sparkleOn
		return m, tickCmd()
	}

	return m, nil
}

func (m *Model) recalculateViewport() {
	// Layout: list + divider(1) + previewHeader(1) + underline(1) + preview + footer(1)
	previewHeight := m.height - m.listHeight - 4
	if previewHeight < 3 {
		previewHeight = 3
	}

	m.viewport = viewport.New(m.width-8, previewHeight) // -8 for line numbers + gutter + padding
	m.viewport.Style = lipgloss.NewStyle()
	m.dividerY = m.listHeight
	m.previewReady = true
	m.updatePreview()
}

func (m *Model) updatePreview() {
	if !m.previewReady || len(m.files) == 0 {
		return
	}

	file := m.files[m.selected]
	fullPath := filepath.Join(m.dir, file.Path)

	// Check if file was deleted
	if strings.Contains(file.GitCode, "D") {
		m.diffLines = make(map[int]string)
		m.diffStats = git.DiffStats{}
		m.viewport.SetContent(fmt.Sprintf("\n\n  %s was deleted", file.Path))
		return
	}

	// Check if file type is unsupported for preview
	if isUnsupportedFile(file.Path) {
		m.diffLines = make(map[int]string)
		m.diffStats = git.DiffStats{}
		m.viewport.SetContent(fmt.Sprintf("\n\n  %s was updated, but isn't supported in perch", file.Path))
		return
	}

	// Get diff info for uncommitted files (use file's GitRoot for git commands)
	if file.Status == "uncommitted" {
		gitRoot := file.GitRoot
		if gitRoot == "" {
			gitRoot = m.gitRoot
		}
		m.diffLines = git.GetDiffLines(gitRoot, file.FullPath)
		m.diffStats = git.GetDiffStats(gitRoot, file.FullPath)
	} else {
		m.diffLines = make(map[int]string)
		m.diffStats = git.DiffStats{}
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		m.viewport.SetContent(fmt.Sprintf("\n\n  couldn't read %s", file.Path))
		m.rawLines = nil
		m.highlightLines = nil
		return
	}

	// Store raw lines
	m.rawLines = strings.Split(string(content), "\n")

	// Syntax highlight the content
	m.highlightLines = highlightCode(string(content), file.Path)

	// Set viewport content (we'll render with highlighting in View)
	m.viewport.SetContent(string(content))
}

// highlightCode returns syntax-highlighted lines for the given content
func highlightCode(content, filename string) []string {
	// Get lexer for file type
	lexer := lexers.Match(filename)

	// Handle ERB files explicitly
	if strings.HasSuffix(filename, ".erb") {
		lexer = lexers.Get("erb")
		if lexer == nil {
			lexer = lexers.Get("html")
		}
	}

	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Use a muted dark terminal style
	style := styles.Get("nord")
	if style == nil {
		style = styles.Fallback
	}

	// Format for terminal (256 colors)
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	// Tokenize and format
	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return strings.Split(content, "\n")
	}

	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return strings.Split(content, "\n")
	}

	return strings.Split(buf.String(), "\n")
}

// isUnsupportedFile returns true for binary/non-text files
func isUnsupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	unsupported := map[string]bool{
		// Xcode
		".xcuserstate": true, ".xcworkspace": true, ".pbxproj": true,
		// Images
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".webp": true,
		// Binary
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		// Archives
		".zip": true, ".tar": true, ".gz": true, ".rar": true,
		// Media
		".mp3": true, ".mp4": true, ".wav": true, ".mov": true,
		// Fonts
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
		// PDFs
		".pdf": true,
	}
	if unsupported[ext] {
		return true
	}
	// Xcode workspace directories
	if strings.Contains(path, ".xcworkspace") || strings.Contains(path, ".xcodeproj") {
		return true
	}
	return false
}

// View implements tea.Model
func (m Model) View() string {
	var b strings.Builder

	// Top padding
	b.WriteString("\n")

	// File list
	shortPath := truncatePath(m.dir, 2)
	// Sparkle glows between dim and bright
	sparkle := ""
	if m.sparkleOn {
		sparkle = sparkleStyle.Render("✦")
	} else {
		sparkle = dimStyle.Render("✦")
	}
	if len(m.files) == 0 {
		// Empty state - centered message
		header := " " + sparkle + dimStyle.Render(" perched on "+shortPath)
		hint := keyStyle.Render("↑↓") + dimStyle.Render(" browse")
		b.WriteString(m.padRight(header, hint) + "\n")

		// Calculate vertical center
		totalHeight := m.height - 2 // minus header and footer
		emptyMsg := "no files found"
		emptyHint := dimStyle.Render("is " + shortPath + " inside a git directory?")

		// Pad to center vertically
		topPad := totalHeight / 3
		for i := 0; i < topPad; i++ {
			b.WriteString("\n")
		}

		// Center the message horizontally
		msgPad := (m.width - lipgloss.Width(emptyMsg)) / 2
		if msgPad < 0 {
			msgPad = 0
		}
		b.WriteString(strings.Repeat(" ", msgPad) + emptyMsg + "\n\n")

		hintPad := (m.width - lipgloss.Width(emptyHint)) / 2
		if hintPad < 0 {
			hintPad = 0
		}
		b.WriteString(strings.Repeat(" ", hintPad) + emptyHint + "\n")

		// Fill remaining space
		for i := topPad + 3; i < totalHeight; i++ {
			b.WriteString("\n")
		}

		// Footer
		quitHint := keyStyle.Render("q") + dimStyle.Render(" quit")
		b.WriteString(m.padRight("", quitHint))
		return b.String()
	} else {
		// Calculate how many file lines we can actually show
		showUpArrow := m.listScroll > 0

		// Calculate visible capacity: total - header - arrows
		fileSlots := m.listHeight - 1 // -1 for header
		if showUpArrow {
			fileSlots-- // -1 for up arrow
		}
		// Check if we need down arrow (tentatively)
		potentialEnd := m.listScroll + fileSlots
		showDownArrow := potentialEnd < len(m.files)
		if showDownArrow {
			fileSlots-- // -1 for down arrow
		}
		if fileSlots < 1 {
			fileSlots = 1
		}

		visibleStart := m.listScroll
		visibleEnd := m.listScroll + fileSlots
		if visibleEnd > len(m.files) {
			visibleEnd = len(m.files)
		}

		linesUsed := 0

		// Header line with directory path
		header := " " + sparkle + dimStyle.Render(" perched on "+shortPath)
		hint := keyStyle.Render("↑↓") + dimStyle.Render(" browse")
		b.WriteString(m.padRight(header, hint) + "\n")
		linesUsed++

		if showUpArrow {
			b.WriteString(dimStyle.Render("  ↑ more") + "\n")
			linesUsed++
		}

		for i := visibleStart; i < visibleEnd; i++ {
			f := m.files[i]

			// Icons: ✦ new, - uncommitted/modified, ✓ committed
			icon := "✓ "
			if f.Status == "uncommitted" {
				if f.GitCode == "??" || f.GitCode == "A " || f.GitCode == "AM" {
					icon = "✦ " // sparkle for new files
				} else {
					icon = "- " // dash for modified
				}
			}

			var line string
			if i == m.selected {
				line = selectedStyle.Render("› " + icon + f.Path)
			} else {
				line = "  " + dimStyle.Render(icon) + f.Path
			}

			b.WriteString(line + "\n")
			linesUsed++
		}

		if showDownArrow {
			b.WriteString(dimStyle.Render("  ↓ more") + "\n")
			linesUsed++
		}

		for linesUsed < m.listHeight {
			b.WriteString("\n")
			linesUsed++
		}
	}

	// Divider
	dividerWidth := m.width
	if dividerWidth < 1 {
		dividerWidth = 40
	}
	b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

	// Preview header with scroll hint and +/- stats
	if len(m.files) > 0 {
		file := m.files[m.selected]
		basename := filepath.Base(file.Path)

		// Build context string
		var context string
		if file.Status == "uncommitted" {
			changeType := file.ChangeType()
			if m.diffStats.Added > 0 || m.diffStats.Deleted > 0 {
				adds := addedStyle.Render(fmt.Sprintf("+%d", m.diffStats.Added))
				dels := deletedStyle.Render(fmt.Sprintf("-%d", m.diffStats.Deleted))
				context = changeType + "  " + adds + " " + dels
			} else {
				context = changeType
			}
		} else {
			context = file.ChangeType()
		}

		header := "  " + cyanStyle.Render(basename) + "  " + dimStyle.Render(context)
		hint := keyStyle.Render("j k") + dimStyle.Render(" scroll  ")
		b.WriteString(m.padRight(header, hint) + "\n")

		// Underline below filename (full width, same as top divider)
		b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

		// Preview content with syntax highlighting and diff colors
		if m.previewReady && len(m.highlightLines) > 0 {
			startLine := m.viewport.YOffset
			endLine := startLine + m.viewport.Height
			if endLine > len(m.highlightLines) {
				endLine = len(m.highlightLines)
			}

			// Check if markdown (no gutter)
			isMarkdown := strings.HasSuffix(strings.ToLower(file.Path), ".md")

			for i := startLine; i < endLine; i++ {
				lineNum := i + 1 // 1-indexed for diff lookup

				// Get diff status for this line
				status, isDiff := m.diffLines[lineNum]

				var lineContent string
				if isDiff {
					// For diff lines, use raw content with color override
					rawLine := ""
					if i < len(m.rawLines) {
						rawLine = m.rawLines[i]
					}
					if status == "added" {
						lineContent = lineAddStyle.Render(rawLine)
					} else {
						lineContent = lineDelStyle.Render(rawLine)
					}
				} else {
					// Use syntax highlighted line
					lineContent = m.highlightLines[i]
				}

				if isMarkdown {
					// No gutter for markdown
					b.WriteString("  " + lineContent + "\n")
				} else {
					// Gutter: dot normally, +/- for diffs
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
			}
		}
	} else {
		b.WriteString("\n")
		for i := 0; i < m.viewport.Height; i++ {
			b.WriteString("\n")
		}
	}

	// Footer - quit on right (add trailing spaces for safety)
	quitHint := keyStyle.Render("q") + dimStyle.Render(" quit  ")
	b.WriteString(m.padRight("", quitHint))

	return b.String()
}

// truncatePath returns the last n path components
func truncatePath(path string, n int) string {
	parts := strings.Split(path, "/")
	if len(parts) <= n {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-n:], "/")
}

// padRight adds padding between left content and right hint
func (m Model) padRight(left, right string) string {
	// Strip ANSI codes for length calculation
	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	// -1 to account for terminal edge
	padding := m.width - leftLen - rightLen - 1
	if padding < 1 {
		padding = 1
	}
	return left + strings.Repeat(" ", padding) + right
}
