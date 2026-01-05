package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kateleext/perch/internal/git"
	"github.com/kateleext/perch/internal/highlight"
)

// Styles
var (
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cyanStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	dividerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// Model is the bubbletea model
type Model struct {
	files        []git.FileStatus
	selected     int
	dir          string
	width        int
	height       int
	viewport     viewport.Model
	previewReady bool
}

// New creates a new UI model
func New(dir string) Model {
	return Model{
		dir: dir,
	}
}

// RefreshMsg tells the model to refresh files
type RefreshMsg struct{}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return m.loadFiles
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
				m.viewport.GotoTop()
				m.updatePreview()
			}
		case "down":
			if m.selected < len(m.files)-1 {
				m.selected++
				m.viewport.GotoTop()
				m.updatePreview()
			}
		case "j":
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		case "k":
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		case "g":
			m.viewport.GotoTop()
		case "G":
			m.viewport.GotoBottom()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// File list takes ~12 lines, rest is preview
		previewHeight := m.height - 14
		if previewHeight < 5 {
			previewHeight = 5
		}

		m.viewport = viewport.New(m.width, previewHeight)
		m.viewport.Style = lipgloss.NewStyle()
		m.previewReady = true
		m.updatePreview()

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
	}

	return m, nil
}

func (m *Model) updatePreview() {
	if !m.previewReady || len(m.files) == 0 {
		return
	}

	file := m.files[m.selected]
	fullPath := filepath.Join(m.dir, file.Path)

	// Get highlighted content
	lineCount, _ := highlight.LineCount(fullPath)
	content, err := highlight.HighlightFile(fullPath, 1, lineCount)
	if err != nil {
		content = fmt.Sprintf("Error: %v", err)
	}

	m.viewport.SetContent(content)
}

// View implements tea.Model
func (m Model) View() string {
	var b strings.Builder

	// Header
	b.WriteString(dimStyle.Render("✦") + " perch\n\n")

	// File list
	if len(m.files) == 0 {
		b.WriteString(dimStyle.Render("  no changes") + "\n")
	} else {
		for i, f := range m.files {
			icon := "✓ "
			if f.Status == "uncommitted" {
				icon = "○ "
			}

			if i == m.selected {
				b.WriteString(selectedStyle.Render("› "+icon+f.Path) + "\n")
			} else {
				b.WriteString("  " + dimStyle.Render(icon) + f.Path + "\n")
			}
		}
	}

	b.WriteString("\n")

	// Divider
	divider := strings.Repeat("─", m.width-2)
	b.WriteString(dividerStyle.Render(divider) + "\n\n")

	// Preview header
	if len(m.files) > 0 {
		file := m.files[m.selected]
		basename := filepath.Base(file.Path)
		context := "has changes"
		if file.Status == "committed" {
			context = file.TimeAgo + " · " + file.Commit
		}
		b.WriteString(cyanStyle.Render(basename) + "  " + dimStyle.Render(context) + "\n\n")

		// Preview content
		if m.previewReady {
			b.WriteString(m.viewport.View())
		}
	}

	// Footer
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("↑↓ browse · j k scroll · g top · q quit"))

	return b.String()
}
