package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	claudeDir   string
	projectsDir string
)

func init() {
	home := HomeDir()
	claudeDir = filepath.Join(home, ".claude")
	projectsDir = filepath.Join(claudeDir, "projects")
}

// ListSessions returns all available sessions, optionally filtered by project substring.
func ListSessions(projectFilter string) ([]SessionMeta, error) {
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("claude Code projects directory not found at %s\nMake sure Claude Code is installed and has been used at least once", projectsDir)
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("reading projects directory: %w", err)
	}

	var sessions []SessionMeta

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		decoded := decodeProjectPath(entry.Name())
		if projectFilter != "" && !strings.Contains(decoded, projectFilter) {
			continue
		}

		dirPath := filepath.Join(projectsDir, entry.Name())
		files, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}

			filePath := filepath.Join(dirPath, f.Name())
			sessionID := strings.TrimSuffix(f.Name(), ".jsonl")

			meta, err := buildSessionMeta(sessionID, entry.Name(), decoded, filePath)
			if err != nil || meta.TurnCount == 0 {
				continue
			}
			sessions = append(sessions, meta)
		}
	}

	// Sort by most recent first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastTimestamp.After(sessions[j].LastTimestamp)
	})

	return sessions, nil
}

// ReadTurns streams a session JSONL file and returns parsed conversation turns.
// Filters to user/assistant on the main thread by default.
func ReadTurns(filePath string, includeSidechains bool) ([]Turn, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening session file: %w", err)
	}
	defer f.Close()

	var turns []Turn
	idx := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4*1024*1024), 16*1024*1024) // 16MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event RawEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue // skip malformed lines
		}

		// Only conversation turns
		if event.Type != "user" && event.Type != "assistant" {
			continue
		}

		// Skip sidechains unless requested
		if !includeSidechains && event.IsSidechain {
			continue
		}

		text := ExtractText(event.Message.Content)

		turns = append(turns, Turn{
			Index:         idx,
			Role:          event.Message.Role,
			Content:       event.Message.Content,
			ContentText:   text,
			Timestamp:     event.Timestamp,
			UUID:          event.UUID,
			ParentUUID:    event.ParentUUID,
			IsSidechain:   event.IsSidechain,
			TokenEstimate: estimateTokens(text),
		})
		idx++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning session file: %w", err)
	}

	return turns, nil
}

// --- internal helpers ---

func decodeProjectPath(encoded string) string {
	// Unix: "-Users-alice-my-project" → "/Users/alice/my-project"
	//   Leading "-" means the original path started with "/"
	if strings.HasPrefix(encoded, "-") {
		return "/" + strings.ReplaceAll(encoded[1:], "-", "/")
	}

	// Windows: "c--Work-my-project" → "C:\Work\my-project"
	//   Pattern: single letter + "--" = drive letter + ":\"
	if len(encoded) >= 3 && encoded[1] == '-' && encoded[2] == '-' {
		drive := strings.ToUpper(string(encoded[0]))
		rest := encoded[3:]
		if rest == "" {
			return drive + ":\\"
		}
		return drive + ":\\" + strings.ReplaceAll(rest, "-", "\\")
	}

	return encoded
}

func buildSessionMeta(sessionID, encodedPath, decodedPath, filePath string) (SessionMeta, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return SessionMeta{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	var (
		firstEvent    *RawEvent
		lastConvEvent *RawEvent
		turnCount     int
		preview       string
	)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event RawEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		if firstEvent == nil {
			firstEvent = &event
		}

		if (event.Type == "user" || event.Type == "assistant") && event.Timestamp != "" {
			lastConvEvent = &event
		}

		if event.Type == "user" || event.Type == "assistant" {
			turnCount++
		}

		if preview == "" && event.Type == "user" {
			preview = TruncatePreview(ExtractText(event.Message.Content), 100)
		}
	}

	if firstEvent == nil {
		return SessionMeta{}, fmt.Errorf("empty session")
	}

	firstTime, _ := time.Parse(time.RFC3339Nano, firstEvent.Timestamp)
	var lastTime time.Time
	if lastConvEvent != nil {
		lastTime, _ = time.Parse(time.RFC3339Nano, lastConvEvent.Timestamp)
	}

	if lastTime.IsZero() {
		if info, err := os.Stat(filePath); err == nil {
			lastTime = info.ModTime()
		}
	}

	return SessionMeta{
		ID:             sessionID,
		ProjectPath:    decodedPath,
		EncodedPath:    encodedPath,
		FilePath:       filePath,
		TurnCount:      turnCount,
		FirstTimestamp: firstTime,
		LastTimestamp:  lastTime,
		Preview:        preview,
	}, nil
}

func estimateTokens(text string) int {
	// ~4 chars per token for English
	n := len(text) / 4
	if n == 0 && len(text) > 0 {
		return 1
	}
	return n
}
