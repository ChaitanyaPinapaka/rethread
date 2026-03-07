# rethread

Selective export of AI CLI conversations from **Claude Code** and **Gemini CLI**.

## The Problem

AI CLI conversation histories are valuable, but they are often trapped in formats that are difficult to reuse. `rethread` allows you to list, inspect, and export these sessions into clean, usable formats like Markdown or minimal JSONL.

This enables you to:
-   Document a conversation.
-   Share it with teammates.
-   Use it as context for another model or a different AI tool.
-   Analyze the conversation's structure and content.

## Supported Sources

| Tool          | Storage Location                             | Format                     | Auto-detected |
| ------------- | -------------------------------------------- | -------------------------- | ------------- |
| **Claude Code** | `~/.claude/projects/<encoded-path>/*.jsonl` | JSONL (one event per line) | Yes           |
| **Gemini CLI**  | `~/.gemini/tmp/<project-hash>/chats/*.json`  | JSON (session object)      | Yes           |

By default, rethread auto-detects and lists sessions from all installed sources. Use `--source claude` or `--source gemini` to filter.

## Install

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

Or build from source:

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
  Sessions (6 of 6):

  d0873f7a  Mar 07 05:58  268 turns  claude   c--Work-rethread
  46a4f397  Mar 02 04:36    3 turns  claude   c--Work-personal-blog
  1c1e0625  Mar 02 04:10  314 turns  claude   c--Work-personal-blog
  b3cc573d  Feb 17 01:26    8 turns  gemini   gemini:744858f965d4
  620bb439  Jan 12 05:15    6 turns  gemini   gemini:13905c9c7c3a
  daeb08c8  Jan 12 04:21    5 turns  gemini   gemini:13905c9c7c3a
```

---

### `rethread inspect`

Analyze a session's turns and get a recommended export strategy.

```bash
rethread inspect d0873f7a            # Inspect a Claude Code session
rethread inspect daeb08c8            # Inspect a Gemini CLI session
```

**Output:**

```
  Session: d0873f7a-...
  Project: c--Work-rethread

  Total turns:      268
    User:           130
    Assistant:      138
    Sidechain:      0
    Low-signal:     5
  Token estimate:   ~85000
  Fits in context:  yes

  Recommended export strategy:
  - Full export (entire conversation)
    rethread export d0873f7a -f clean -o d0873f7a-full.jsonl
```

---

### `rethread export`

Export a session to a file or stdout, optionally pruning or selecting turns.

```bash
# Export as JSONL (default)
rethread export abc123 -o conversation.jsonl

# Export as "clean" JSONL (ideal for cross-model use)
rethread export abc123 -f clean -o cleaned.jsonl

# Export as markdown
rethread export abc123 -f markdown -o conversation.md

# Export as structured XML turns
rethread export abc123 -f turns -o conversation.xml

# Export with selection
rethread export abc123 -f clean --turns 20    # last 20 turns
rethread export abc123 -f markdown --prune    # pruned
```

| Flag       | Short | Default | Description                                                  |
| ---------- | ----- | ------- | ------------------------------------------------------------ |
| `--format` | `-f`  | `jsonl` | Output format: `jsonl`, `clean`, `markdown`, `turns`         |
| `--output` | `-o`  | stdout  | Output file path                                             |
| `--turns`  | `-t`  | `0` (all) | Export only the last N turns                               |
| `--prune`  |       | `false` | Prune low-signal (acknowledgment) turns before export        |

## Output Formats

| Format     | Description                                                                            | Use case                                |
| ---------- | -------------------------------------------------------------------------------------- | --------------------------------------- |
| `jsonl`    | Full native format with all fields (Claude only).                                      | Tooling, backup, exact replay           |
| `clean`    | Minimal JSONL — keeps `text`, `thinking`, `tool_use`; drops `tool_result`, signatures. | Cross-model use (Gemini, GPT), analysis |
| `markdown` | Human-readable with role headers and timestamps.                                       | Documentation, review, sharing          |
| `turns`    | XML structured format with `<conversation>` and `<turn>` tags.                         | LLM prompts, structured pipelines       |

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
Three selection strategies are available for export:

| Strategy   | Flag        | What it does                                                                       |
| ---------- | ----------- | ---------------------------------------------------------------------------------- |
| Full       | *(default)* | Every turn, verbatim.                                                              |
| Last N     | `--turns N` | The most recent N turns.                                                           |
| Prune      | `--prune`   | Drops simple acknowledgment turns (e.g., "ok", "sounds good", "got it"). |

When a selection exceeds the context window of most models (~150k tokens), the `inspect` command will recommend a `last N` strategy.

## License

MIT
