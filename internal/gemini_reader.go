package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var geminiDir string

func init() {
	geminiDir = filepath.Join(HomeDir(), ".gemini", "tmp")
}

// GeminiSession is the top-level JSON structure of a Gemini CLI session file.
type GeminiSession struct {
	SessionID   string          `json:"sessionId"`
	ProjectHash string          `json:"projectHash"`
	StartTime   string          `json:"startTime"`
	LastUpdated string          `json:"lastUpdated"`
	Messages    []GeminiMessage `json:"messages"`
}

// GeminiMessage is a single message in a Gemini CLI session.
type GeminiMessage struct {
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"` // "user" or "gemini"
	Content   string          `json:"content"`
	Thoughts  []GeminiThought `json:"thoughts,omitempty"`
	Tokens    *GeminiTokens   `json:"tokens,omitempty"`
	Model     string          `json:"model,omitempty"`
}

// GeminiThought is a thinking step from Gemini.
type GeminiThought struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Timestamp   string `json:"timestamp"`
}

// GeminiTokens holds token usage for a Gemini message.
type GeminiTokens struct {
	Input    int `json:"input"`
	Output   int `json:"output"`
	Cached   int `json:"cached"`
	Thoughts int `json:"thoughts"`
	Tool     int `json:"tool"`
	Total    int `json:"total"`
}

// ListGeminiSessions returns all available Gemini CLI sessions.
func ListGeminiSessions(projectFilter string) ([]SessionMeta, error) {
	if _, err := os.Stat(geminiDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("Gemini CLI sessions directory not found at %s\nMake sure Gemini CLI is installed and has been used at least once", geminiDir)
	}

	projectDirs, err := os.ReadDir(geminiDir)
	if err != nil {
		return nil, fmt.Errorf("reading Gemini sessions directory: %w", err)
	}

	var sessions []SessionMeta

	for _, projEntry := range projectDirs {
		if !projEntry.IsDir() {
			continue
		}

		chatsDir := filepath.Join(geminiDir, projEntry.Name(), "chats")
		if _, err := os.Stat(chatsDir); os.IsNotExist(err) {
			continue
		}

		files, err := os.ReadDir(chatsDir)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}

			filePath := filepath.Join(chatsDir, f.Name())
			meta, err := buildGeminiSessionMeta(projEntry.Name(), filePath, projectFilter)
			if err != nil || meta.TurnCount == 0 {
				continue
			}
			sessions = append(sessions, meta)
		}
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastTimestamp.After(sessions[j].LastTimestamp)
	})

	return sessions, nil
}

// ReadGeminiTurns parses a Gemini CLI session JSON file into Turn structs.
func ReadGeminiTurns(filePath string, _ bool) ([]Turn, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading Gemini session file: %w", err)
	}

	var session GeminiSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parsing Gemini session file: %w", err)
	}

	var turns []Turn
	for idx, msg := range session.Messages {
		role := msg.Type
		if role == "gemini" {
			role = "assistant"
		}

		// Build content blocks matching the internal format
		var contentBlocks []interface{}

		// Add thinking blocks from thoughts
		for _, thought := range msg.Thoughts {
			contentBlocks = append(contentBlocks, map[string]interface{}{
				"type":     "thinking",
				"thinking": fmt.Sprintf("[%s] %s", thought.Subject, thought.Description),
			})
		}

		// Add text content
		if msg.Content != "" {
			contentBlocks = append(contentBlocks, map[string]interface{}{
				"type": "text",
				"text": msg.Content,
			})
		}

		var content interface{}
		if len(contentBlocks) > 0 {
			content = contentBlocks
		} else {
			content = msg.Content
		}

		tokenEst := estimateTokens(msg.Content)
		if msg.Tokens != nil {
			tokenEst = msg.Tokens.Output
			if msg.Type == "user" {
				tokenEst = msg.Tokens.Input
			}
			if tokenEst == 0 {
				tokenEst = estimateTokens(msg.Content)
			}
		}

		turns = append(turns, Turn{
			Index:         idx,
			Role:          role,
			Content:       content,
			ContentText:   msg.Content,
			Timestamp:     msg.Timestamp,
			UUID:          msg.ID,
			IsSidechain:   false,
			TokenEstimate: tokenEst,
		})
	}

	return turns, nil
}

func buildGeminiSessionMeta(projectHash, filePath, projectFilter string) (SessionMeta, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SessionMeta{}, err
	}

	var session GeminiSession
	if err := json.Unmarshal(data, &session); err != nil {
		return SessionMeta{}, err
	}

	if len(session.Messages) == 0 {
		return SessionMeta{}, fmt.Errorf("empty session")
	}

	// Use project hash as project path (Gemini doesn't store the original path)
	projectPath := "gemini:" + projectHash[:12]

	if projectFilter != "" && !strings.Contains(projectPath, projectFilter) && !strings.Contains(projectHash, projectFilter) {
		return SessionMeta{}, fmt.Errorf("filtered out")
	}

	// Find first user message for preview
	preview := ""
	for _, msg := range session.Messages {
		if msg.Type == "user" {
			preview = TruncatePreview(msg.Content, 100)
			break
		}
	}

	startTime, _ := time.Parse(time.RFC3339Nano, session.StartTime)
	lastUpdated, _ := time.Parse(time.RFC3339Nano, session.LastUpdated)

	return SessionMeta{
		ID:             session.SessionID,
		ProjectPath:    projectPath,
		EncodedPath:    projectHash,
		FilePath:       filePath,
		TurnCount:      len(session.Messages),
		FirstTimestamp:  startTime,
		LastTimestamp:   lastUpdated,
		Preview:        preview,
	}, nil
}
