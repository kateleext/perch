package ui

import (
	"fmt"
	"testing"
	
	"github.com/kateleext/perch/internal/git"
)

func TestFileStatus(t *testing.T) {
	dir := "/Users/kate/Projects/takuma-os/labs/fun/perch"
	
	files, _ := git.GetStatus(dir)
	for _, f := range files {
		if f.Path == "README.md" {
			fmt.Printf("README.md found:\n")
			fmt.Printf("  Status: %s\n", f.Status)
			fmt.Printf("  GitCode: %s\n", f.GitCode)
			fmt.Printf("  FullPath: %s\n", f.FullPath)
		}
	}
}
