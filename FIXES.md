# Perch Rendering Fixes

## Issues Fixed

### 1. Go File Rendering Collapse
**Problem**: When rendering `.go` files, the entire static section would shift upward and collapse, making the UI unusable.

**Root Cause**: ANSI escape sequences from Chroma's syntax highlighter were being split incorrectly. The formatter outputs multi-byte ANSI codes, and splitting on `\n` after formatting could sever codes mid-sequence, corrupting the terminal output.

**Fix**: Modified `highlightCode()` to:
- Split content into lines **before** tokenizing
- Process each line independently through the lexer and formatter
- Trim trailing newlines from each formatted line
- Prevents ANSI sequence fragmentation

### 2. Unknown File Type Corruption (/perch/perch)
**Problem**: Files without recognized extensions (like `/perch/perch`) would render with completely corrupted output, showing garbled characters.

**Root Cause**: When `lexers.Match()` returns `nil`, the code fell back to `lexers.Fallback`, a generic tokenizer that produces malformed ANSI codes for unknown file types.

**Fix**: Modified `highlightCode()` to:
- Return plaintext (no highlighting) when lexer is `nil`
- Avoids the broken Fallback lexer entirely
- Unknown file types now render cleanly without styling

### 3. Procfile.dev Header/Selector Duplication
**Problem**: When viewing `Procfile.dev`, the header and file selector would duplicate themselves, appearing to render simultaneously.

**Root Cause**: Race condition between two refresh mechanisms:
1. `tickCmd()` in `Init()` sending `RefreshMsg` every 2 seconds
2. Separate goroutine in `main()` also sending `RefreshMsg` every 2 seconds
These collided on fast files like `Procfile.dev`, causing render races.

**Fix**: Removed the duplicate polling goroutine in `main.go`:
- Kept the single `tickCmd()` mechanism from Init
- Eliminated the goroutine that was creating collisions
- Clean, single refresh cycle per 2-second interval

## Technical Details

### ANSI Sequence Handling
The new per-line highlighting approach is more robust:
```go
for i, line := range lines {
    iterator, err := lexer.Tokenise(nil, line)
    // Format individual line, trim newline
    highlightedLines[i] = strings.TrimSuffix(buf.String(), "\n")
}
```

### Fallback Behavior
Instead of:
```go
if lexer == nil {
    lexer = lexers.Fallback  // ❌ Broken ANSI codes
}
```

Now:
```go
if lexer == nil {
    return lines  // ✓ Clean plaintext
}
```

### Polling Simplification
Removed:
```go
go func() {
    ticker := time.NewTicker(2 * time.Second)
    for range ticker.C {
        p.Send(ui.RefreshMsg{})
    }
}()
```

The `tickCmd()` in `model.Init()` handles all refresh timing.

## Testing
All three file types should now render correctly:
- **Go files** (`.go`) - No more collapse
- **Unknown extensions** - Plaintext instead of corruption
- **Short files** (`Procfile.dev`) - No duplication

Build: `go build -o perch ./cmd/perch/`
