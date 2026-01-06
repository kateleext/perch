# PERCH

**Minimal file viewer that stays perched on your agents' progress.**

Like many, I recently moved to the terminal as my primary surface. The only thing I missed from IDEs was the file tree - not to edit,
but to stay grounded in what the agent was doing. Even without reading every line, seeing which files changed created a shared vocabulary between me and the agents.

But agent TUIs are already information-dense. I needed something minimal. Read-only. Single-purpose.

Perch shows the most recent changes in any git directory. Whatever your agent just touched appears at the top. File preview with syntax highlighting shows diffs inline, read-only. That's it.

## Install

**Direct (Recommended):**
```
# macOS arm64 (Apple Silicon)
curl -L https://github.com/kateleext/perch/releases/download/v0.0.1/perch_darwin_arm64.tar.gz | tar xz -C /usr/local/bin

# macOS x86_64 (Intel)
curl -L https://github.com/kateleext/perch/releases/download/v0.0.1/perch_darwin_amd64.tar.gz | tar xz -C /usr/local/bin

# Linux arm64
curl -L https://github.com/kateleext/perch/releases/download/v0.0.1/perch_linux_arm64.tar.gz | tar xz -C /usr/local/bin

# Linux x86_64
curl -L https://github.com/kateleext/perch/releases/download/v0.0.1/perch_linux_amd64.tar.gz | tar xz -C /usr/local/bin
```

**Go:**
```
go install github.com/kateleext/perch/cmd/perch@latest
```

**Homebrew** (experimental):
```
brew tap kateleext/homebrew-tap
brew install perch
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
