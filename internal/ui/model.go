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

// PreviewContent holds the rendered preview data for a specific file
type PreviewContent struct {
	Valid          bool             // whether there's content to show
	Message        string           // message for deleted/unsupported files (mutually exclusive with Lines)
	Lines          []string         // syntax highlighted lines
	RawLines       []string         // original unhighlighted lines for diff display
	DiffLines      map[int]string   // line number -> "added" or "deleted"
	DiffStats      git.DiffStats
	IsMarkdown     bool
}

// Model is the main bubbletea model
type Model struct {
	files          []git.FileStatus
	selected       int
	listScroll     int
	previewScroll  int            // manual scroll position for preview content
	dir            string
	gitRoot        string
	width          int
	height         int
	listHeight     int
	previewReady   bool
	dragging       bool
	dividerY       int
	preview        PreviewContent  // immutable content for currently selected file
	sparkleOn      bool             // sparkle animation state
}

// New creates a new UI model
func New(dir string) Model {
	gitRoot, _ := git.GetGitRoot(dir)
	return Model{
		dir:        dir,
		gitRoot:    gitRoot,
		listHeight: 8,
		preview:    PreviewContent{},
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
				m.previewScroll = 0
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
				m.previewScroll = 0
				m.updatePreview()
			}
		case "j":
			m.previewScroll++
		case "k":
			if m.previewScroll > 0 {
				m.previewScroll--
			}
		case "g":
			m.previewScroll = 0
		case "G":
			// Scroll to bottom - calculate max scroll
			if len(m.preview.Lines) > 0 {
				previewHeight := m.height - m.listHeight - 4
				if previewHeight < 1 {
					previewHeight = 1
				}
				m.previewScroll = len(m.preview.Lines) - previewHeight
				if m.previewScroll < 0 {
					m.previewScroll = 0
				}
			}
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
				if m.previewScroll > 0 {
					m.previewScroll--
				}
			}
		case tea.MouseWheelDown:
			if msg.Y > m.dividerY {
				m.previewScroll++
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
	m.dividerY = m.listHeight
	m.previewReady = true
	m.updatePreview()
}

func (m *Model) updatePreview() {
	if !m.previewReady || len(m.files) == 0 {
		m.preview = PreviewContent{}
		return
	}

	file := m.files[m.selected]
	fullPath := filepath.Join(m.dir, file.Path)

	// Check if file was deleted
	if strings.Contains(file.GitCode, "D") {
		m.preview = PreviewContent{
			Valid:   true,
			Message: fmt.Sprintf("%s was deleted", file.Path),
		}
		m.previewScroll = 0
		return
	}

	// Check if file type is unsupported for preview
	if isUnsupportedFile(file.Path) {
		reason := "not supported in perch"
		if filepath.Ext(file.Path) == "" {
			reason = "no file extension — open in your editor"
		}
		m.preview = PreviewContent{
			Valid:   true,
			Message: fmt.Sprintf("%s\n%s", filepath.Base(file.Path), reason),
		}
		m.previewScroll = 0
		return
	}

	// Get diff info for uncommitted files (use file's GitRoot for git commands)
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
		m.preview = PreviewContent{
			Valid:   true,
			Message: fmt.Sprintf("couldn't read %s", file.Path),
		}
		m.previewScroll = 0
		return
	}

	// Parse raw lines
	rawLines := strings.Split(string(content), "\n")

	// Syntax highlight the content
	highlightedLines := highlightCode(string(content), file.Path)

	// Build and store the preview content atomically
	m.preview = PreviewContent{
		Valid:      true,
		Lines:      highlightedLines,
		RawLines:   rawLines,
		DiffLines:  diffLines,
		DiffStats:  diffStats,
		IsMarkdown: strings.HasSuffix(strings.ToLower(file.Path), ".md"),
	}

	// Reset scroll to top when viewing new file
	m.previewScroll = 0
}

// highlightCode returns syntax-highlighted lines for the given content
func highlightCode(content, filename string) []string {
	lines := strings.Split(content, "\n")
	
	// Skip highlighting for Go files - they have rendering issues with Chroma's formatter
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".go" {
		return lines
	}
	
	// Get lexer for file type
	lexer := lexers.Match(filename)

	// Handle ERB files explicitly
	if strings.HasSuffix(filename, ".erb") {
		lexer = lexers.Get("erb")
		if lexer == nil {
			lexer = lexers.Get("html")
		}
	}

	// If no lexer found, return plaintext (don't use Fallback which produces broken ANSI)
	if lexer == nil {
		return lines
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

	// Highlight each line independently to avoid ANSI sequence corruption
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

		// Trim trailing newline from formatter output
		highlighted := strings.TrimSuffix(buf.String(), "\n")
		highlightedLines[i] = highlighted
	}

	return highlightedLines
}

// isUnsupportedFile returns true for binary/non-text files
func isUnsupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	
	// Files with no extension
	if ext == "" {
		return true
	}
	
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
// Two-pane layout: top=file list, bottom=preview, footer=quit hint
func (m Model) View() string {
	snap := m.captureSnapshot()
	return renderViewClean(snap, m)
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
	padding := m.width - leftLen - rightLen - 1  // Note: still uses m.width for consistency with model state
	if padding < 1 {
		padding = 1
	}
	return left + strings.Repeat(" ", padding) + right
}
