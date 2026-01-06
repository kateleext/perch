# PERCH

**Minimal file viewer that stays perched on your agents' progress.**

Like many, I recently moved to terminal-based coding agents. The only thing I missed from VS Code was the file tree - not to edit,
but to stay grounded in what the agent was doing. Even without reading every line, seeing which files changed helped me prompt
better. We'd build a shared vocabulary: this file does X, that function handles Y.

But agent TUIs are already information-dense. I needed something minimal. Read-only. Single-purpose.

Perch shows the most recent changes in any git directory. Whatever your agent just touched appears at the top. That's it.

## Install

```
go install github.com/kateleext/perch/cmd/perch@latest
```

## Usage

```
perch [directory]
```

Run it in a split pane. It refreshes every 2 seconds.

| Key | Action |
|-----|--------|
| `↑↓` | Navigate files |
| `j/k` | Scroll preview |
| `g/G` | Top/bottom |
| `q` | Quit |
| `shift` + select | Copy text |

---

v0.1 is a proof of concept for my own workflow. Contributions welcome.
