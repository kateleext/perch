package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// gitCmd creates a git command with --no-optional-locks to avoid lock contention
func gitCmd(args ...string) *exec.Cmd {
	fullArgs := append([]string{"--no-optional-locks"}, args...)
	return exec.Command("git", fullArgs...)
}

// FileStatus represents a file's git status
type FileStatus struct {
	Status   string    // "uncommitted" or "committed"
	GitCode  string    // "??", "M ", "A ", etc. for uncommitted files
	Path     string    // display path (relative to target directory)
	FullPath string    // path relative to GitRoot (for git commands)
	GitRoot  string    // git root for this file (may differ for submodules)
	Commit   string    // short hash for committed files
	TimeAgo  string    // "2 hours ago" for committed files
	IsFile   bool      // true if it's a file (not directory)
	ModTime  time.Time // file modification time for sorting
}

// ChangeType returns a human-readable description of the change
func (f FileStatus) ChangeType() string {
	if f.Status == "committed" {
		return f.TimeAgo + " · " + f.Commit
	}

	// Parse git status code
	switch {
	case f.GitCode == "??" || f.GitCode == "A " || f.GitCode == "AM":
		return "new file"
	case strings.Contains(f.GitCode, "D"):
		return "deleted"
	case strings.Contains(f.GitCode, "R"):
		return "renamed"
	default:
		return "modified"
	}
}

// DiffStats holds line addition/deletion counts
type DiffStats struct {
	Added   int
	Deleted int
}

// shouldSkipFile returns true for temp/binary files that shouldn't be displayed
func shouldSkipFile(path string) bool {
	base := filepath.Base(path)
	// Vim swap files
	if strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, ".swo") {
		return true
	}
	// Emacs backup files
	if strings.HasSuffix(base, "~") {
		return true
	}
	// macOS metadata
	if base == ".DS_Store" {
		return true
	}
	// Xcode user state (binary)
	if strings.HasSuffix(path, ".xcuserstate") {
		return true
	}
	return false
}

// GetDiffStats returns +/- line counts for a file
func GetDiffStats(dir, path string) DiffStats {
	cmd := gitCmd("diff", "--numstat", "--", path)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		// Try for untracked files - compare to empty
		cmd = gitCmd("diff", "--numstat", "/dev/null", path)
		cmd.Dir = dir
		output, _ = cmd.Output()
	}

	stats := DiffStats{}
	line := strings.TrimSpace(string(output))
	if line == "" {
		return stats
	}

	parts := strings.Fields(line)
	if len(parts) >= 2 {
		fmt.Sscanf(parts[0], "%d", &stats.Added)
		fmt.Sscanf(parts[1], "%d", &stats.Deleted)
	}
	return stats
}

// GetDiffLines returns which lines were added/deleted/unchanged
func GetDiffLines(dir, path string) map[int]string {
	result := make(map[int]string)

	// Get unified diff
	cmd := gitCmd("diff", "-U0", "--", path)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	lineNum := 0
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()

		// Parse hunk header: @@ -start,count +start,count @@
		if strings.HasPrefix(line, "@@") {
			// Extract the +start number
			parts := strings.Split(line, "+")
			if len(parts) >= 2 {
				numPart := strings.Split(parts[1], ",")[0]
				numPart = strings.Split(numPart, " ")[0]
				fmt.Sscanf(numPart, "%d", &lineNum)
				lineNum-- // Will be incremented on first content line
			}
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			lineNum++
			result[lineNum] = "added"
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			// Deleted lines don't have a line number in new file
			result[lineNum] = "deleted"
		} else if !strings.HasPrefix(line, "\\") && lineNum > 0 {
			lineNum++
		}
	}

	return result
}

// GetGitRoot returns the root of the git repository
func GetGitRoot(dir string) (string, error) {
	cmd := gitCmd("rev-parse", "--show-toplevel")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetSubmodules returns paths of submodules within a directory
func GetSubmodules(dir string) []string {
	cmd := gitCmd("submodule", "status", "--recursive")
	cmd.Dir = dir

	// Capture stdout separately - git may output valid data before hitting errors
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Run() // Ignore exit code - parse whatever stdout we got

	var submodules []string
	scanner := bufio.NewScanner(strings.NewReader(stdout.String()))
	for scanner.Scan() {
		line := scanner.Text()
		// Format: " hash path (branch)" or "+hash path (branch)"
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			submodules = append(submodules, parts[1])
		}
	}
	return submodules
}

// GetStatus returns files from git status and recent commits
func GetStatus(dir string) ([]FileStatus, error) {
	var files []FileStatus
	seen := make(map[string]bool)

	// Find git root and calculate prefix for filtering
	gitRoot, err := GetGitRoot(dir)
	if err != nil {
		return nil, err
	}

	// Use gitRoot's case as canonical (git returns correct case)
	// Check if dir is same as gitRoot (case-insensitive for macOS)
	relPrefix := ""
	if !strings.EqualFold(dir, gitRoot) {
		// Rebuild dir with gitRoot's case prefix for accurate Rel calculation
		if strings.HasPrefix(strings.ToLower(dir), strings.ToLower(gitRoot)) {
			dir = gitRoot + dir[len(gitRoot):]
		}
		relPrefix, _ = filepath.Rel(gitRoot, dir)
		if relPrefix != "" && !strings.HasSuffix(relPrefix, "/") {
			relPrefix += "/"
		}
	}

	// Get uncommitted files first
	uncommitted, err := getUncommitted(gitRoot, relPrefix, gitRoot)
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
	committed, err := getRecentlyCommitted(gitRoot, relPrefix, gitRoot)
	if err != nil {
		return nil, err
	}
	for _, f := range committed {
		if !seen[f.Path] {
			files = append(files, f)
			seen[f.Path] = true
		}
	}

	// Check submodules within target directory
	submodules := GetSubmodules(gitRoot)
	for _, subPath := range submodules {
		subFullPath := filepath.Join(gitRoot, subPath)

		// Only process submodules within our target directory
		if !strings.HasPrefix(subFullPath, dir) {
			continue
		}

		// Get files from this submodule
		subFiles, err := getNestedRepoFiles(subFullPath, dir)
		if err != nil {
			continue
		}
		for _, f := range subFiles {
			if !seen[f.Path] {
				files = append(files, f)
				seen[f.Path] = true
			}
		}
	}

	// Also check for nested git repos that aren't submodules
	nestedRepos := findNestedRepos(dir, gitRoot)
	for _, repoPath := range nestedRepos {
		subFiles, err := getNestedRepoFiles(repoPath, dir)
		if err != nil {
			continue
		}
		for _, f := range subFiles {
			if !seen[f.Path] {
				files = append(files, f)
				seen[f.Path] = true
			}
		}
	}

	// Sort by modification time (most recent first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

// findNestedRepos finds git repositories nested within a directory that aren't submodules
func findNestedRepos(dir, parentGitRoot string) []string {
	var repos []string
	submodules := make(map[string]bool)

	// Get list of known submodules to exclude
	for _, sub := range GetSubmodules(parentGitRoot) {
		submodules[filepath.Join(parentGitRoot, sub)] = true
	}

	// Walk directory looking for .git directories
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return filepath.SkipDir
		}

		// Skip if this is a submodule directory
		if submodules[path] {
			return filepath.SkipDir
		}

		// Skip common ignored directories
		if info.IsDir() {
			name := info.Name()
			switch name {
			case ".git":
				// Check if this is a nested repo (not the parent's .git)
				if filepath.Dir(path) != parentGitRoot {
					repoPath := filepath.Dir(path)
					if !submodules[repoPath] {
						repos = append(repos, repoPath)
					}
				}
				return filepath.SkipDir
			case "node_modules", "vendor", ".next", "__pycache__", "archive", ".cache", "tmp", "build", "dist":
				return filepath.SkipDir
			}
		}

		return nil
	})

	return repos
}

// getNestedRepoFiles gets files from a nested repo, with paths relative to targetDir
func getNestedRepoFiles(repoPath, targetDir string) ([]FileStatus, error) {
	var files []FileStatus

	// Get uncommitted files
	cmd := gitCmd("status", "--porcelain", "-uall")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			continue
		}

		gitCode := line[:2]
		path := line[3:]

		// Skip temp/binary files
		if shouldSkipFile(path) {
			continue
		}

		fullPath := filepath.Join(repoPath, path)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		// Path relative to target directory
		displayPath, _ := filepath.Rel(targetDir, fullPath)

		files = append(files, FileStatus{
			Status:   "uncommitted",
			GitCode:  gitCode,
			Path:     displayPath,
			FullPath: path,
			GitRoot:  repoPath,
			IsFile:   true,
			ModTime:  info.ModTime(),
		})
	}

	// Also get recently committed files from nested repo
	cmd = gitCmd("log", "--name-only", "--pretty=format:%h|%ar", "-n", "5")
	cmd.Dir = repoPath
	output, err = cmd.Output()
	if err != nil {
		return files, nil // Return what we have
	}

	var currentCommit, currentTime string
	seenInRepo := make(map[string]bool)
	for _, f := range files {
		seenInRepo[f.FullPath] = true
	}

	scanner = bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		if strings.Contains(line, "|") {
			parts := strings.SplitN(line, "|", 2)
			currentCommit = parts[0]
			currentTime = parts[1]
			continue
		}

		if currentCommit == "" || seenInRepo[line] {
			continue
		}

		if shouldSkipFile(line) {
			continue
		}

		fullPath := filepath.Join(repoPath, line)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		displayPath, _ := filepath.Rel(targetDir, fullPath)

		files = append(files, FileStatus{
			Status:   "committed",
			Path:     displayPath,
			FullPath: line,
			GitRoot:  repoPath,
			Commit:   currentCommit,
			TimeAgo:  currentTime,
			IsFile:   true,
			ModTime:  info.ModTime(),
		})
		seenInRepo[line] = true
	}

	return files, nil
}

func getUncommitted(gitRoot, prefix, fileGitRoot string) ([]FileStatus, error) {
	cmd := gitCmd("status", "--porcelain", "-uall")
	cmd.Dir = gitRoot
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

		gitCode := line[:2]
		path := line[3:]

		// Filter by prefix (subdirectory)
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			continue
		}

		// Skip temp/binary files
		if shouldSkipFile(path) {
			continue
		}

		// Check if it's a file
		fullPath := filepath.Join(gitRoot, path)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		// Store path relative to target directory for display
		displayPath := path
		if prefix != "" {
			displayPath = strings.TrimPrefix(path, prefix)
		}

		files = append(files, FileStatus{
			Status:   "uncommitted",
			GitCode:  gitCode,
			Path:     displayPath,
			FullPath: path,
			GitRoot:  fileGitRoot,
			IsFile:   true,
			ModTime:  info.ModTime(),
		})
	}

	return files, nil
}

func getRecentlyCommitted(gitRoot, prefix, fileGitRoot string) ([]FileStatus, error) {
	// Get last 5 commits with files
	cmd := gitCmd("log", "--name-only", "--pretty=format:%h|%ar", "-n", "5")
	cmd.Dir = gitRoot
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

		// Filter by prefix (subdirectory)
		if prefix != "" && !strings.HasPrefix(line, prefix) {
			continue
		}

		// Skip temp/binary files
		if shouldSkipFile(line) {
			continue
		}

		// Check if file still exists and is a file
		fullPath := filepath.Join(gitRoot, line)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}

		// Store path relative to target directory for display
		displayPath := line
		if prefix != "" {
			displayPath = strings.TrimPrefix(line, prefix)
		}

		files = append(files, FileStatus{
			Status:   "committed",
			Path:     displayPath,
			FullPath: line,
			GitRoot:  fileGitRoot,
			Commit:   currentCommit,
			TimeAgo:  currentTime,
			IsFile:   true,
			ModTime:  info.ModTime(),
		})
	}

	return files, nil
}
