package ui

// This file is now minimal - the viewport handles content rendering.
// Keeping the snapshot types for potential future use.

// ViewSnapshot captures the immutable state needed to render a frame
type ViewSnapshot struct {
	Files         []FilePreview
	SelectedIdx   int
	ListScroll    int
	ListHeight    int
	Width         int
	Height        int
	SparkleOn     bool
	Dir           string
	Preview       PreviewContent
}

// FilePreview is the subset of FileStatus needed for rendering
type FilePreview struct {
	Path       string
	Status     string
	GitCode    string
	ChangeType string
	IsSelected bool
}
