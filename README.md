# rethread

Selective replay of Claude Code conversations into new sessions.

## The Problem

When a Claude Code chat stays open, every follow-up benefits from the full conversation history — the reasoning, the dead ends, the texture of how decisions were made. When you manually write context to a file and load it in a new session, you lose all of that. Same information, worse results.

This happens for two reasons:

1. **Lossy compression.** Writing context to a file keeps facts and decisions but loses the reasoning embedded in assistant replies, the sequence of ruled-out alternatives, and how settled each decision was.

2. **Lost in the Middle.** Models attend most to the beginning and end of their context window. A context file loaded at the start gets pushed into the middle as conversation grows — exactly where attention is lowest. ([Liu et al., 2023](https://arxiv.org/abs/2307.03172))

**rethread** solves this by replaying selected conversation turns — with their original structure intact — into a new session. No summarization, no flattening. The model sees structured `[user]...[assistant]...` turns, not a document.

## Install

```bash
go install github.com/ChaitanyaPinapaka/rethread@latest
```

Or build from source:

```bash
git clone https://github.com/ChaitanyaPinapaka/rethread.git
cd rethread
go build -o rethread .
```

Single binary. No runtime. Sub-millisecond startup.

## Commands

### `rethread list`

Discover and list available Claude Code sessions.

```bash
rethread list                        # list all sessions (most recent first)
rethread list -p myapp               # filter by project path
rethread list -n 5                   # show only 5 sessions
rethread list -v                     # verbose: show preview and file path
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--project` | `-p` | | Filter by project path (substring match) |
| `--limit` | `-n` | `15` | Max sessions to show |
| `--verbose` | `-v` | `false` | Show first message preview and file path |

**Output:**

```
  Sessions (4 of 4):

  d0873f7a  Mar 07 02:55   83 turns  Work/rethread
  46a4f397  Mar 02 04:36    3 turns  Work/personal-blog
  52621718  Mar 02 04:34   14 turns  Work/personal-blog
  1c1e0625  Mar 02 04:10  314 turns  Work/personal-blog
```

Session IDs can be used as prefixes — `d087` is enough if unambiguous.

---

### `rethread inspect`

Analyze a session's turns and get a recommended replay strategy.

```bash
rethread inspect d0873f7a
rethread inspect d087                # prefix match works
```

**Output:**

```
  Session: d0873f7a-...
  Project: /Work/rethread

  Total turns:      83
    User:           40
    Assistant:      43
    Sidechain:       0
    Low-signal:      2
  Token estimate:   ~45000
  Fits in context:  yes

  Recommended: full replay (fits in context window)
    rethread fork d0873f7a
```

---

### `rethread fork`

Fork a session into a new Claude Code session. The main command.

```bash
# Basic usage
rethread fork abc123                 # full replay of session
rethread fork --last                 # most recent session
rethread fork                        # same as --last

# Selection strategies
rethread fork abc123 --turns 20      # last 20 turns only
rethread fork abc123 --prune         # drop acknowledgments and filler
rethread fork abc123 --prune --prune-threshold 50  # custom threshold

# Preview before launching
rethread fork abc123 --dry-run

# Write to file instead of launching
rethread fork abc123 -o context.md -f markdown
rethread fork abc123 -o session.jsonl -f jsonl
rethread fork abc123 -o cleaned.jsonl -f clean

# Start with a prompt
rethread fork abc123 -p "Continue from where we left off on the auth module"
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--last` | | `false` | Use the most recent session |
| `--turns` | `-t` | `0` (all) | Replay only the last N turns |
| `--prune` | | `false` | Auto-prune low-signal turns |
| `--prune-threshold` | | `30` | Token threshold for pruning |
| `--dry-run` | | `false` | Preview what would be replayed |
| `--prompt` | `-p` | | Initial prompt for the new session |
| `--output` | `-o` | | Write to file instead of launching |
| `--format` | `-f` | `turns` | Output format: `turns`, `jsonl`, `clean`, `markdown` |

**Injection modes** (chosen automatically):

- **Native fork** — Full replay within the same project. Uses `claude --resume`.
- **Context injection** — For pruned/selected turns. Formats as structured XML and pipes to a new `claude` session.

---

### `rethread export`

Export a session to a file or stdout.

```bash
# Export as markdown (default)
rethread export abc123
rethread export abc123 -o conversation.md

# Export as JSONL (native Claude Code format)
rethread export abc123 -f jsonl > full.jsonl

# Export as clean JSONL (text + thinking + tool_use, no tool_result noise)
rethread export abc123 -f clean -o cleaned.jsonl

# Export with selection
rethread export abc123 -f clean -t 20           # last 20 turns
rethread export abc123 -f markdown --prune       # pruned
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--format` | `-f` | `markdown` | Output format: `turns`, `jsonl`, `clean`, `markdown` |
| `--output` | `-o` | stdout | Output file path |
| `--turns` | `-t` | `0` (all) | Export only the last N turns |
| `--prune` | | `false` | Prune low-signal turns before export |

## Output Formats

| Format | Description | Use case |
|--------|-------------|----------|
| `turns` | Structured XML with `<turn role="...">` tags | Injection into Claude Code |
| `jsonl` | Full native Claude Code format with all fields | Tooling, backup, replay |
| `clean` | Minimal JSONL — keeps `text`, `thinking`, `tool_use`; drops `tool_result`, signatures, metadata | Cross-model use (Gemini, GPT), analysis |
| `markdown` | Human-readable with role headers and timestamps | Documentation, review, sharing |

### `clean` format example

```json
{"role":"user","content":[{"type":"text","text":"Review this package and tell me what it does"}]}
{"role":"assistant","content":[{"type":"thinking","thinking":"Let me explore the project structure."}]}
{"role":"assistant","content":[{"type":"tool_use","name":"Glob","input":{"pattern":"**/*.go"}}]}
{"role":"assistant","content":[{"type":"text","text":"This is a CLI tool that..."}]}
```

Strips out `tool_result` (raw file dumps that inflate size), cryptographic `signature` blobs from thinking blocks, and all metadata (`uuid`, `parentUuid`, `timestamp`, `isSidechain`).

## How It Works

### Reading

rethread reads Claude Code's local conversation history from `~/.claude/projects/`. Each session is a JSONL file where every line is a conversation event with role, content, timestamp, and parent/child relationships. Files are streamed line-by-line with `bufio.Scanner` — no need to load entire sessions into memory.

### Selecting

Four selection strategies:

| Strategy | Flag | What it does |
|----------|------|-------------|
| Full | *(default)* | Every turn, verbatim |
| Last N | `--turns N` | Most recent N turns |
| Prune | `--prune` | Drop acknowledgments and filler |
| Range | *(programmatic)* | Explicit turn range |

**Pruning rules:** A turn is dropped only if it's under a token threshold AND matches acknowledgment patterns (e.g., "ok", "sounds good", "got it"). Turns containing code, URLs, or file paths are never pruned. First 2 and last 2 turns are always preserved.

### Token Budget

When selected turns exceed the context window (~150k tokens with margin), rethread uses a **skeleton + recent** strategy:

1. Keep the first 2 turns (problem statement / initial direction)
2. Fill remaining budget from the end (most recent turns)

This maps to the "Lost in the Middle" research: beginning sets the frame, end has the live context.

## Design Principles

1. **Never summarize.** We include a turn verbatim or drop it entirely. No rewriting.
2. **Structure over prose.** The model processes `[user]...[assistant]...` turns differently from a document. Preserve the structure.
3. **Recent > old.** When budget is tight, keep the end of the conversation where attention is highest.
4. **Pruning ≠ compression.** Removing "ok sounds good" is not the same as summarizing a design discussion. One is noise removal, the other is lossy compression.

## How Claude Code Stores Conversations

```
~/.claude/
  projects/
    -Users-alice-myproject/       # encoded project path
      abc123-def456.jsonl         # one file per session
```

Each JSONL line:
```json
{
  "type": "user",
  "message": { "role": "user", "content": "..." },
  "timestamp": "2025-06-02T18:46:59.937Z",
  "uuid": "...",
  "parentUuid": "...",
  "isSidechain": false,
  "sessionId": "abc123-def456"
}
```

## License

MIT
