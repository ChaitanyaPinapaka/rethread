package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- ExtractText ---

func TestExtractText_String(t *testing.T) {
	result := ExtractText("plain text")
	if result != "plain text" {
		t.Errorf("expected 'plain text', got %q", result)
	}
}

func TestExtractText_ContentBlocks(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "first"},
		map[string]interface{}{"type": "tool_use", "name": "read"},
		map[string]interface{}{"type": "text", "text": "second"},
	}

	result := ExtractText(content)
	if result != "first\nsecond" {
		t.Errorf("expected 'first\\nsecond', got %q", result)
	}
}

func TestExtractText_EmptyBlocks(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "tool_use", "name": "read"},
	}

	result := ExtractText(content)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestExtractText_OtherType(t *testing.T) {
	result := ExtractText(42)
	if result != "42" {
		t.Errorf("expected '42', got %q", result)
	}
}

// --- decodeProjectPath ---

func TestDecodeProjectPath_Unix(t *testing.T) {
	result := decodeProjectPath("-Users-alice-my-project")
	if result != "/Users/alice/my/project" {
		t.Errorf("expected '/Users/alice/my/project', got %q", result)
	}
}

func TestDecodeProjectPath_Windows(t *testing.T) {
	result := decodeProjectPath("c--Work-rethread")
	if result != "C:\\Work\\rethread" {
		t.Errorf("expected 'C:\\Work\\rethread', got %q", result)
	}
}

func TestDecodeProjectPath_WindowsDriveOnly(t *testing.T) {
	result := decodeProjectPath("c--")
	if result != "C:\\" {
		t.Errorf("expected 'C:\\', got %q", result)
	}
}

func TestDecodeProjectPath_Plain(t *testing.T) {
	result := decodeProjectPath("someproject")
	if result != "someproject" {
		t.Errorf("expected 'someproject', got %q", result)
	}
}

// --- estimateTokens ---

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"hi", 1},
		{"hello world test", 4},
		{"a", 1},
	}

	for _, tt := range tests {
		got := estimateTokens(tt.text)
		if got != tt.want {
			t.Errorf("estimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

// --- ReadTurns with temp file ---

func TestReadTurns(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	events := []RawEvent{
		{
			Type:      "user",
			Message:   Message{Role: "user", Content: "hello"},
			Timestamp: "2024-01-01T00:00:00Z",
			UUID:      "u1",
			SessionID: "s1",
		},
		{
			Type:      "assistant",
			Message:   Message{Role: "assistant", Content: "hi there"},
			Timestamp: "2024-01-01T00:01:00Z",
			UUID:      "u2",
			SessionID: "s1",
		},
		{
			Type:        "assistant",
			Message:     Message{Role: "assistant", Content: "sidechain msg"},
			Timestamp:   "2024-01-01T00:02:00Z",
			UUID:        "u3",
			SessionID:   "s1",
			IsSidechain: true,
		},
		{
			Type:      "system",
			Message:   Message{Role: "system", Content: "init"},
			Timestamp: "2024-01-01T00:00:00Z",
			UUID:      "u0",
			SessionID: "s1",
		},
	}

	var lines []byte
	for _, e := range events {
		data, _ := json.Marshal(e)
		lines = append(lines, data...)
		lines = append(lines, '\n')
	}
	os.WriteFile(filePath, lines, 0644)

	// Without sidechains
	turns, err := ReadTurns(filePath, false)
	if err != nil {
		t.Fatalf("ReadTurns failed: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns (no sidechains, no system), got %d", len(turns))
	}
	if turns[0].Role != "user" || turns[1].Role != "assistant" {
		t.Error("unexpected turn roles")
	}

	// With sidechains
	turns, err = ReadTurns(filePath, true)
	if err != nil {
		t.Fatalf("ReadTurns with sidechains failed: %v", err)
	}
	if len(turns) != 3 {
		t.Fatalf("expected 3 turns with sidechains, got %d", len(turns))
	}
}

func TestReadTurns_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(filePath, []byte(""), 0644)

	turns, err := ReadTurns(filePath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(turns) != 0 {
		t.Errorf("expected 0 turns, got %d", len(turns))
	}
}

func TestReadTurns_MalformedLines(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "malformed.jsonl")
	content := "not json\n{invalid}\n"
	os.WriteFile(filePath, []byte(content), 0644)

	turns, err := ReadTurns(filePath, false)
	if err != nil {
		t.Fatalf("malformed lines should be skipped, got error: %v", err)
	}
	if len(turns) != 0 {
		t.Errorf("expected 0 turns from malformed input, got %d", len(turns))
	}
}

func TestReadTurns_FileNotFound(t *testing.T) {
	_, err := ReadTurns("/nonexistent/file.jsonl", false)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// --- buildSessionMeta ---

func TestBuildSessionMeta(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-session.jsonl")

	events := []RawEvent{
		{
			Type:      "user",
			Message:   Message{Role: "user", Content: "first question about something"},
			Timestamp: "2024-06-01T10:00:00Z",
			UUID:      "u1",
			SessionID: "s1",
		},
		{
			Type:      "assistant",
			Message:   Message{Role: "assistant", Content: "answer"},
			Timestamp: "2024-06-01T10:05:00Z",
			UUID:      "u2",
			SessionID: "s1",
		},
	}

	var lines []byte
	for _, e := range events {
		data, _ := json.Marshal(e)
		lines = append(lines, data...)
		lines = append(lines, '\n')
	}
	os.WriteFile(filePath, lines, 0644)

	meta, err := buildSessionMeta("s1", "encoded-path", "/decoded/path", filePath)
	if err != nil {
		t.Fatalf("buildSessionMeta failed: %v", err)
	}

	if meta.ID != "s1" {
		t.Errorf("ID: expected 's1', got %q", meta.ID)
	}
	if meta.TurnCount != 2 {
		t.Errorf("TurnCount: expected 2, got %d", meta.TurnCount)
	}
	if meta.Preview != "first question about something" {
		t.Errorf("Preview: expected 'first question about something', got %q", meta.Preview)
	}
	if meta.ProjectPath != "/decoded/path" {
		t.Errorf("ProjectPath: expected '/decoded/path', got %q", meta.ProjectPath)
	}
}

func TestBuildSessionMeta_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(filePath, []byte(""), 0644)

	_, err := buildSessionMeta("s1", "enc", "/dec", filePath)
	if err == nil {
		t.Error("expected error for empty session file")
	}
}
