package git

// DiffLine represents a line in a diff
type DiffLine struct {
	Number  int
	Content string
	Type    string // "add", "remove", "context"
}

// GetFileDiff returns the diff for a specific file
func GetFileDiff(dir, path string) ([]DiffLine, error) {
	// TODO: exec git diff <path>
	// TODO: parse unified diff format
	// TODO: return line-by-line with markers
	return nil, nil
}

// GetFileWithDiff returns full file content with diff markers
func GetFileWithDiff(dir, path string) ([]DiffLine, error) {
	// TODO: read file content
	// TODO: overlay diff markers on full content
	return nil, nil
}
