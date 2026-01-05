# Drift

Passive file monitor for coding sessions. See what's changing.

## Install

```bash
go install github.com/takumahq/drift@latest
```

Or build from source:

```bash
make build
```

## Usage

```bash
# Watch current directory
drift

# Watch specific directory
drift /path/to/project

# Vertical layout
drift -v
```

## Proof of Concept

Before building the full TUI, validate the concept works:

```bash
./poc/drift.sh
```
