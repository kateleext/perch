# Perch

A passive file monitor for terminal-based coding sessions. Watch what's changing from above.

## The Problem

When working with AI coding agents (Claude Code, Cursor, etc.), you lose situational awareness. Files change, but you're focused on the conversation. You need to see what's happening to reason about your next prompt.

Existing tools (lazygit, gitui) are full git clients. Too much. You just want to see.

## The Solution

A read-only, auto-refreshing view of recently modified files with syntax-highlighted preview.

```
┌─────────────────────────────────────────┐
│  › M app/models/user.rb                 │
│    M app/views/index.erb                │
│    A lib/new_thing.rb                   │
├─────────────────────────────────────────┤
│  class User < ApplicationRecord         │
│    has_many :posts                      │
│ +  has_many :comments                   │
│    validates :email, presence: true     │
│                                         │
│ -  def name                             │
│ +  def full_name                        │
│      "#{first_name} #{last_name}"       │
│    end                                  │
│  end                                    │
│                                   ↓ 73% │
└─────────────────────────────────────────┘
```

## Core Behaviors

1. **Passive by default** - Opens, shows state, auto-updates. No interaction required.
2. **Most recent wins** - Auto-selects the most recently saved file for preview.
3. **Diff in context** - Shows the full file with changes highlighted, not just the diff.
4. **Scrollable** - Arrow keys to scroll preview, or let it auto-follow changes.
5. **Two layouts** - Horizontal (list | preview) or vertical (list / preview).

## Non-Goals

- No staging, committing, or git operations
- No file editing
- No configuration files
- No plugins

## Aesthetic

Zen. Calm. Muted colors. Minimal chrome. The tool should feel like a quiet companion, not a demanding interface.

## Technical Approach

- **Go + Bubbletea** - Charm.sh ecosystem for zen TUI aesthetics
- **fswatch/fsnotify** - React to file changes, don't poll
- **bat/chroma** - Syntax highlighting
- **git plumbing** - `git status --porcelain`, `git diff`

## MVP Scope

1. Watch current directory for changes
2. Show modified files list (git status)
3. Preview most recent file with syntax highlighting
4. Show diff markers (+/-) in preview
5. Scroll with arrow keys
6. That's it.
