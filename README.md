# Perch

Passive file monitor for coding sessions. Watch what's changing from above.

> The bird's eye view of your work.

## Install

```bash
go install github.com/takumahq/perch@latest
```

Or build from source:

```bash
make build
```

## Usage

```bash
# Watch current directory
perch

# Watch specific directory
perch /path/to/project

# Vertical layout
perch -v
```

## Proof of Concept

Before building the full TUI, validate the concept works:

```bash
./poc/perch.sh
```
