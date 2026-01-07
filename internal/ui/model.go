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

// DevBuild indicates if this is a development build
var DevBuild = false

// ANSI background codes for diff lines
const (
	bgAddANSI = "\033[48;2;18;40;18m" // #122812 - darker green
	bgDelANSI = "\033[48;2;45;18;18m" // #2d1212 - darker red
	ansiReset = "\033[0m"
)

// Styles
var (
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cyanStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	dividerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	lineAddGutter = lipgloss.NewStyle().Foreground(lipgloss.Color("#5a8a5a")) // muted green, blends with bg
	lineDelGutter = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a5a5a")) // muted red, blends with bg
	blueStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))  // Subtle blue for sparkle
	blueDimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("67"))  // Dimmer blue for sparkle off
)

// TickMsg for sparkle animation
type TickMsg time.Time

// PreviewContent holds the rendered preview data for a specific file
type PreviewContent struct {
	Valid            bool
	Message          string
	RawLines         []string
	HighlightedLines []string
	DiffLines        map[int]string
	DiffStats        git.DiffStats
	WrappedByWidth   map[int][]VisualLine
}

// ResetWrapCache clears the cached wrapped lines
func (pc *PreviewContent) ResetWrapCache() {
	pc.WrappedByWidth = make(map[int][]VisualLine)
}

// WrappedLinesForWidth returns wrapped lines for a given width, using cache
func (pc *PreviewContent) WrappedLinesForWidth(width int) []VisualLine {
	if pc.WrappedByWidth == nil {
		pc.WrappedByWidth = make(map[int][]VisualLine)
	}
	if lines, ok := pc.WrappedByWidth[width]; ok {
		return lines
	}
	lines := wrapAllLines(pc.HighlightedLines, pc.RawLines, pc.DiffLines, width)
	pc.WrappedByWidth[width] = lines
	return lines
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
	loadingFrame     int  // track animation frame for loading screen
	loadingStartTime time.Time // track when loading started
}

// New creates a new UI model
func New(dir string) Model {
	gitRoot, _ := git.GetGitRoot(dir)
	return Model{
		dir:              dir,
		gitRoot:          gitRoot,
		listHeight:       8,
		preview:          PreviewContent{},
		viewport:         viewport.New(80, 10),
		loading:          true, // Start in loading state
		loadingStartTime: time.Now(),
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
		// Enforce minimum 3 second loading screen
		if time.Since(m.loadingStartTime) < 3*time.Second {
			m.loading = true
		} else {
			m.loading = false
		}
		
		// Remember if we were at the top file
		wasAtTop := m.selected == 0
		
		// Remember currently selected file path to preserve selection
		var selectedPath string
		if m.selected >= 0 && m.selected < len(m.files) {
			selectedPath = m.files[m.selected].Path
		}
		
		m.files = msg.files
		
		// If we were at top, stay at top (auto-select newest)
		// Otherwise, try to keep selection on the same file
		newSelected := 0
		sameFile := false
		if wasAtTop {
			// Stay at top - auto-select newest file
			newSelected = 0
			sameFile = (len(m.files) > 0 && m.files[0].Path == selectedPath)
		} else {
			// Find the same file we had selected
			for i, f := range m.files {
				if f.Path == selectedPath {
					newSelected = i
					sameFile = true
					break
				}
			}
		}
		m.selected = newSelected
		
		if m.selected >= len(m.files) {
			m.selected = len(m.files) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
		
		// Refresh preview content (for updated diffs) but preserve scroll if same file
		m.lastSelectedFile = -1
		m.updatePreviewKeepScroll(sameFile)

	case RefreshMsg:
		return m, m.loadFiles

	case TickMsg:
		m.sparkleOn = !m.sparkleOn
		// Increment animation frame during loading
		if m.loading {
			m.loadingFrame++
		}
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
	m.updatePreviewKeepScroll(false)
}

func (m *Model) updatePreviewKeepScroll(keepScroll bool) {
	if !m.previewReady || len(m.files) == 0 {
		m.preview = PreviewContent{}
		m.viewport.SetContent("")
		m.lastSelectedFile = -1
		return
	}

	// Skip if we already have this file loaded (unless forced refresh)
	if m.selected == m.lastSelectedFile && m.preview.Valid {
		return
	}

	file := m.files[m.selected]
	fullPath := filepath.Join(m.dir, file.Path)

	// Check if file was deleted
	if strings.Contains(file.GitCode, "D") {
		m.preview = PreviewContent{Valid: true, Message: fmt.Sprintf("%s was deleted", file.Path)}
		m.viewport.SetContent(m.renderPreviewContent())
		if !keepScroll {
			m.viewport.GotoTop()
		}
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
		if !keepScroll {
			m.viewport.GotoTop()
		}
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
		if !keepScroll {
			m.viewport.GotoTop()
		}
		m.lastSelectedFile = m.selected
		return
	}

	rawLines := strings.Split(string(content), "\n")
	highlightedLines := highlightCode(string(content), file.Path)

	m.preview = PreviewContent{
		Valid:            true,
		RawLines:         rawLines,
		HighlightedLines: highlightedLines,
		DiffLines:        diffLines,
		DiffStats:        diffStats,
	}

	m.viewport.SetContent(m.renderPreviewContent())
	if !keepScroll {
		m.viewport.GotoTop()
	}
	m.lastSelectedFile = m.selected
}

// renderPreviewContent builds the content string for the viewport using wrapped lines
func (m *Model) renderPreviewContent() string {
	if !m.preview.Valid {
		return ""
	}

	if m.preview.Message != "" {
		lines := strings.Split(m.preview.Message, "\n")
		var centered []string
		for _, line := range lines {
			padLeft := (m.width - lipgloss.Width(line)) / 2
			if padLeft < 0 {
				padLeft = 0
			}
			centered = append(centered, strings.Repeat(" ", padLeft)+dimStyle.Render(line))
		}
		vertPad := (m.viewport.Height - len(centered)) / 2
		if vertPad < 0 {
			vertPad = 0
		}
		result := strings.Repeat("\n", vertPad) + strings.Join(centered, "\n")
		return result
	}

	if len(m.preview.HighlightedLines) == 0 {
		return ""
	}

	wrappedLines := m.preview.WrappedLinesForWidth(m.width)

	var b strings.Builder
	for i, vl := range wrappedLines {
		var gutter string
		var bgCode string

		switch vl.DiffStatus {
		case "added":
			gutter = "  " + lineAddGutter.Render(vl.Gutter)
			bgCode = bgAddANSI
		case "deleted":
			gutter = "  " + lineDelGutter.Render(vl.Gutter)
			bgCode = bgDelANSI
		default:
			gutter = "  " + dimStyle.Render(vl.Gutter)
		}

		// Calculate visible width BEFORE any background injection
		// gutter: "  " (2) + vl.Gutter (2, e.g. "+ ") = 4 visible chars
		// We use a fixed gutter width since it's always the same structure
		const gutterVisibleWidth = 4
		textWidth := VisibleWidth(vl.Text)
		totalWidth := gutterVisibleWidth + textWidth
		padding := m.width - totalWidth
		if padding < 0 {
			padding = 0
		}

		// Inject background into both gutter and content so it survives ANSI resets
		text := vl.Text
		if bgCode != "" {
			gutter = InjectBackground(gutter, bgCode)
			text = InjectBackground(vl.Text, bgCode)
		}

		// Build final line - for diff lines, wrap everything in background
		if bgCode != "" {
			// Start with background, write content, add padding, then reset
			// This ensures background extends fully regardless of ANSI codes in content
			b.WriteString(bgCode)
			b.WriteString(gutter)
			b.WriteString(text)
			b.WriteString(strings.Repeat(" ", padding))
			b.WriteString(ansiReset)
		} else {
			b.WriteString(gutter)
			b.WriteString(text)
			b.WriteString(strings.Repeat(" ", padding))
		}

		if i < len(wrappedLines)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// highlightCode returns syntax-highlighted lines using Catppuccin theme
func highlightCode(content, filename string) []string {
	rawLines := strings.Split(content, "\n")

	lexer := lexers.Match(filename)
	if strings.HasSuffix(filename, ".erb") {
		lexer = lexers.Get("erb")
		if lexer == nil {
			lexer = lexers.Get("html")
		}
	}
	if lexer == nil {
		return rawLines
	}
	lexer = chroma.Coalesce(lexer)

	// Use Catppuccin theme based on terminal background
	var styleName string
	if lipgloss.HasDarkBackground() {
		styleName = "catppuccin-mocha"
	} else {
		styleName = "catppuccin-latte"
	}
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	// Highlight each line independently to avoid ANSI bleed between lines
	highlightedLines := make([]string, len(rawLines))
	for i, line := range rawLines {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			highlightedLines[i] = line
			continue
		}

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

		// Clean up: remove any embedded newlines and ensure ANSI reset at end
		highlighted := buf.String()
		// Remove trailing reset, strip newlines, then add reset back
		highlighted = strings.TrimSuffix(highlighted, "\033[0m")
		highlighted = strings.TrimSuffix(highlighted, "\n")
		highlighted = strings.ReplaceAll(highlighted, "\n", "") // remove any embedded newlines
		highlighted += "\033[0m"
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
	devMarker := ""
	if DevBuild {
		devMarker = dimStyle.Render("[dev] ")
	}
	header := devMarker + sparkle + " " + dimStyle.Render("LATEST PROGRESS") + "  " + dimStyle.Render("↑↓")
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
	leftHint := dimStyle.Render("hold ") + keyStyle.Render("shift") + dimStyle.Render(" to select text")
	rightHint := keyStyle.Render("q") + dimStyle.Render(" quit  ")
	return padLine(leftHint, rightHint, m.width)
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
	// ASCII art cloud
	art := []string{
		"                 _ (      ) _",
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
	totalArtHeight := len(art) + 2 // art + gap + title
	topPad := (m.height - totalArtHeight) / 2
	if topPad < 0 {
		topPad = 0
	}
	
	for i := 0; i < topPad; i++ {
		lines = append(lines, "")
	}
	
	// Art (centered) with fade-in animation
	artWidth := 53
	padLeft := (m.width - artWidth) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	
	// Calculate opacity based on animation frame (0-10)
	opacity := m.loadingFrame
	if opacity > 10 {
		opacity = 10
	}
	
	for _, line := range art {
		renderedLine := strings.Repeat(" ", padLeft) + line
		// Apply fade-in by adjusting opacity
		if opacity < 10 {
			renderedLine = strings.Repeat(" ", padLeft) + dimStyle.Render(line)
		} else {
			renderedLine = strings.Repeat(" ", padLeft) + line
		}
		lines = append(lines, renderedLine)
	}
	
	lines = append(lines, "") // gap
	
	// Title
	title := "keeping an eye on the progress..."
	titlePad := (m.width - len(title)) / 2
	if titlePad < 0 {
		titlePad = 0
	}
	lines = append(lines, strings.Repeat(" ", titlePad)+title)
	
	// Pad to full height
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	
	return strings.Join(lines, "\n")
}
