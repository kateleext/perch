package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kateleext/perch/internal/ui"
)

func main() {
	// Get directory from args or use current
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	// Create and run the TUI
	p := tea.NewProgram(
		ui.New(dir),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
