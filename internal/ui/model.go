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
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	lineAddStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("71"))
	lineDelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("167"))
	blueStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))  // Subtle blue for sparkle
	blueDimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("67"))  // Dimmer blue for sparkle off
)

// TickMsg for sparkle animation
type TickMsg time.Time

// PreviewContent holds the rendered preview data for a specific file
type PreviewContent struct {
	Valid     bool
	Message   string
	RawLines  []string
	DiffLines map[int]string
	DiffStats git.DiffStats
}

// Model is the main bubbletea model
type Model struct {
	files            []git.FileStatus
	selected         int
	lastSelectedFile int
	listScroll       int
	dir              string
	gitRoot          string
	width            int
	height           int
	listHeight       int
	previewReady     bool
	preview          PreviewContent
	viewport         viewport.Model
	sparkleOn        bool
	loading          bool // true until first filesLoadedMsg
}

// New creates a new UI model
func New(dir string) Model {
	gitRoot, _ := git.GetGitRoot(dir)
	return Model{
		dir:        dir,
		gitRoot:    gitRoot,
		listHeight: 8,
		preview:    PreviewContent{},
		viewport:   viewport.New(80, 10),
		loading:    true, // Start in loading state
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
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.selected > 0 {
				m.selected--
				topBuffer := 1
				if m.selected < m.listScroll+topBuffer {
					m.listScroll = m.selected - topBuffer
					if m.listScroll < 0 {
						m.listScroll = 0
					}
				}
				m.updatePreview()
			}
		case "down":
			if m.selected < len(m.files)-1 {
				m.selected++
				visibleCapacity := m.listHeight - 3
				if visibleCapacity < 1 {
					visibleCapacity = 1
				}
				bottomBuffer := 2
				if visibleCapacity <= bottomBuffer {
					bottomBuffer = 0
				}
				if m.selected >= m.listScroll+visibleCapacity-bottomBuffer {
					m.listScroll = m.selected - visibleCapacity + bottomBuffer + 1
				}
				m.updatePreview()
			}
		case "j":
			m.viewport.LineDown(1)
		case "k":
			m.viewport.LineUp(1)
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		case "ctrl+d":
			m.viewport.HalfViewDown()
		case "ctrl+u":
			m.viewport.HalfViewUp()
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
			if msg.Y > m.listHeight+3 {
				m.viewport.LineUp(3)
			}
		case tea.MouseWheelDown:
			if msg.Y > m.listHeight+3 {
				m.viewport.LineDown(3)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateViewport()

	case filesLoadedMsg:
		m.loading = false
		
		// Remember currently selected file path to preserve selection
		var selectedPath string
		if m.selected >= 0 && m.selected < len(m.files) {
			selectedPath = m.files[m.selected].Path
		}
		
		m.files = msg.files
		
		// Try to keep selection on the same file
		newSelected := 0
		for i, f := range m.files {
			if f.Path == selectedPath {
				newSelected = i
				break
			}
		}
		m.selected = newSelected
		
		if m.selected >= len(m.files) {
			m.selected = len(m.files) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
		
		// Force preview refresh (even for same file) to update diffs
		m.lastSelectedFile = -1
		m.updatePreview()

	case RefreshMsg:
		return m, m.loadFiles

	case TickMsg:
		m.sparkleOn = !m.sparkleOn
		// Refresh files and diffs every tick
		return m, tea.Batch(tickCmd(), m.loadFiles)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) recalculateViewport() {
	// Layout: fileList (listHeight) + divider (1) + previewHeader (1) + underline (1) + viewport + footer (1)
	previewHeight := m.height - m.listHeight - 4
	if previewHeight < 1 {
		previewHeight = 1
	}
	m.viewport.Width = m.width
	m.viewport.Height = previewHeight
	m.previewReady = true
	m.updatePreview()
}

func (m *Model) updatePreview() {
	if !m.previewReady || len(m.files) == 0 {
		m.preview = PreviewContent{}
		m.viewport.SetContent("")
		m.lastSelectedFile = -1
		return
	}

	// Skip if we already have this file loaded
	if m.selected == m.lastSelectedFile && m.preview.Valid {
		return
	}

	file := m.files[m.selected]
	fullPath := filepath.Join(m.dir, file.Path)

	// Check if file was deleted
	if strings.Contains(file.GitCode, "D") {
		m.preview = PreviewContent{Valid: true, Message: fmt.Sprintf("%s was deleted", file.Path)}
		m.viewport.SetContent(m.renderPreviewContent())
		m.viewport.GotoTop()
		m.lastSelectedFile = m.selected
		return
	}

	// Check if file type is unsupported
	if isUnsupportedFile(file.Path) {
		reason := "not supported in perch"
		if filepath.Ext(file.Path) == "" {
			reason = "no file extension — open in your editor"
		}
		m.preview = PreviewContent{Valid: true, Message: fmt.Sprintf("%s\n%s", filepath.Base(file.Path), reason)}
		m.viewport.SetContent(m.renderPreviewContent())
		m.viewport.GotoTop()
		m.lastSelectedFile = m.selected
		return
	}

	// Get diff info
	var diffLines map[int]string
	var diffStats git.DiffStats
	if file.Status == "uncommitted" {
		gitRoot := file.GitRoot
		if gitRoot == "" {
			gitRoot = m.gitRoot
		}
		diffLines = git.GetDiffLines(gitRoot, file.FullPath)
		diffStats = git.GetDiffStats(gitRoot, file.FullPath)
	} else {
		diffLines = make(map[int]string)
		diffStats = git.DiffStats{}
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		m.preview = PreviewContent{Valid: true, Message: fmt.Sprintf("couldn't read %s", file.Path)}
		m.viewport.SetContent(m.renderPreviewContent())
		m.viewport.GotoTop()
		m.lastSelectedFile = m.selected
		return
	}

	rawLines := strings.Split(string(content), "\n")

	m.preview = PreviewContent{
		Valid:     true,
		RawLines:  rawLines,
		DiffLines: diffLines,
		DiffStats: diffStats,
	}

	m.viewport.SetContent(m.renderPreviewContent())
	m.viewport.GotoTop()
	m.lastSelectedFile = m.selected
}

// renderPreviewContent builds the content string for the viewport
func (m *Model) renderPreviewContent() string {
	if !m.preview.Valid {
		return ""
	}

	if m.preview.Message != "" {
		// Center the message in the viewport
		lines := strings.Split(m.preview.Message, "\n")
		var centered []string
		for _, line := range lines {
			padLeft := (m.width - lipgloss.Width(line)) / 2
			if padLeft < 0 {
				padLeft = 0
			}
			centered = append(centered, strings.Repeat(" ", padLeft)+dimStyle.Render(line))
		}
		// Add vertical padding
		vertPad := (m.viewport.Height - len(centered)) / 2
		if vertPad < 0 {
			vertPad = 0
		}
		result := strings.Repeat("\n", vertPad) + strings.Join(centered, "\n")
		return result
	}

	if len(m.preview.RawLines) == 0 {
		return ""
	}

	var b strings.Builder
	maxLineLen := m.width - 6
	if maxLineLen < 20 {
		maxLineLen = 20
	}

	for i, line := range m.preview.RawLines {
		lineNum := i + 1

		// Truncate long lines
		content := line
		if len(content) > maxLineLen {
			content = content[:maxLineLen-3] + "..."
		}

		// Apply diff styling
		status, isDiff := m.preview.DiffLines[lineNum]
		if isDiff {
			if status == "added" {
				content = lineAddStyle.Render(content)
			} else {
				content = lineDelStyle.Render(content)
			}
		}

		// Build gutter
		var gutter string
		if isDiff {
			if status == "added" {
				gutter = "  " + lineAddStyle.Render("+") + " "
			} else {
				gutter = "  " + lineDelStyle.Render("-") + " "
			}
		} else {
			gutter = "  " + dimStyle.Render("·") + " "
		}

		b.WriteString(gutter + content)
		if i < len(m.preview.RawLines)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// highlightCode returns syntax-highlighted lines (unused for now, keeping for future)
func highlightCode(content, filename string) []string {
	lines := strings.Split(content, "\n")

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".go" {
		return lines
	}

	lexer := lexers.Match(filename)
	if strings.HasSuffix(filename, ".erb") {
		lexer = lexers.Get("erb")
		if lexer == nil {
			lexer = lexers.Get("html")
		}
	}
	if lexer == nil {
		return lines
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("nord")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	highlightedLines := make([]string, len(lines))
	for i, line := range lines {
		iterator, err := lexer.Tokenise(nil, line)
		if err != nil {
			highlightedLines[i] = line
			continue
		}

		var buf bytes.Buffer
		err = formatter.Format(&buf, style, iterator)
		if err != nil {
			highlightedLines[i] = line
			continue
		}

		highlighted := strings.TrimSuffix(buf.String(), "\n")
		highlightedLines[i] = highlighted
	}

	return highlightedLines
}

func isUnsupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return true
	}
	unsupported := map[string]bool{
		".xcuserstate": true, ".xcworkspace": true, ".pbxproj": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".webp": true,
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".zip": true, ".tar": true, ".gz": true, ".rar": true,
		".mp3": true, ".mp4": true, ".wav": true, ".mov": true,
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
		".pdf": true,
	}
	if unsupported[ext] {
		return true
	}
	if strings.Contains(path, ".xcworkspace") || strings.Contains(path, ".xcodeproj") {
		return true
	}
	return false
}

// View implements tea.Model
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Show loading screen
	if m.loading {
		return m.renderLoadingScreen()
	}

	var b strings.Builder

	// === FILE LIST ===
	b.WriteString(m.renderFileList())

	// === DIVIDER ===
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)) + "\n")

	// === PREVIEW HEADER ===
	b.WriteString(m.renderPreviewHeader())
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)) + "\n")

	// === VIEWPORT (preview content) ===
	b.WriteString(m.viewport.View() + "\n")

	// === FOOTER ===
	b.WriteString(m.renderFooter())

	return b.String()
}

func (m Model) renderFileList() string {
	var lines []string

	// Header line: sparkle + "LATEST PROGRESS" + arrows on left, "perched on path" on right
	var sparkle string
	if m.sparkleOn {
		sparkle = blueStyle.Render("✧")
	} else {
		sparkle = blueDimStyle.Render("✧")
	}
	shortPath := truncatePath(m.dir, 2)
	header := " " + sparkle + " " + dimStyle.Render("LATEST PROGRESS") + "  " + dimStyle.Render("↑↓")
	pathHint := dimStyle.Render("perched on ") + dimStyle.Render(shortPath)
	lines = append(lines, padLine(header, pathHint, m.width))

	if len(m.files) == 0 {
		for len(lines) < m.listHeight {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n") + "\n"
	}

	// Calculate visible range (now we have 1 header line)
	showUpDots := m.listScroll > 0
	fileSlots := m.listHeight - 1 // -1 for header line
	if showUpDots {
		fileSlots--
	}
	potentialEnd := m.listScroll + fileSlots
	showDownDots := potentialEnd < len(m.files)
	if showDownDots {
		fileSlots--
	}
	if fileSlots < 1 {
		fileSlots = 1
	}

	visibleStart := m.listScroll
	visibleEnd := m.listScroll + fileSlots
	if visibleEnd > len(m.files) {
		visibleEnd = len(m.files)
	}

	// Up dots
	if showUpDots {
		lines = append(lines, dimStyle.Render("  ..."))
	}

	// Files
	maxPathLen := m.width - 8
	if maxPathLen < 10 {
		maxPathLen = 10
	}
	for i := visibleStart; i < visibleEnd; i++ {
		f := m.files[i]
		icon := "✓ "
		if f.Status == "uncommitted" {
			if f.GitCode == "??" || f.GitCode == "A " || f.GitCode == "AM" {
				icon = "✦ "
			} else {
				icon = "- "
			}
		}
		displayPath := f.Path
		if len(displayPath) > maxPathLen {
			displayPath = "..." + displayPath[len(displayPath)-maxPathLen+3:]
		}
		if i == m.selected {
			lines = append(lines, selectedStyle.Render("› "+icon+displayPath))
		} else {
			lines = append(lines, "  "+dimStyle.Render(icon)+displayPath)
		}
	}

	// Down dots
	if showDownDots {
		lines = append(lines, dimStyle.Render("  ..."))
	}

	// Pad to listHeight
	for len(lines) < m.listHeight {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n") + "\n"
}

func (m Model) renderPreviewHeader() string {
	if len(m.files) == 0 || m.selected < 0 || m.selected >= len(m.files) {
		return "\n"
	}

	f := m.files[m.selected]
	basename := filepath.Base(f.Path)
	header := "  " + cyanStyle.Render(basename) + "  " + dimStyle.Render(f.ChangeType())
	hint := keyStyle.Render("j k") + dimStyle.Render(" scroll  ")
	return padLine(header, hint, m.width) + "\n"
}

func (m Model) renderFooter() string {
	hint := keyStyle.Render("q") + dimStyle.Render(" quit  ")
	return padLine("", hint, m.width)
}

// Helper functions
func truncatePath(path string, n int) string {
	parts := strings.Split(path, "/")
	if len(parts) <= n {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-n:], "/")
}

func padLine(left, right string, width int) string {
	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	padding := width - leftLen - rightLen
	if padding < 1 {
		padding = 1
	}
	return left + strings.Repeat(" ", padding) + right
}

func (m Model) renderLoadingScreen() string {
	// ASCII art cloud with PERCH title
	art := []string{
		"                 _ (      ) _                PERCH",
		"           _  . (            )  .  _",
		"       _ (                          ) _",
		"     (   @@@%%##**++--..               )",
		"   (  @@@@@@@@%%%%####***+++--..         )",
		" (  @@@@@@@@@@@@@@@%%%%%%#####*****+++--.  )",
		"(  @@@@@@@@@@@@@@@@@@@@@@@%%%%%%%######***** )",
		" (  @@@@@@@@@@@@@@@@@@@@@@@@@@%%%%%%%%#####  )",
		"   (   @@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@%%   )",
		"     (____________________________________)",
	}

	var lines []string
	
	// Vertical centering
	totalArtHeight := len(art) + 2 // art + gap + message
	topPad := (m.height - totalArtHeight) / 2
	if topPad < 0 {
		topPad = 0
	}
	
	for i := 0; i < topPad; i++ {
		lines = append(lines, "")
	}
	
	// Art (centered)
	artWidth := 53
	padLeft := (m.width - artWidth) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	for _, line := range art {
		lines = append(lines, strings.Repeat(" ", padLeft)+dimStyle.Render(line))
	}
	
	lines = append(lines, "") // gap
	
	// Loading message
	msg := "scanning..."
	msgPad := (m.width - len(msg)) / 2
	if msgPad < 0 {
		msgPad = 0
	}
	lines = append(lines, strings.Repeat(" ", msgPad)+dimStyle.Render(msg))
	
	// Pad to full height
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	
	return strings.Join(lines, "\n")
}
