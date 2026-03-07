package internal

import "time"

// RawEvent is a single line from a Claude Code session JSONL file.
type RawEvent struct {
	Type       string  `json:"type"`
	Message    Message `json:"message"`
	Timestamp  string  `json:"timestamp"`
	UUID       string  `json:"uuid"`
	ParentUUID *string `json:"parentUuid"`
	SessionID  string  `json:"sessionId"`
	IsSidechain bool   `json:"isSidechain"`
	UserType   string  `json:"userType,omitempty"`
	Cwd        string  `json:"cwd,omitempty"`
	Version    string  `json:"version,omitempty"`
}

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ContentBlock
}

type ContentBlock struct {
	Type    string      `json:"type"`
	Text    string      `json:"text,omitempty"`
	Name    string      `json:"name,omitempty"`
	Input   interface{} `json:"input,omitempty"`
	Content interface{} `json:"content,omitempty"`
	ID      string      `json:"id,omitempty"`
}

// Turn is a cleaned-up conversation turn ready for selection and replay.
type Turn struct {
	Index         int
	Role          string
	Content       interface{} // preserve original structure
	ContentText   string      // extracted plain text
	Timestamp     string
	UUID          string
	ParentUUID    *string
	IsSidechain   bool
	TokenEstimate int
}

// SessionMeta holds metadata about a discovered session.
type SessionMeta struct {
	ID             string
	ProjectPath    string // decoded original path
	EncodedPath    string // directory name under ~/.claude/projects/
	FilePath       string // full path to .jsonl file
	TurnCount      int
	FirstTimestamp time.Time
	LastTimestamp   time.Time
	Preview        string // first user message, truncated
}

// SelectionStrategy determines which turns to include.
type SelectionStrategy struct {
	Kind          string // "full", "last", "prune", "range"
	N             int    // for "last"
	MinTokens     int    // for "prune"
	From          int    // for "range"
	To            int    // for "range"
}

// ReplayContext is the fully resolved set of turns to inject.
type ReplayContext struct {
	SourceSessionID    string
	SourceProject      string
	Turns              []Turn
	Strategy           SelectionStrategy
	TotalTokenEstimate int
}

// TurnAnalysis is the result of inspecting a session's turns.
type TurnAnalysis struct {
	TotalTurns          int
	UserTurns           int
	AssistantTurns      int
	SidechainTurns      int
	LowSignalTurns      int
	TotalTokenEstimate  int
	FitsInContext        bool
	RecommendedStrategy SelectionStrategy
}

// InjectResult describes what happened when we injected context.
type InjectResult struct {
	Mode           string // "native-fork", "context-injection", "dry-run"
	SessionID      string
	ContextFile    string
	TokenEstimate  int
	TurnCount      int
}
