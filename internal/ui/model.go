package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/takumahq/drift/internal/git"
)

// Layout determines pane arrangement
type Layout int

const (
	Horizontal Layout = iota // list | preview
	Vertical                 // list / preview
)

// Model is the bubbletea model
type Model struct {
	files       []git.FileStatus
	selected    int
	scrollY     int
	layout      Layout
	width       int
	height      int
	previewText string
}

// New creates a new UI model
func New() Model {
	return Model{
		layout: Vertical,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			// scroll preview up
		case "down", "j":
			// scroll preview down
		case "v":
			// toggle layout
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	// TODO: render file list
	// TODO: render preview pane
	// TODO: apply layout
	return "drift"
}
