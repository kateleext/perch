package highlight

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Theme is a muted, zen color palette
var Theme = styles.Get("dracula") // TODO: create custom zen theme

// Highlight applies syntax highlighting to source code
func Highlight(filename, source string) (string, error) {
	lexer := lexers.Match(filename)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// TODO: tokenize and format with ANSI colors
	_ = Theme
	return source, nil
}
