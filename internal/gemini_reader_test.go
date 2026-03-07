package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadGeminiTurns(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.json")

	session := GeminiSession{
		SessionID:   "g1",
		ProjectHash: "abc123def456",
		StartTime:   "2024-06-01T10:00:00Z",
		LastUpdated: "2024-06-01T10:05:00Z",
		Messages: []GeminiMessage{
			{
				ID:        "m1",
				Timestamp: "2024-06-01T10:00:00Z",
				Type:      "user",
				Content:   "explain Go interfaces",
			},
			{
				ID:        "m2",
				Timestamp: "2024-06-01T10:01:00Z",
				Type:      "gemini",
				Content:   "Go interfaces are...",
				Thoughts: []GeminiThought{
					{Subject: "analysis", Description: "thinking about interfaces", Timestamp: "2024-06-01T10:00:30Z"},
				},
				Tokens: &GeminiTokens{Input: 0, Output: 50, Total: 70},
			},
		},
	}

	data, _ := json.Marshal(session)
	os.WriteFile(filePath, data, 0644)

	turns, err := ReadGeminiTurns(filePath, false)
	if err != nil {
		t.Fatalf("ReadGeminiTurns failed: %v", err)
	}

	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}

	// First turn: user
	if turns[0].Role != "user" {
		t.Errorf("turn 0 role: expected 'user', got %q", turns[0].Role)
	}
	if turns[0].ContentText != "explain Go interfaces" {
		t.Errorf("turn 0 text: got %q", turns[0].ContentText)
	}

	// Second turn: gemini -> assistant
	if turns[1].Role != "assistant" {
		t.Errorf("turn 1 role: expected 'assistant', got %q", turns[1].Role)
	}
	if turns[1].TokenEstimate != 50 {
		t.Errorf("turn 1 tokens: expected 50 (from Tokens.Output), got %d", turns[1].TokenEstimate)
	}

	// Content should include thinking blocks
	blocks, ok := turns[1].Content.([]interface{})
	if !ok {
		t.Fatal("assistant content should be []interface{}")
	}
	if len(blocks) < 2 {
		t.Errorf("expected at least 2 content blocks (thinking + text), got %d", len(blocks))
	}
}

func TestReadGeminiTurns_EmptyMessages(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.json")

	session := GeminiSession{
		SessionID: "g2",
		Messages:  []GeminiMessage{},
	}
	data, _ := json.Marshal(session)
	os.WriteFile(filePath, data, 0644)

	turns, err := ReadGeminiTurns(filePath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(turns) != 0 {
		t.Errorf("expected 0 turns, got %d", len(turns))
	}
}

func TestReadGeminiTurns_NoTokens(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notokens.json")

	session := GeminiSession{
		SessionID: "g3",
		Messages: []GeminiMessage{
			{ID: "m1", Type: "user", Content: "short message"},
		},
	}
	data, _ := json.Marshal(session)
	os.WriteFile(filePath, data, 0644)

	turns, err := ReadGeminiTurns(filePath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turns[0].TokenEstimate != estimateTokens("short message") {
		t.Errorf("expected fallback token estimate, got %d", turns[0].TokenEstimate)
	}
}

func TestReadGeminiTurns_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "invalid.json")
	os.WriteFile(filePath, []byte("not json"), 0644)

	_, err := ReadGeminiTurns(filePath, false)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReadGeminiTurns_FileNotFound(t *testing.T) {
	_, err := ReadGeminiTurns("/nonexistent/file.json", false)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestBuildGeminiSessionMeta(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.json")

	session := GeminiSession{
		SessionID:   "gemini-sess-1",
		ProjectHash: "abcdef123456789",
		StartTime:   "2024-06-01T10:00:00Z",
		LastUpdated: "2024-06-01T11:00:00Z",
		Messages: []GeminiMessage{
			{ID: "m1", Type: "user", Content: "first question here"},
			{ID: "m2", Type: "gemini", Content: "answer here"},
		},
	}

	data, _ := json.Marshal(session)
	os.WriteFile(filePath, data, 0644)

	meta, err := buildGeminiSessionMeta("abcdef123456789", filePath, "")
	if err != nil {
		t.Fatalf("buildGeminiSessionMeta failed: %v", err)
	}

	if meta.ID != "gemini-sess-1" {
		t.Errorf("ID: expected 'gemini-sess-1', got %q", meta.ID)
	}
	if meta.TurnCount != 2 {
		t.Errorf("TurnCount: expected 2, got %d", meta.TurnCount)
	}
	if meta.Preview != "first question here" {
		t.Errorf("Preview: expected 'first question here', got %q", meta.Preview)
	}
}

func TestBuildGeminiSessionMeta_FilteredOut(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.json")

	session := GeminiSession{
		SessionID:   "g1",
		ProjectHash: "abc123def456",
		Messages:    []GeminiMessage{{ID: "m1", Type: "user", Content: "hello"}},
	}

	data, _ := json.Marshal(session)
	os.WriteFile(filePath, data, 0644)

	_, err := buildGeminiSessionMeta("abc123def456", filePath, "nonexistent-filter")
	if err == nil {
		t.Error("expected error when filter doesn't match")
	}
}

func TestBuildGeminiSessionMeta_EmptySession(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.json")

	session := GeminiSession{SessionID: "g1", ProjectHash: "abc123", Messages: []GeminiMessage{}}
	data, _ := json.Marshal(session)
	os.WriteFile(filePath, data, 0644)

	_, err := buildGeminiSessionMeta("abc123", filePath, "")
	if err == nil {
		t.Error("expected error for empty session")
	}
}
