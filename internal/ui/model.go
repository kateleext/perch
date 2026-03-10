package ui

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	gopdf "github.com/ledongthuc/pdf"

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

// Version is the current version of perch
var Version = "0.0.4"

// ANSI codes for diff lines
const (
	bgAddANSI = "\033[48;2;12;28;12m" // #0c1c0c - very dark green
	bgDelANSI = "\033[48;2;32;12;12m" // #200c0c - very dark red
	fgAddANSI = "\033[38;2;80;180;80m" // muted green text
	fgDelANSI = "\033[38;2;180;80;80m" // muted red text
	ansiReset = "\033[0m"
)

// stripANSIColors removes all ANSI escape sequences from a string
func stripANSIColors(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == 0x1b && s[i+1] == '[' {
			// Skip ANSI sequence
			j := i + 2
			for j < len(s) {
				b := s[j]
				if b >= 0x40 && b <= 0x7E {
					j++
					break
				}
				j++
			}
			i = j
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

// Styles
var (
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cyanStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	dividerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	keyStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	lineAddGutter  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5a8a5a")) // muted green, blends with bg
	lineDelGutter  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a5a5a")) // muted red, blends with bg
	lineDotStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))     // very subtle dots
	sparkleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))     // white sparkle
)

// TickMsg for sparkle animation
type TickMsg time.Time

// previewRequestMsg signals that a debounced preview load should start
type previewRequestMsg struct {
	selectedIndex int
}

// previewLoadedMsg carries the async-loaded preview content
type previewLoadedMsg struct {
	selectedIndex int
	gen           uint64 // generation when this load was started
	preview       PreviewContent
}

// flashClearMsg signals that the flash message should be cleared
type flashClearMsg struct{}

// PreviewContent holds the rendered preview data for a specific file
type PreviewContent struct {
	Valid            bool
	Message          string
	ImageRender      string // raw terminal output for image preview (bypass viewport)
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
	previewPending   int    // index of pending preview request (-1 = none)
	previewGen       uint64 // generation counter — incremented on each selection change
	previewCache     map[string]PreviewContent // cache by file path
	xOffset          int  // horizontal scroll offset for wide content
	flashMsg         string    // temporary status message
	flashExpiry      time.Time // when to clear flash
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
		previewPending:   -1,
		previewCache:     make(map[string]PreviewContent),
	}
}

// RefreshMsg tells the model to refresh files
type RefreshMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// debouncePreviewCmd waits briefly then signals to load the preview
func debouncePreviewCmd(selectedIndex int) tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return previewRequestMsg{selectedIndex: selectedIndex}
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
				m.previewGen++
				m.previewPending = m.selected
				m.lastSelectedFile = -1 // invalidate so header updates
				cmds = append(cmds, debouncePreviewCmd(m.selected))
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
				m.previewGen++
				m.previewPending = m.selected
				m.lastSelectedFile = -1
				cmds = append(cmds, debouncePreviewCmd(m.selected))
			}
		case "j":
			m.viewport.LineDown(1)
		case "k":
			m.viewport.LineUp(1)
		case "h", "left":
			if m.xOffset > 0 {
				m.xOffset -= 4
				if m.xOffset < 0 {
					m.xOffset = 0
				}
				m.viewport.SetContent(m.renderPreviewContent())
			}
		case "l", "right":
			m.xOffset += 4
			m.viewport.SetContent(m.renderPreviewContent())
		case "c":
			if text := m.getPreviewText(); text != "" {
				if copyToClipboard(text) == nil {
					m.flashMsg = "copied content"
					m.flashExpiry = time.Now().Add(2 * time.Second)
					cmds = append(cmds, clearFlashAfter(2*time.Second))
				}
			}
		case "p":
			if m.selected >= 0 && m.selected < len(m.files) {
				file := m.files[m.selected]
				fullPath := filepath.Join(m.dir, file.Path)
				if copyToClipboard(fullPath) == nil {
					m.flashMsg = "copied path"
					m.flashExpiry = time.Now().Add(2 * time.Second)
					cmds = append(cmds, clearFlashAfter(2*time.Second))
				}
			}
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
		case "shift+up":
			// Jump to top file
			if m.selected != 0 {
				m.selected = 0
				m.listScroll = 0
				m.previewGen++
				m.previewPending = m.selected
				m.lastSelectedFile = -1
				cmds = append(cmds, debouncePreviewCmd(m.selected))
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

	case previewRequestMsg:
		// Only load if this is still the pending request (debounce)
		if msg.selectedIndex != m.previewPending {
			return m, nil
		}
		if msg.selectedIndex < 0 || msg.selectedIndex >= len(m.files) {
			return m, nil
		}
		// Check cache first
		file := m.files[msg.selectedIndex]
		if cached, ok := m.previewCache[file.Path]; ok && file.Status == "committed" {
			// Use cached preview for committed files (they don't change)
			m.preview = cached
			m.viewport.SetContent(m.renderPreviewContent())
			m.viewport.GotoTop()
			m.lastSelectedFile = msg.selectedIndex
			m.previewPending = -1
			return m, nil
		}
		// Load async
		return m, m.loadPreviewAsync(msg.selectedIndex)

	case flashClearMsg:
		if time.Now().After(m.flashExpiry) {
			m.flashMsg = ""
		}
		return m, nil

	case previewLoadedMsg:
		// Only apply if still relevant (check both index and generation)
		if msg.selectedIndex != m.selected || msg.gen != m.previewGen {
			return m, nil
		}
		m.preview = msg.preview
		m.xOffset = 0
		// Cache committed file previews
		if msg.selectedIndex < len(m.files) {
			file := m.files[msg.selectedIndex]
			if file.Status == "committed" {
				m.previewCache[file.Path] = msg.preview
			}
		}
		m.viewport.SetContent(m.renderPreviewContent())
		
		// Auto-scroll to first diff for uncommitted files
		if msg.selectedIndex < len(m.files) {
			file := m.files[msg.selectedIndex]
			if file.Status == "uncommitted" && len(m.preview.DiffLines) > 0 {
				m.scrollToFirstDiff()
			} else {
				m.viewport.GotoTop()
			}
		} else {
			m.viewport.GotoTop()
		}
		
		m.lastSelectedFile = msg.selectedIndex
		m.previewPending = -1
	}


	return m, tea.Batch(cmds...)
}

// loadPreviewAsync returns a command that loads preview content in the background
func (m *Model) loadPreviewAsync(selectedIndex int) tea.Cmd {
	file := m.files[selectedIndex]
	dir := m.dir
	gitRoot := m.gitRoot
	gen := m.previewGen
	viewWidth := m.width
	viewHeight := m.viewport.Height
	if file.GitRoot != "" {
		gitRoot = file.GitRoot
	}

	return func() tea.Msg {
		fullPath := filepath.Join(dir, file.Path)

		// Check if file was deleted
		if strings.Contains(file.GitCode, "D") {
			return previewLoadedMsg{
				selectedIndex: selectedIndex,
				gen:           gen,
				preview:       PreviewContent{Valid: true, Message: fmt.Sprintf("%s was deleted", file.Path)},
			}
		}

		// Check if it's an image file
		if isImageFile(file.Path) {
			imgRender, msg := renderImagePreview(fullPath, viewWidth, viewHeight)
			return previewLoadedMsg{
				selectedIndex: selectedIndex,
				gen:           gen,
				preview:       PreviewContent{Valid: true, ImageRender: imgRender, Message: msg},
			}
		}

		// Check if it's a zip-based file (docx, xlsx, pptx, zip, etc.)
		if isZipBasedFile(file.Path) {
			ext := strings.ToLower(filepath.Ext(file.Path))
			// Docx/pptx: structured extraction with headings, wrapping, gutters
			if ext == ".docx" || ext == ".pptx" || ext == ".odt" {
				var rawLines, hlLines []string
				switch ext {
				case ".docx":
					rawLines, hlLines = extractDocxStructured(fullPath)
				case ".pptx":
					rawLines, hlLines = extractPptxStructured(fullPath)
				case ".odt":
					rawLines, hlLines = extractOdtStructured(fullPath)
				}
				if len(rawLines) > 0 {
					return previewLoadedMsg{
						selectedIndex: selectedIndex,
						gen:           gen,
						preview: PreviewContent{
							Valid:            true,
							RawLines:         rawLines,
							HighlightedLines: hlLines,
							DiffLines:        make(map[int]string),
						},
					}
				}
			}
			// xlsx/zip/etc: ImageRender (tables, raw ANSI)
			text := renderZipPreview(fullPath)
			return previewLoadedMsg{
				selectedIndex: selectedIndex,
				gen:           gen,
				preview:       PreviewContent{Valid: true, ImageRender: text},
			}
		}

		// Check if it's a PDF
		if strings.ToLower(filepath.Ext(file.Path)) == ".pdf" {
			rawLines, hlLines := extractPdfStructured(fullPath)
			if len(rawLines) > 0 {
				return previewLoadedMsg{
					selectedIndex: selectedIndex,
					gen:           gen,
					preview: PreviewContent{
						Valid:            true,
						RawLines:         rawLines,
						HighlightedLines: hlLines,
						DiffLines:        make(map[int]string),
					},
				}
			}
			return previewLoadedMsg{
				selectedIndex: selectedIndex,
				gen:           gen,
				preview:       PreviewContent{Valid: true, Message: filepath.Base(file.Path) + "\nPDF — no extractable text"},
			}
		}

		// Check if file type is supported text
		if !isSupportedTextFile(file.Path) {
			info, _ := os.Stat(fullPath)
			reason := "binary file — open in your editor"
			if filepath.Ext(file.Path) == "" {
				reason = "no file extension — open in your editor"
			}
			sizeHint := ""
			if info != nil {
				sizeHint = formatFileSize(info.Size()) + " · "
			}
			return previewLoadedMsg{
				selectedIndex: selectedIndex,
				gen:           gen,
				preview:       PreviewContent{Valid: true, Message: fmt.Sprintf("%s\n%s%s", filepath.Base(file.Path), sizeHint, reason)},
			}
		}

		// Get diff info
		var diffLines map[int]string
		var diffStats git.DiffStats
		if file.Status == "uncommitted" {
			diffLines = git.GetDiffLines(gitRoot, file.FullPath)
			diffStats = git.GetDiffStats(gitRoot, file.FullPath)
		} else {
			diffLines = make(map[int]string)
			diffStats = git.DiffStats{}
		}

		// Read file content
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return previewLoadedMsg{
				selectedIndex: selectedIndex,
				gen:           gen,
				preview:       PreviewContent{Valid: true, Message: fmt.Sprintf("couldn't read %s", file.Path)},
			}
		}

		rawLines := strings.Split(string(content), "\n")
		var highlightedLines []string
		if isMarkdownERBFile(file.Path) {
			highlightedLines = highlightMarkdownLines(rawLines, file.Path)
			highlightedLines = applyERBStyling(highlightedLines)
		} else if isMarkdownFile(file.Path) {
			highlightedLines = highlightMarkdownLines(rawLines, file.Path)
		} else if isERBFile(file.Path) {
			highlightedLines = highlightCode(string(content), file.Path)
			highlightedLines = applyERBStyling(highlightedLines)
		} else {
			highlightedLines = highlightCode(string(content), file.Path)
		}

		return previewLoadedMsg{
			selectedIndex: selectedIndex,
			gen:           gen,
			preview: PreviewContent{
				Valid:            true,
				RawLines:         rawLines,
				HighlightedLines: highlightedLines,
				DiffLines:        diffLines,
				DiffStats:        diffStats,
			},
		}
	}
}

func (m *Model) recalculateViewport() {
	// Layout: fileList (listHeight) + divider (1) + previewHeader (1) + underline (1) + viewport + indicators (up to 2) + footer (1)
	// Reserve space for up to 2 indicator lines (top + bottom dots) to keep layout stable
	previewHeight := m.height - m.listHeight - 6
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

	// Check if it's an image file
	if isImageFile(file.Path) {
		imgRender, msg := renderImagePreview(fullPath, m.width, m.viewport.Height)
		m.preview = PreviewContent{Valid: true, ImageRender: imgRender, Message: msg}
		m.viewport.SetContent(m.renderPreviewContent())
		if !keepScroll {
			m.viewport.GotoTop()
		}
		m.lastSelectedFile = m.selected
		return
	}

	// Check if it's a zip-based file (docx, xlsx, pptx, zip, etc.)
	if isZipBasedFile(file.Path) {
		ext := strings.ToLower(filepath.Ext(file.Path))
		if ext == ".docx" || ext == ".pptx" || ext == ".odt" {
			var rawLines, hlLines []string
			switch ext {
			case ".docx":
				rawLines, hlLines = extractDocxStructured(fullPath)
			case ".pptx":
				rawLines, hlLines = extractPptxStructured(fullPath)
			case ".odt":
				rawLines, hlLines = extractOdtStructured(fullPath)
			}
			if len(rawLines) > 0 {
				m.preview = PreviewContent{
					Valid:            true,
					RawLines:         rawLines,
					HighlightedLines: hlLines,
					DiffLines:        make(map[int]string),
				}
				m.viewport.SetContent(m.renderPreviewContent())
				if !keepScroll {
					m.viewport.GotoTop()
				}
				m.lastSelectedFile = m.selected
				return
			}
		}
		text := renderZipPreview(fullPath)
		m.preview = PreviewContent{
			Valid:       true,
			ImageRender: text,
		}
		m.viewport.SetContent(m.renderPreviewContent())
		if !keepScroll {
			m.viewport.GotoTop()
		}
		m.lastSelectedFile = m.selected
		return
	}

	// Check if it's a PDF
	if strings.ToLower(filepath.Ext(file.Path)) == ".pdf" {
		rawLines, hlLines := extractPdfStructured(fullPath)
		if len(rawLines) > 0 {
			m.preview = PreviewContent{
				Valid:            true,
				RawLines:         rawLines,
				HighlightedLines: hlLines,
				DiffLines:        make(map[int]string),
			}
		} else {
			m.preview = PreviewContent{Valid: true, Message: filepath.Base(file.Path) + "\nPDF — no extractable text"}
		}
		m.viewport.SetContent(m.renderPreviewContent())
		if !keepScroll {
			m.viewport.GotoTop()
		}
		m.lastSelectedFile = m.selected
		return
	}

	// Check if file type is supported text
	if !isSupportedTextFile(file.Path) {
		info, _ := os.Stat(fullPath)
		reason := "binary file — open in your editor"
		if filepath.Ext(file.Path) == "" {
			reason = "no file extension — open in your editor"
		}
		sizeHint := ""
		if info != nil {
			sizeHint = formatFileSize(info.Size()) + " · "
		}
		m.preview = PreviewContent{Valid: true, Message: fmt.Sprintf("%s\n%s%s", filepath.Base(file.Path), sizeHint, reason)}
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
	var highlightedLines []string
	if isMarkdownERBFile(file.Path) {
		highlightedLines = highlightMarkdownLines(rawLines, file.Path)
		highlightedLines = applyERBStyling(highlightedLines)
	} else if isMarkdownFile(file.Path) {
		highlightedLines = highlightMarkdownLines(rawLines, file.Path)
	} else if isERBFile(file.Path) {
		highlightedLines = highlightCode(string(content), file.Path)
		highlightedLines = applyERBStyling(highlightedLines)
	} else {
		highlightedLines = highlightCode(string(content), file.Path)
	}

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

	// ImageRender is raw terminal output — apply horizontal scroll if needed
	if m.preview.ImageRender != "" {
		if m.xOffset == 0 {
			return m.preview.ImageRender
		}
		lines := strings.Split(m.preview.ImageRender, "\n")
		shifted := make([]string, len(lines))
		for i, line := range lines {
			shifted[i] = ansiTrimLeft(line, m.xOffset)
		}
		return strings.Join(shifted, "\n")
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
		var fgCode string

		switch vl.DiffStatus {
		case "added":
			gutter = "  " + lineAddGutter.Render(vl.Gutter)
			bgCode = bgAddANSI
			fgCode = fgAddANSI
		case "deleted":
			gutter = "  " + lineDelGutter.Render(vl.Gutter)
			bgCode = bgDelANSI
			fgCode = fgDelANSI
		default:
			gutter = "  " + lineDotStyle.Render(vl.Gutter)
		}

		// Apply horizontal scroll to wide lines (e.g. unwrapped table lines)
		displayText := vl.Text
		if m.xOffset > 0 {
			textWidth := VisibleWidth(vl.Text)
			if textWidth > m.width-gutterWidth {
				displayText = ansiTrimLeft(vl.Text, m.xOffset)
			}
		}

		// Calculate visible width BEFORE any background injection
		// gutter: "  " (2) + vl.Gutter (2, e.g. "+ ") = 4 visible chars
		// We use a fixed gutter width since it's always the same structure
		const gutterVisibleWidth = 4
		textWidth := VisibleWidth(displayText)
		totalWidth := gutterVisibleWidth + textWidth
		padding := m.width - totalWidth
		if padding < 0 {
			padding = 0
		}

		// Inject background into both gutter and content so it survives ANSI resets
		text := displayText
		if bgCode != "" {
			gutter = InjectBackground(gutter, bgCode)
			// Apply foreground color to text (overrides syntax highlighting)
			text = fgCode + stripANSIColors(vl.Text) + ansiReset
			text = InjectBackground(text, bgCode)
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

// scrollToFirstDiff scrolls the viewport to the first diff line with context
func (m *Model) scrollToFirstDiff() {
	wrappedLines := m.preview.WrappedLinesForWidth(m.width)
	
	// Find the first line with a diff status
	firstDiffIndex := -1
	for i, vl := range wrappedLines {
		if vl.DiffStatus == "added" || vl.DiffStatus == "deleted" {
			firstDiffIndex = i
			break
		}
	}
	
	if firstDiffIndex == -1 {
		// No diff found, go to top
		m.viewport.GotoTop()
		return
	}
	
	// Add context lines above (3 lines of context)
	contextLines := 3
	targetOffset := firstDiffIndex - contextLines
	if targetOffset < 0 {
		targetOffset = 0
	}
	
	// Don't scroll if the diff is already near the top
	if targetOffset <= 2 {
		m.viewport.GotoTop()
		return
	}
	
	// Set the viewport offset
	m.viewport.SetYOffset(targetOffset)
}

// isMarkdownFile checks if a file is markdown based on extension

func isMarkdownFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".markdown", ".mdown", ".mdx":
		return true
	}
	return false
}

// highlightCode returns syntax-highlighted lines using algol theme
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

	styleName := "monokai"
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

// Supported text file extensions (whitelist approach)
var supportedTextExtensions = map[string]bool{
	// Programming languages
	".go": true, ".py": true, ".rb": true, ".js": true, ".ts": true,
	".jsx": true, ".tsx": true, ".rs": true, ".c": true, ".h": true,
	".cpp": true, ".cc": true, ".cxx": true, ".hpp": true, ".hxx": true,
	".java": true, ".kt": true, ".kts": true, ".swift": true,
	".cs": true, ".fs": true, ".fsx": true,
	".php": true, ".lua": true, ".r": true, ".m": true, ".mm": true,
	".scala": true, ".clj": true, ".cljs": true, ".cljc": true,
	".ex": true, ".exs": true, ".erl": true, ".hrl": true,
	".hs": true, ".lhs": true, ".ml": true, ".mli": true,
	".dart": true, ".zig": true, ".nim": true, ".v": true,
	".d": true, ".pas": true, ".pp": true,
	".pl": true, ".pm": true,
	".sh": true, ".bash": true, ".zsh": true, ".fish": true,
	".ps1": true, ".psm1": true, ".bat": true, ".cmd": true,
	".groovy": true, ".gradle": true,
	".vim": true, ".el": true, ".lisp": true, ".rkt": true,
	".jl": true, ".cr": true, ".raku": true,
	".awk": true, ".sed": true,
	// Web
	".html": true, ".htm": true, ".xhtml": true,
	".css": true, ".scss": true, ".sass": true, ".less": true, ".styl": true,
	".vue": true, ".svelte": true, ".astro": true,
	// Data/Config
	".json": true, ".jsonc": true, ".json5": true,
	".yaml": true, ".yml": true,
	".toml": true, ".xml": true, ".plist": true,
	".ini": true, ".cfg": true, ".conf": true,
	".env": true, ".properties": true,
	// Markup/Docs
	".md": true, ".markdown": true, ".mdown": true, ".mdx": true,
	".txt": true, ".text": true, ".rst": true, ".tex": true, ".adoc": true,
	".org": true,
	// Templates
	".erb": true, ".ejs": true, ".hbs": true, ".mustache": true,
	".j2": true, ".jinja": true, ".jinja2": true,
	".tmpl": true, ".tpl": true, ".liquid": true,
	".haml": true, ".slim": true, ".pug": true, ".jade": true,
	// Build
	".cmake": true, ".gemspec": true, ".podspec": true,
	// SQL
	".sql": true,
	// Other
	".graphql": true, ".gql": true, ".proto": true,
	".tf": true, ".hcl": true, ".nix": true,
	".lock": true, ".sum": true, ".mod": true,
	".csv": true, ".tsv": true,
	".log": true, ".diff": true, ".patch": true,
	".svg": true, // SVG is XML text
	// Dotfile extensions
	".gitignore": true, ".dockerignore": true, ".editorconfig": true,
	".htaccess": true, ".npmrc": true, ".nvmrc": true, ".yarnrc": true,
	".eslintrc": true, ".prettierrc": true, ".babelrc": true,
	".ladder": true,
}

// Known text filenames without extensions
var supportedFilenames = map[string]bool{
	"makefile": true, "dockerfile": true, "gemfile": true,
	"rakefile": true, "procfile": true, "vagrantfile": true,
	"brewfile": true, "justfile": true, "taskfile": true,
	"cmakelists.txt": true, "license": true, "readme": true,
	"changelog": true, "authors": true, "contributors": true,
	"copying": true, "todo": true,
	".gitattributes": true, ".gitmodules": true,
	".ruby-version": true, ".node-version": true, ".python-version": true,
	".tool-versions": true,
}

func isSupportedTextFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if supportedFilenames[base] {
		return true
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return false
	}

	// Handle double extensions like .html.erb — check the last ext
	return supportedTextExtensions[ext]
}

func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tiff", ".tif", ".ico":
		return true
	}
	return false
}

// renderImagePreview renders an image as half-block characters for the viewport.
// Each character cell = 2 vertical pixels using ▀ with fg=top, bg=bottom.
// Returns (imageRender, message) — imageRender is raw ANSI for the viewport.
func renderImagePreview(path string, viewWidth, viewHeight int) (imageRender string, message string) {
	f, err := os.Open(path)
	if err != nil {
		return "", renderImageFallback(path)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return "", renderImageFallback(path)
	}

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW == 0 || srcH == 0 {
		return "", renderImageFallback(path)
	}

	// Build info line
	info, _ := os.Stat(path)
	sizeStr := ""
	if info != nil {
		sizeStr = formatFileSize(info.Size()) + " · "
	}
	infoLine := fmt.Sprintf("%s  %s%dx%d · %s", filepath.Base(path), sizeStr, srcW, srcH, format)

	return renderImageHalfBlocks(img, infoLine, viewWidth, viewHeight), ""
}

// renderImageHalfBlocks renders an image using half-block characters (▀).
// Each character cell represents 2 vertical pixels with fg=top, bg=bottom.
func renderImageHalfBlocks(img image.Image, infoLine string, viewWidth, viewHeight int) string {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	// Calculate target size to fit viewport
	maxCols := viewWidth - 6
	maxRows := (viewHeight - 3) * 2 // each row of chars = 2 pixel rows
	if maxCols < 10 {
		maxCols = 10
	}
	if maxRows < 4 {
		maxRows = 4
	}

	// Scale to fit, maintaining aspect ratio
	scale := float64(maxCols) / float64(srcW)
	if float64(srcH)*scale > float64(maxRows) {
		scale = float64(maxRows) / float64(srcH)
	}

	dstW := int(float64(srcW) * scale)
	dstH := int(float64(srcH) * scale)
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}
	// Ensure even height for half-block pairing
	if dstH%2 != 0 {
		dstH++
	}

	var b strings.Builder

	// Center info line
	infoPad := (viewWidth - len(infoLine)) / 2
	if infoPad < 0 {
		infoPad = 0
	}
	b.WriteString(strings.Repeat(" ", infoPad))
	b.WriteString(dimStyle.Render(infoLine))
	b.WriteString("\n\n")

	// Center the image horizontally
	imgPad := (viewWidth - dstW) / 2
	if imgPad < 0 {
		imgPad = 0
	}
	padStr := strings.Repeat(" ", imgPad)

	for row := 0; row < dstH; row += 2 {
		b.WriteString(padStr)
		for col := 0; col < dstW; col++ {
			// Map to source coordinates (nearest neighbor)
			srcX := bounds.Min.X + col*srcW/dstW
			srcY1 := bounds.Min.Y + row*srcH/dstH
			srcY2 := bounds.Min.Y + (row+1)*srcH/dstH

			r1, g1, b1 := colorToRGB(img.At(srcX, srcY1))
			var r2, g2, b2 uint8
			if row+1 < dstH {
				r2, g2, b2 = colorToRGB(img.At(srcX, srcY2))
			}

			// ▀ with fg=top pixel, bg=bottom pixel
			b.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀\033[0m",
				r1, g1, b1, r2, g2, b2))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func colorToRGB(c color.Color) (uint8, uint8, uint8) {
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func renderImageFallback(path string) string {
	info, _ := os.Stat(path)
	sizeStr := ""
	if info != nil {
		sizeStr = formatFileSize(info.Size()) + " · "
	}
	return fmt.Sprintf("%s\n%simage file", filepath.Base(path), sizeStr)
}

func formatFileSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
	}
}

// --- ZIP / Office document support ---

// extractPdfStructured extracts text from a PDF, returning lines for RawLines/HighlightedLines.
func extractPdfStructured(path string) (rawLines []string, highlightedLines []string) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, nil
	}

	reader, err := gopdf.NewReader(f, info.Size())
	if err != nil {
		return nil, nil
	}

	numPages := reader.NumPage()
	header := fmt.Sprintf("PDF · %d page%s", numPages, pluralS(numPages))
	rawLines = append(rawLines, header, "")
	highlightedLines = append(highlightedLines, ansiColor256(241, header), "")

	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		if numPages > 1 {
			pageHeader := fmt.Sprintf("── page %d ──", i)
			rawLines = append(rawLines, pageHeader)
			highlightedLines = append(highlightedLines, ansiColor256(241, pageHeader))
		}
		for _, line := range strings.Split(strings.TrimRight(text, "\n\r "), "\n") {
			rawLines = append(rawLines, line)
			highlightedLines = append(highlightedLines, ansiColor256(252, line))
		}
		rawLines = append(rawLines, "")
		highlightedLines = append(highlightedLines, "")
	}

	return rawLines, highlightedLines
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// copyToClipboard writes text to the system clipboard.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		return fmt.Errorf("unsupported platform")
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// clearFlashAfter returns a command that sends flashClearMsg after a delay.
func clearFlashAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return flashClearMsg{} })
}

// getPreviewText returns the plain text content of the current preview.
func (m *Model) getPreviewText() string {
	if !m.preview.Valid {
		return ""
	}
	if len(m.preview.RawLines) > 0 {
		return strings.Join(m.preview.RawLines, "\n")
	}
	if m.preview.ImageRender != "" {
		return stripANSI(m.preview.ImageRender)
	}
	return ""
}

// stripANSI removes all ANSI escape sequences from a string.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
		} else {
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String()
}

func isZipBasedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".zip", ".docx", ".xlsx", ".pptx",
		".jar", ".war", ".ear",
		".odt", ".ods", ".odp",
		".epub":
		return true
	}
	return false
}

// renderZipPreview extracts readable content from zip-based files.
// For Office docs it extracts text; for plain zips it lists contents.
func renderZipPreview(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".docx":
		if text := extractZipXMLText(path, "word/document.xml"); text != "" {
			return text
		}
	case ".pptx":
		if text := extractPptxText(path); text != "" {
			return text
		}
	case ".xlsx":
		if text := extractXlsxText(path); text != "" {
			return text
		}
	case ".epub":
		// Could extract from OEBPS/content files, but list contents for now
	case ".odt":
		if text := extractZipXMLText(path, "content.xml"); text != "" {
			return text
		}
	}

	// Default: list zip contents
	return listZipContents(path)
}

func extractZipXMLText(zipPath, xmlFile string) string {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return ""
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == xmlFile {
			rc, err := f.Open()
			if err != nil {
				return ""
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			return extractTextFromXML(data)
		}
	}
	return ""
}

// extractDocxStructured extracts text from a docx with heading detection and paragraph spacing.
// Returns lines suitable for RawLines/HighlightedLines (wrappable, with gutters).
func extractDocxStructured(zipPath string) (rawLines []string, highlightedLines []string) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil
	}
	defer r.Close()

	var data []byte
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, nil
			}
			data, _ = io.ReadAll(rc)
			rc.Close()
			break
		}
	}
	if data == nil {
		return nil, nil
	}

	type paragraph struct {
		text  string
		style string // e.g. "Heading1", "Heading2", "Title", "ListParagraph"
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	var paragraphs []paragraph
	var currentText strings.Builder
	var currentStyle string
	inParagraph := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "p" {
				inParagraph = true
				currentText.Reset()
				currentStyle = ""
			}
			// Paragraph style: <w:pStyle w:val="Heading1"/>
			if t.Name.Local == "pStyle" && inParagraph {
				for _, attr := range t.Attr {
					if attr.Name.Local == "val" {
						currentStyle = attr.Value
					}
				}
			}
			// List bullet: <w:numPr> indicates a list item
			if t.Name.Local == "numPr" && inParagraph && currentStyle == "" {
				currentStyle = "ListParagraph"
			}
		case xml.EndElement:
			if t.Name.Local == "p" && inParagraph {
				text := strings.TrimSpace(currentText.String())
				paragraphs = append(paragraphs, paragraph{text: text, style: currentStyle})
				inParagraph = false
			}
		case xml.CharData:
			if inParagraph {
				s := string(t)
				if strings.TrimSpace(s) != "" {
					currentText.WriteString(s)
				}
			}
		}
	}

	// Convert paragraphs to styled lines
	for i, p := range paragraphs {
		if p.text == "" {
			// Empty paragraph = blank line separator
			rawLines = append(rawLines, "")
			highlightedLines = append(highlightedLines, "")
			continue
		}

		style := strings.ToLower(p.style)
		switch {
		case style == "title" || style == "heading1" || strings.HasPrefix(style, "heading1"):
			// Add blank line before headings (unless first)
			if i > 0 {
				rawLines = append(rawLines, "")
				highlightedLines = append(highlightedLines, "")
			}
			rawLines = append(rawLines, p.text)
			highlightedLines = append(highlightedLines, ansiBoldColor256(109, strings.ToUpper(p.text)))
		case style == "heading2" || strings.HasPrefix(style, "heading2"):
			if i > 0 {
				rawLines = append(rawLines, "")
				highlightedLines = append(highlightedLines, "")
			}
			rawLines = append(rawLines, p.text)
			highlightedLines = append(highlightedLines, ansiBoldColor256(109, p.text))
		case style == "heading3" || strings.HasPrefix(style, "heading3"),
			style == "heading4" || strings.HasPrefix(style, "heading4"):
			if i > 0 {
				rawLines = append(rawLines, "")
				highlightedLines = append(highlightedLines, "")
			}
			rawLines = append(rawLines, p.text)
			highlightedLines = append(highlightedLines, ansiBoldColor256(252, p.text))
		case style == "listparagraph" || strings.Contains(style, "list"):
			raw := "  • " + p.text
			rawLines = append(rawLines, raw)
			highlightedLines = append(highlightedLines, ansiColor256(241, "  • ")+ansiColor256(252, p.text))
		default:
			// Body text — add paragraph break after if next is also body
			rawLines = append(rawLines, p.text)
			highlightedLines = append(highlightedLines, ansiColor256(252, p.text))
			// Add blank line between body paragraphs for readability
			if i+1 < len(paragraphs) {
				nextStyle := strings.ToLower(paragraphs[i+1].style)
				nextIsBody := nextStyle == "" && paragraphs[i+1].text != ""
				if nextIsBody {
					rawLines = append(rawLines, "")
					highlightedLines = append(highlightedLines, "")
				}
			}
		}
	}

	return rawLines, highlightedLines
}

func extractPptxText(zipPath string) string {
	rawLines, _ := extractPptxStructured(zipPath)
	return strings.Join(rawLines, "\n")
}

// extractPptxStructured extracts slide content with title detection and dividers.
func extractPptxStructured(zipPath string) (rawLines []string, highlightedLines []string) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil
	}
	defer r.Close()

	// Collect slide files and sort them
	var slideFiles []*zip.File
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideFiles = append(slideFiles, f)
		}
	}
	sort.Slice(slideFiles, func(i, j int) bool {
		return slideFiles[i].Name < slideFiles[j].Name
	})

	header := fmt.Sprintf("%d slide%s", len(slideFiles), pluralS(len(slideFiles)))
	rawLines = append(rawLines, header, "")
	highlightedLines = append(highlightedLines, ansiColor256(241, header), "")

	for _, f := range slideFiles {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()

		slideNum := strings.TrimPrefix(f.Name, "ppt/slides/slide")
		slideNum = strings.TrimSuffix(slideNum, ".xml")

		shapes := extractPptxShapes(data)
		if len(shapes) == 0 {
			continue
		}

		// Slide divider
		divider := fmt.Sprintf("━━ slide %s ━━", slideNum)
		rawLines = append(rawLines, divider)
		highlightedLines = append(highlightedLines, ansiBoldColor256(109, divider))

		for _, shape := range shapes {
			text := strings.TrimSpace(shape.text)
			if text == "" {
				continue
			}
			switch {
			case shape.isTitle:
				rawLines = append(rawLines, text)
				highlightedLines = append(highlightedLines, ansiBoldColor256(252, text))
			case shape.isSubtitle:
				rawLines = append(rawLines, text)
				highlightedLines = append(highlightedLines, ansiColor256(245, text))
			default:
				// Body text — split into lines for proper wrapping
				for _, line := range strings.Split(text, "\n") {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					rawLines = append(rawLines, "  "+line)
					highlightedLines = append(highlightedLines, "  "+ansiColor256(252, line))
				}
			}
		}
		rawLines = append(rawLines, "")
		highlightedLines = append(highlightedLines, "")
	}

	return rawLines, highlightedLines
}

type pptxShape struct {
	text       string
	isTitle    bool
	isSubtitle bool
}

// extractPptxShapes parses a slide XML and returns text shapes with type info.
func extractPptxShapes(data []byte) []pptxShape {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var shapes []pptxShape
	var currentText strings.Builder
	isTitle := false
	isSubtitle := false
	inShape := false
	depth := 0

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "sp" {
				inShape = true
				depth = 0
				isTitle = false
				isSubtitle = false
				currentText.Reset()
			}
			if inShape {
				depth++
			}
			// Detect placeholder type
			if t.Name.Local == "ph" && inShape {
				for _, attr := range t.Attr {
					if attr.Name.Local == "type" {
						switch attr.Value {
						case "title", "ctrTitle":
							isTitle = true
						case "subTitle":
							isSubtitle = true
						}
					}
				}
			}
			// New paragraph within shape = newline
			if t.Name.Local == "p" && inShape && currentText.Len() > 0 {
				currentText.WriteString("\n")
			}
		case xml.EndElement:
			if t.Name.Local == "sp" && inShape {
				text := currentText.String()
				if strings.TrimSpace(text) != "" {
					shapes = append(shapes, pptxShape{
						text:       text,
						isTitle:    isTitle,
						isSubtitle: isSubtitle,
					})
				}
				inShape = false
			}
			if inShape {
				depth--
			}
		case xml.CharData:
			if inShape {
				s := strings.TrimSpace(string(t))
				if s != "" {
					currentText.WriteString(s)
					currentText.WriteString(" ")
				}
			}
		}
	}
	return shapes
}

// extractOdtStructured extracts text from an ODT with heading detection.
// ODT uses <text:h> for headings (with outline-level) and <text:p> for paragraphs.
func extractOdtStructured(zipPath string) (rawLines []string, highlightedLines []string) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil
	}
	defer r.Close()

	var data []byte
	for _, f := range r.File {
		if f.Name == "content.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, nil
			}
			data, _ = io.ReadAll(rc)
			rc.Close()
			break
		}
	}
	if data == nil {
		return nil, nil
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	var currentText strings.Builder
	inHeading := false
	headingLevel := 0
	inParagraph := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "h" {
				inHeading = true
				headingLevel = 1
				currentText.Reset()
				for _, attr := range t.Attr {
					if attr.Name.Local == "outline-level" {
						fmt.Sscanf(attr.Value, "%d", &headingLevel)
					}
				}
			}
			if t.Name.Local == "p" && !inHeading {
				inParagraph = true
				currentText.Reset()
			}
		case xml.EndElement:
			if t.Name.Local == "h" && inHeading {
				text := strings.TrimSpace(currentText.String())
				if text != "" {
					rawLines = append(rawLines, "", text)
					switch {
					case headingLevel <= 1:
						highlightedLines = append(highlightedLines, "", ansiBoldColor256(109, strings.ToUpper(text)))
					case headingLevel == 2:
						highlightedLines = append(highlightedLines, "", ansiBoldColor256(109, text))
					default:
						highlightedLines = append(highlightedLines, "", ansiBoldColor256(252, text))
					}
				}
				inHeading = false
			}
			if t.Name.Local == "p" && inParagraph {
				text := strings.TrimSpace(currentText.String())
				if text != "" {
					rawLines = append(rawLines, text)
					highlightedLines = append(highlightedLines, ansiColor256(252, text))
				}
				inParagraph = false
			}
		case xml.CharData:
			if inHeading || inParagraph {
				s := string(t)
				if strings.TrimSpace(s) != "" {
					currentText.WriteString(s)
				}
			}
		}
	}

	return rawLines, highlightedLines
}

func extractXlsxText(zipPath string) string {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return ""
	}
	defer r.Close()

	// First extract shared strings
	var sharedStrings []string
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				break
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			sharedStrings = parseXlsxSharedStrings(data)
			break
		}
	}

	// Then extract sheet data
	var result strings.Builder
	var sheetFiles []*zip.File
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			sheetFiles = append(sheetFiles, f)
		}
	}
	sort.Slice(sheetFiles, func(i, j int) bool {
		return sheetFiles[i].Name < sheetFiles[j].Name
	})

	for _, f := range sheetFiles {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		rows := extractXlsxSheetRows(data, sharedStrings)
		if len(rows) > 0 {
			sheetNum := strings.TrimPrefix(f.Name, "xl/worksheets/sheet")
			sheetNum = strings.TrimSuffix(sheetNum, ".xml")
			result.WriteString(ansiColor256(241, fmt.Sprintf("── sheet %s ──", sheetNum)) + "\n")
			result.WriteString(renderTable(rows))
			result.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(result.String())
}

func parseXlsxSharedStrings(data []byte) []string {
	var strings_ []string
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var current strings.Builder
	inSI := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "si" {
				inSI = true
				current.Reset()
			}
		case xml.EndElement:
			if t.Name.Local == "si" {
				strings_ = append(strings_, current.String())
				inSI = false
			}
		case xml.CharData:
			if inSI {
				current.Write(t)
			}
		}
	}
	return strings_
}

func extractXlsxSheetRows(data []byte, sharedStrings []string) [][]string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var rows [][]string
	var currentRow []string
	var cellValue strings.Builder
	var cellType string
	inRow := false
	inValue := false

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "row" {
				inRow = true
				currentRow = nil
			} else if t.Name.Local == "c" {
				cellType = ""
				for _, attr := range t.Attr {
					if attr.Name.Local == "t" {
						cellType = attr.Value
					}
				}
				cellValue.Reset()
			} else if t.Name.Local == "v" || t.Name.Local == "t" {
				inValue = true
			}
		case xml.EndElement:
			if t.Name.Local == "v" || t.Name.Local == "t" {
				inValue = false
			} else if t.Name.Local == "row" {
				if inRow && len(currentRow) > 0 {
					rows = append(rows, currentRow)
				}
				inRow = false
			} else if t.Name.Local == "c" {
				val := cellValue.String()
				if cellType == "s" {
					idx := 0
					fmt.Sscanf(val, "%d", &idx)
					if idx < len(sharedStrings) {
						val = sharedStrings[idx]
					}
				}
				currentRow = append(currentRow, val)
			}
		case xml.CharData:
			if inValue {
				cellValue.Write(t)
			}
		}
	}
	return rows
}

// ansiTrimLeft skips n visible columns from the left while preserving ANSI escapes.
func ansiTrimLeft(s string, n int) string {
	var out strings.Builder
	col := 0
	i := 0
	// First pass: skip n visible columns, but emit any ANSI sequences encountered
	for i < len(s) && col < n {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// ANSI escape — collect and emit (these set color state)
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++ // include the final letter
			}
			out.WriteString(s[i:j])
			i = j
		} else {
			// Visible character — skip it (advance by full rune, not single byte)
			_, size := utf8.DecodeRuneInString(s[i:])
			col++
			i += size
		}
	}
	// Emit the rest
	out.WriteString(s[i:])
	return out.String()
}

// ANSI color helpers for goroutine-safe rendering (lipgloss needs TTY context)
func ansiColor256(code int, text string) string {
	return fmt.Sprintf("\x1b[38;5;%dm%s\x1b[0m", code, text)
}

func ansiBoldColor256(code int, text string) string {
	return fmt.Sprintf("\x1b[1;38;5;%dm%s\x1b[0m", code, text)
}

// Rainbow column palette — distinct, readable colors on dark backgrounds
var rainbowCols = []int{
	109, // cyan
	179, // yellow
	174, // salmon
	114, // green
	141, // purple
	215, // orange
	74,  // blue
	218, // pink
	150, // lime
	183, // lavender
}

// renderTable renders rows as a rainbow-colored aligned table (each column = one color).
// Uses raw ANSI escapes because this runs in async goroutines without TTY context.
func renderTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	// Find max columns
	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	// Calculate column widths
	colWidths := make([]int, maxCols)
	for _, row := range rows {
		for i, cell := range row {
			w := len(strings.TrimSpace(cell))
			if w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	// Cap column widths
	for i := range colWidths {
		if colWidths[i] > 30 {
			colWidths[i] = 30
		}
		if colWidths[i] < 2 {
			colWidths[i] = 2
		}
	}

	sep := ansiColor256(241, " │ ")

	var b strings.Builder
	for rowIdx, row := range rows {
		for colIdx := 0; colIdx < maxCols; colIdx++ {
			if colIdx > 0 {
				b.WriteString(sep)
			}

			cell := ""
			if colIdx < len(row) {
				cell = strings.TrimSpace(row[colIdx])
			}

			// Truncate if needed
			if len(cell) > colWidths[colIdx] {
				cell = cell[:colWidths[colIdx]-1] + "…"
			}

			// Pad to column width
			pad := colWidths[colIdx] - len(cell)
			if pad < 0 {
				pad = 0
			}
			padded := cell + strings.Repeat(" ", pad)

			color := rainbowCols[colIdx%len(rainbowCols)]
			if rowIdx == 0 {
				b.WriteString(ansiBoldColor256(color, padded))
			} else {
				b.WriteString(ansiColor256(color, padded))
			}
		}
		b.WriteString("\n")

		// Separator after header
		if rowIdx == 0 {
			for colIdx := 0; colIdx < maxCols; colIdx++ {
				if colIdx > 0 {
					b.WriteString(ansiColor256(241, "─┼─"))
				}
				b.WriteString(ansiColor256(241, strings.Repeat("─", colWidths[colIdx])))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '.' || c == '-' || c == '+' || c == ',' || c == '%' || c == '$' {
			continue
		}
		if (c == 'e' || c == 'E') && i > 0 {
			continue
		}
		return false
	}
	return true
}

// extractTextFromXML extracts text content from Office XML, with newlines at paragraph boundaries.
func extractTextFromXML(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var result strings.Builder

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.EndElement:
			// Insert newline at paragraph boundaries
			if t.Name.Local == "p" || t.Name.Local == "si" {
				result.WriteString("\n")
			}
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				result.WriteString(text)
				result.WriteString(" ")
			}
		}
	}

	// Clean up multiple blank lines
	text := result.String()
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

func listZipContents(path string) string {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "couldn't read archive"
	}
	defer r.Close()

	info, _ := os.Stat(path)
	var header string
	fileCount := 0
	for _, f := range r.File {
		if !f.FileInfo().IsDir() {
			fileCount++
		}
	}
	if info != nil {
		header = fmt.Sprintf("%s  %s  %d files\n", filepath.Base(path), formatFileSize(info.Size()), fileCount)
	}

	var lines []string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		size := formatFileSize(int64(f.UncompressedSize64))
		lines = append(lines, fmt.Sprintf("  %8s  %s", size, f.Name))
	}

	return header + strings.Join(lines, "\n")
}

// View implements tea.Model
func (m Model) View() string {
	// Show loading screen instantly, even before dimensions arrive
	if m.loading {
		if m.width == 0 || m.height == 0 {
			// Minimal instant display before window size is known
			return "\n\n  " + cyanStyle.Render("PERCH") + "\n"
		}
		return m.renderLoadingScreen()
	}

	if m.width == 0 || m.height == 0 {
		return ""
	}

	var b strings.Builder

	// === FILE LIST ===
	b.WriteString(m.renderFileList())

	// === DIVIDER ===
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)) + "\n")

	// === PREVIEW HEADER ===
	b.WriteString(m.renderPreviewHeader())
	b.WriteString(dividerStyle.Render(strings.Repeat("─", m.width)) + "\n")

	// === VIEWPORT (preview content with scroll indicators) ===
	b.WriteString(m.renderPreviewWithIndicators())

	// === FOOTER ===
	b.WriteString(m.renderFooter())

	return b.String()
}

// renderPreviewWithIndicators renders the viewport with scroll indicators at display level
func (m Model) renderPreviewWithIndicators() string {
	var lines []string

	// Check if we should show top indicator
	showTopDots := m.viewport.YOffset > 0

	// Check if we should show bottom indicator
	totalContentLines := m.viewport.TotalLineCount()
	visibleEnd := m.viewport.YOffset + m.viewport.Height
	showBottomDots := visibleEnd < totalContentLines

	// Top indicator (or empty line to maintain layout)
	if showTopDots {
		lines = append(lines, cyanStyle.Render("  ..."))
	} else {
		lines = append(lines, "") // Empty line to maintain layout
	}

	// Main viewport content
	lines = append(lines, m.viewport.View())

	// Bottom indicator (or empty line to maintain layout)
	if showBottomDots {
		lines = append(lines, cyanStyle.Render("  ..."))
	} else {
		lines = append(lines, "") // Empty line to maintain layout
	}

	return strings.Join(lines, "\n") + "\n"
}

func (m Model) renderFileList() string {
	var lines []string

	// Header line: sparkle + "PERCHED ON PROGRESS" on left, "...path" on right
	var sparkle string
	if m.sparkleOn {
		sparkle = sparkleStyle.Render("✧")
	} else {
		sparkle = " " // invisible when off
	}
	shortPath := truncatePath(m.dir, 2)
	devMarker := ""
	if DevBuild {
		devMarker = dimStyle.Render("[dev] ")
	}
	header := devMarker + dimStyle.Render("PERCHED ON PROGRESS") + " " + sparkle
	pathHint := dimStyle.Render("..." + shortPath)
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
	hint := keyStyle.Render("h l") + dimStyle.Render(" pan  ") + keyStyle.Render("j k") + dimStyle.Render(" scroll  ")
	return padLine(header, hint, m.width) + "\n"
}

func (m Model) renderFooter() string {
	if m.flashMsg != "" && time.Now().Before(m.flashExpiry) {
		leftHint := cyanStyle.Render("  ✓ " + m.flashMsg)
		rightHint := keyStyle.Render("q") + dimStyle.Render(" quit  ")
		return padLine(leftHint, rightHint, m.width)
	}
	leftHint := keyStyle.Render("  c") + dimStyle.Render(" copy  ") + keyStyle.Render("p") + dimStyle.Render(" path  ")
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
	var lines []string

	// Block ASCII art for PERCH
	ascii := []string{
		"██████╗ ███████╗██████╗  ██████╗██╗  ██╗",
		"██╔══██╗██╔════╝██╔══██╗██╔════╝██║  ██║",
		"██████╔╝█████╗  ██████╔╝██║     ███████║",
		"██╔═══╝ ██╔══╝  ██╔══██╗██║     ██╔══██║",
		"██║     ███████╗██║  ██║╚██████╗██║  ██║",
		"╚═╝     ╚══════╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝",
	}
	asciiWidth := 40

	// Content height: ascii (6) + gap + version + gap + tagline = 10
	totalContentHeight := 10
	topPad := (m.height - totalContentHeight) / 2
	if topPad < 0 {
		topPad = 0
	}

	for i := 0; i < topPad; i++ {
		lines = append(lines, "")
	}

	// PERCH ASCII in cyan
	asciiPad := (m.width - asciiWidth) / 2
	if asciiPad < 0 {
		asciiPad = 0
	}
	for _, line := range ascii {
		lines = append(lines, strings.Repeat(" ", asciiPad)+cyanStyle.Render(line))
	}

	// Version
	versionStr := dimStyle.Render(Version)
	versionPad := (m.width - len(Version)) / 2
	if versionPad < 0 {
		versionPad = 0
	}
	lines = append(lines, strings.Repeat(" ", versionPad)+versionStr)

	lines = append(lines, "") // gap

	// Tagline
	tagline := dimStyle.Render("let's keep an eye on the progress.")
	taglinePad := (m.width - 34) / 2
	if taglinePad < 0 {
		taglinePad = 0
	}
	lines = append(lines, strings.Repeat(" ", taglinePad)+tagline)

	// Pad to leave room for attribution at bottom
	for len(lines) < m.height-1 {
		lines = append(lines, "")
	}

	// Attribution at bottom
	attr := dimStyle.Render("@kateleext")
	attrPad := (m.width - 10) / 2
	if attrPad < 0 {
		attrPad = 0
	}
	lines = append(lines, strings.Repeat(" ", attrPad)+attr)

	return strings.Join(lines, "\n")
}