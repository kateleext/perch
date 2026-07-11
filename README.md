# PERCH

A file viewer for your terminal. Point it at any directory, see what changed, preview everything.

<img width="498" height="340" alt="image" src="https://github.com/user-attachments/assets/83b36a4f-7f35-4ff5-b3c1-60602f20d4f6" />

Built for watching coding agents work, but useful anywhere you want a quick read-only view of files sorted by what changed most recently.

## What it does

- Shows files sorted by most recent changes (git-aware or plain filesystem)
- Syntax-highlighted preview with inline diffs
- Previews Office docs (docx, pptx, xlsx), PDFs, images, and zip archives
- Works in git repos and regular directories
- Refreshes every 2 seconds

## Install

**Homebrew:**
```
brew tap kateleext/homebrew-tap
brew install perch
```

**Go:**
```
go install github.com/kateleext/perch/cmd/perch@latest
```

**Direct download:**
```
# macOS arm64 (Apple Silicon)
curl -L https://github.com/kateleext/perch/releases/download/v0.0.5/perch_darwin_arm64.tar.gz | tar xz -C /usr/local/bin

# macOS x86_64 (Intel)
curl -L https://github.com/kateleext/perch/releases/download/v0.0.5/perch_darwin_amd64.tar.gz | tar xz -C /usr/local/bin

# Linux arm64
curl -L https://github.com/kateleext/perch/releases/download/v0.0.5/perch_linux_arm64.tar.gz | tar xz -C /usr/local/bin

# Linux x86_64
curl -L https://github.com/kateleext/perch/releases/download/v0.0.5/perch_linux_amd64.tar.gz | tar xz -C /usr/local/bin
```

## Usage

```
perch              # current directory
perch /some/path   # any directory
```

Run it in a split pane next to your editor or agent.

| Key | Action |
|-----|--------|
| `up/down` | Navigate files |
| `j/k` | Scroll preview |
| `h/l` | Pan wide content left/right |
| `g/G` | Jump to top/bottom |
| `c` | Copy file content |
| `p` | Copy file path |
| `o` | Open file |
| `f` | Reveal in file manager (Finder on macOS) |
| `+/-` | Resize file list |
| `q` | Quit |

## File support

**Full preview:** Any text file with syntax highlighting (100+ languages), Markdown with styled headings/tables/code blocks

**Office docs:** Word (.docx) with heading hierarchy, PowerPoint (.pptx) with slide dividers, Excel (.xlsx) as rainbow-colored tables, OpenDocument formats

**Other:** PDF text extraction, image preview (half-block rendering), zip archive contents

Files it can't preview get a clean placeholder instead of garbled output.
