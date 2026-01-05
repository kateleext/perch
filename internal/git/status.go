package git

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileStatus represents a file's git status
type FileStatus struct {
	Status   string // "uncommitted" or "committed"
	Path     string
	Commit   string // short hash for committed files
	TimeAgo  string // "2 hours ago" for committed files
	IsFile   bool   // true if it's a file (not directory)
}

// GetStatus returns files from git status and recent commits
func GetStatus(dir string) ([]FileStatus, error) {
	var files []FileStatus
	seen := make(map[string]bool)

	// Get uncommitted files first
	uncommitted, err := getUncommitted(dir)
	if err != nil {
		return nil, err
	}
	for _, f := range uncommitted {
		if !seen[f.Path] {
			files = append(files, f)
			seen[f.Path] = true
		}
	}

	// Get recently committed files
	committed, err := getRecentlyCommitted(dir)
	if err != nil {
		return nil, err
	}
	for _, f := range committed {
		if !seen[f.Path] {
			files = append(files, f)
			seen[f.Path] = true
		}
	}

	return files, nil
}

func getUncommitted(dir string) ([]FileStatus, error) {
	cmd := exec.Command("git", "status", "--porcelain", "-uall")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []FileStatus
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			continue
		}
		path := line[3:]

		// Check if it's a file
		fullPath := filepath.Join(dir, path)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		files = append(files, FileStatus{
			Status: "uncommitted",
			Path:   path,
			IsFile: true,
		})
	}

	return files, nil
}

func getRecentlyCommitted(dir string) ([]FileStatus, error) {
	// Get last 5 commits with files
	cmd := exec.Command("git", "log", "--name-only", "--pretty=format:%h|%ar", "-n", "5")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []FileStatus
	var currentCommit, currentTime string

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Check if it's a commit line (contains |)
		if strings.Contains(line, "|") {
			parts := strings.SplitN(line, "|", 2)
			currentCommit = parts[0]
			currentTime = parts[1]
			continue
		}

		// It's a file path
		if currentCommit == "" {
			continue
		}

		// Check if file still exists and is a file
		fullPath := filepath.Join(dir, line)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		files = append(files, FileStatus{
			Status:  "committed",
			Path:    line,
			Commit:  currentCommit,
			TimeAgo: currentTime,
			IsFile:  true,
		})
	}

	return files, nil
}
