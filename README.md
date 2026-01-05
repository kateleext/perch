# Perch

Passive file monitor for coding sessions. See what's changing from above.

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
