package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kateleext/perch/internal/ui"
)

func main() {
	// Get directory from args or use current
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	// Convert to absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Printf("Error resolving path: %v\n", err)
		os.Exit(1)
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		fmt.Printf("Not a valid directory: %s\n", absDir)
		os.Exit(1)
	}

	// Check if it's a git repo
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = absDir
	if err := cmd.Run(); err != nil {
		fmt.Printf("Not a git repository: %s\n", absDir)
		os.Exit(1)
	}

	// Check if this is a dev build
	if os.Getenv("PERCH_DEV") == "1" {
		ui.DevBuild = true
	}

	// Create and run the TUI
	p := tea.NewProgram(
		ui.New(absDir),
		tea.WithAltScreen(),
		tea.WithMouseAllMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
