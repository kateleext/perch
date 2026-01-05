package git

// FileStatus represents a file's git status
type FileStatus struct {
	Status string // M, A, D, ?, etc
	Path   string
	Mtime  int64 // for sorting by most recent
}

// GetStatus returns modified files from git status --porcelain
func GetStatus(dir string) ([]FileStatus, error) {
	// TODO: exec git status --porcelain
	// TODO: parse output into FileStatus slice
	// TODO: sort by mtime (most recent first)
	return nil, nil
}
