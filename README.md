# PERCH

**Minimal file viewer that stays perched on your agents' progress.**

<img width="498" height="340" alt="image" src="https://github.com/user-attachments/assets/83b36a4f-7f35-4ff5-b3c1-60602f20d4f6" />

<hr>
Agent TUIs are already information-dense with diffs flying all over the screen. I found myself yearning for something minimal. Read-only. Single-purpose. Just shows what files are being worked on so I feel just enough control to not feel like leaving the terminal. 

Perch shows the most recent changes in any git directory. Whatever your agent(s) just touched appears at the top. File preview with syntax highlighting shows diffs inline, read-only. 

That's it.

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

**Direct:**
```
# macOS arm64 (Apple Silicon)
curl -L https://github.com/kateleext/perch/releases/download/v0.0.2/perch_darwin_arm64.tar.gz | tar xz -C /usr/local/bin

# macOS x86_64 (Intel)
curl -L https://github.com/kateleext/perch/releases/download/v0.0.2/perch_darwin_amd64.tar.gz | tar xz -C /usr/local/bin

# Linux arm64
curl -L https://github.com/kateleext/perch/releases/download/v0.0.2/perch_linux_arm64.tar.gz | tar xz -C /usr/local/bin

# Linux x86_64
curl -L https://github.com/kateleext/perch/releases/download/v0.0.2/perch_linux_amd64.tar.gz | tar xz -C /usr/local/bin
```

## Usage

```
# Run in current directory
perch

# Watch a specific directory
perch /path/to/repo
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
