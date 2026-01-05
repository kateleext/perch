# Perch Build Plan

## Phase 0: Proof of Concept ← YOU ARE HERE

Validate the concept with bash before writing Go.

```bash
./poc/perch.sh
```

**Validates:**
- fswatch detects changes
- git status gives us modified files
- bat renders with syntax + diff highlighting
- The UX feels right

**Exit criteria:** Run POC for a coding session. Does it help? Adjust vision if not.

---

## Phase 1: Minimal TUI

Replace bash POC with Go. Same features, proper TUI.

```
internal/
  git/
    status.go     # git status --porcelain parsing
    diff.go       # git diff for a file
  watcher/
    watch.go      # fsnotify wrapper
  ui/
    model.go      # bubbletea model
    view.go       # render file list + preview
  highlight/
    highlight.go  # chroma syntax highlighting

cmd/perch/
  main.go         # entry point
```

**Features:**
- [x] File list from git status
- [ ] Auto-refresh on file change
- [ ] Preview pane with syntax highlighting
- [ ] Diff markers in preview
- [ ] Scroll with arrow keys

---

## Phase 2: Polish

- [ ] Vertical/horizontal layout toggle (v key)
- [ ] Muted color theme (zen palette)
- [ ] Smooth transitions
- [ ] Handle edge cases (no git repo, binary files)

---

## Phase 3: Ship

- [ ] Homebrew formula
- [ ] goreleaser config
- [ ] README with demo gif

---

## What We're NOT Building

- Git operations (staging, commits)
- File editing
- Config files
- Plugins
- Multiple directory watching
