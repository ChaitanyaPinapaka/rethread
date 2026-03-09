# rethread

Selective export of AI CLI conversations from **Claude Code** and **Gemini CLI**.

## The Problem

AI CLI conversation histories are valuable, but they are often trapped in formats that are difficult to reuse. `rethread` allows you to list, inspect, and export these sessions into clean, usable JSONL.

This enables you to:
-   Document a conversation.
-   Share it with others.
-   Use it as context for another model or a different AI tool.
-   Analyze the conversation's structure and content.

## Supported Sources

| Tool          | Storage Location                             | Format                     | Auto-detected |
| ------------- | -------------------------------------------- | -------------------------- | ------------- |
| **Claude Code** | `~/.claude/projects/<encoded-path>/*.jsonl` | JSONL (one event per line) | Yes           |
| **Gemini CLI**  | `~/.gemini/tmp/<project-hash>/chats/*.json`  | JSON (session object)      | Yes           |

By default, rethread auto-detects and lists sessions from all installed sources. Use `--source claude` or `--source gemini` to filter.

## Install

### Quick install (Linux / macOS)

```bash
curl -sSL https://raw.githubusercontent.com/ChaitanyaPinapaka/rethread/main/install.sh | bash
```

Downloads the latest pre-built binary and installs to `/usr/local/bin`.

### Download from GitHub Releases

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/ChaitanyaPinapaka/rethread/releases) page.

### With Go

```bash
go install github.com/ChaitanyaPinapaka/rethread@latest
```

This places the binary in `$GOPATH/bin` (or `$HOME/go/bin` by default). Make sure it's in your `PATH`:

```bash
# Linux / macOS (add to ~/.bashrc or ~/.zshrc)
export PATH="$PATH:$(go env GOPATH)/bin"

# Windows (PowerShell — run once as admin)
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";" + (go env GOPATH) + "\bin", "User")
```

### Build from source

```bash
git clone https://github.com/ChaitanyaPinapaka/rethread.git
cd rethread
go build -o rethread .
```

Then move the binary to a directory in your `PATH`, or run it directly with `./rethread`.

## Global Flags

| Flag        | Default | Description                                       |
| ----------- | ------- | ------------------------------------------------- |
| `--source`  | `auto`  | Session source: `claude`, `gemini`, `auto` (both) |
| `--version` |         | Print version and exit                            |

```bash
rethread list --source gemini       # only Gemini CLI sessions
rethread list --source claude       # only Claude Code sessions
rethread list                       # both (auto-detect)
```

## Commands

### `rethread list`

Discover and list available sessions from all sources.

```bash
rethread list                        # list all sessions (most recent first)
rethread list -p myapp               # filter by project path
rethread list -n 5                   # show only 5 sessions
rethread list -v                     # verbose: show preview and file path
```

| Flag        | Short | Default | Description                                |
| ----------- | ----- | ------- | ------------------------------------------ |
| `--project` | `-p`  |         | Filter by project path (substring match)   |
| `--limit`   | `-n`  | `15`    | Max sessions to show                       |
| `--verbose` | `-v`  | `false` | Show first message preview and file path |

**Output:**

```
  Sessions (5 of 5):

  a1b2c3d4  Mar 05 14:22   42 turns  claude   Work/my-app
  e5f6a7b8  Mar 03 09:15   18 turns  claude   Projects/api-server
  c9d0e1f2  Feb 28 11:30    7 turns  gemini   gemini:8f4a2b1c9d3e
  34ab56cd  Feb 25 16:45   12 turns  gemini   gemini:5e7f1a3b8c2d
  78ef90ab  Feb 20 08:10    3 turns  claude   Projects/docs-site
```

---

### `rethread inspect`

Analyze a session's turns and get a recommended export strategy.

```bash
rethread inspect a1b2c3d4            # Inspect a Claude Code session
rethread inspect c9d0e1f2            # Inspect a Gemini CLI session
```

**Output:**

```
  Session: a1b2c3d4-5678-9abc-def0-1234567890ab
  Project: Work/my-app

  Total turns:      42
    User:           20
    Assistant:      22
    Sidechain:      0
    Low-signal:     3
  Token estimate:   ~24000
  Fits in context:  yes

  Recommended export strategy:
  - Full export (entire conversation)
    rethread export a1b2c3d4 -f clean -o a1b2c3d4-full.jsonl
```

---

### `rethread export`

Export a session to a file or stdout, optionally pruning or selecting turns.

```bash
# Export as JSONL (default)
rethread export abc123 -o conversation.jsonl

# Export as "clean" JSONL (ideal for cross-model use)
rethread export abc123 -f clean -o cleaned.jsonl

# Export with selection
rethread export abc123 -f clean --turns 20    # last 20 turns
rethread export abc123 -f clean --range 10-20 # inclusive turn range
rethread export abc123 -f clean --prune       # pruned
```

| Flag       | Short | Default | Description                                                  |
| ---------- | ----- | ------- | ------------------------------------------------------------ |
| `--format` | `-f`  | `jsonl` | Output format: `jsonl`, `clean`                              |
| `--output` | `-o`  | stdout  | Output file path                                             |
| `--turns`  | `-t`  | `0` (all) | Export only the last N turns                               |
| `--range`  |       |         | Export an inclusive turn range like `10-20`                  |
| `--prune`  |       | `false` | Prune low-signal (acknowledgment) turns before export        |

---

### `rethread validate`

Measure how much the `clean` format reduces export size while preserving key conversation content.

```bash
rethread validate a1b2c3d4
```

The report includes:
- raw vs compact byte size
- per-block savings breakdown
- dropped-turn counts
- quality checks for preserved user text, assistant text, file paths, turn order, and non-empty output

## Output Formats

| Format     | Description                                                                            | Use case                                |
| ---------- | -------------------------------------------------------------------------------------- | --------------------------------------- |
| `jsonl`    | Normalized event JSONL with full turn fields preserved across supported sources.        | Tooling, backup, replay/import          |
| `clean`    | Minimal JSONL — keeps `text`, `thinking`, `tool_use`; drops `tool_result`, signatures. | Cross-model use (Gemini, GPT), analysis |

### `clean` format example

```json
{"role":"user","content":[{"type":"text","text":"Review this package and tell me what it does"}]}
{"role":"assistant","content":[{"type":"thinking","thinking":"Let me explore the project structure."}]}
{"role":"assistant","content":[{"type":"tool_use","name":"Glob","input":{"pattern":"**/*.go"}}]}
{"role":"assistant","content":[{"type":"text","text":"This is a CLI tool that..."}]}
```
Both Claude and Gemini sessions are normalized to this same unified format.

## How It Works

### Reading
`rethread` reads local conversation history and normalizes it into a common `Turn` structure, so selection and export work identically regardless of the source.

### Selecting
Four selection strategies are available for export:

| Strategy   | Flag        | What it does                                                                       |
| ---------- | ----------- | ---------------------------------------------------------------------------------- |
| Full       | *(default)* | Every turn, verbatim.                                                              |
| Last N     | `--turns N` | The most recent N turns.                                                           |
| Range      | `--range A-B` | Export turns from index `A` through `B` (inclusive).                            |
| Prune      | `--prune`   | Drops simple acknowledgment turns (e.g., "ok", "sounds good", "got it"). |

When a selection exceeds the context window of most models (~150k tokens), the `inspect` command will recommend a `last N` strategy.

## Disclaimer

This is an independent tool. Not affiliated with or endorsed by Anthropic or Google.

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
