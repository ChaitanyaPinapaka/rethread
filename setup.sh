#!/bin/bash
# rethread — setup
#
# Usage: ./setup.sh

set -e

REPO_NAME="rethread"

echo "→ Resolving dependencies..."
go mod tidy

echo "→ Building..."
go build -o rethread .

echo "→ Running smoke test..."
./rethread --version

echo "→ Initializing git repo..."
git init
git add .
git commit -m "initial commit: rethread v0.1.0

Selective replay of Claude Code conversations into new sessions.
Go CLI — single binary, sub-ms startup, streams JSONL with bufio.Scanner.

- internal/reader.go:    discovers sessions, parses JSONL from ~/.claude/projects/
- internal/selector.go:  full, last-n, prune, range strategies + token budget
- internal/formatter.go: structured turns, JSONL, markdown output
- internal/injector.go:  native fork or context injection
- cmd/:                  cobra CLI (list, inspect, fork, export)"

echo "→ Creating private repo on GitHub..."
gh repo create "$REPO_NAME" --private --source=. --remote=origin --push

echo ""
echo "✓ Done. Repo: https://github.com/ChaitanyaPinapaka/$REPO_NAME"
echo ""
echo "Install globally:"
echo "  go install github.com/ChaitanyaPinapaka/rethread@latest"
echo ""
echo "Or use locally:"
echo "  ./rethread list"
echo "  ./rethread inspect <session-id>"
echo "  ./rethread fork --last"
