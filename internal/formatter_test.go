package internal

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatForExport_JSONL(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "hello", ContentText: "hello", Timestamp: "2024-01-01T00:00:00Z", UUID: "u1"},
		{Role: "assistant", Content: "hi", ContentText: "hi", Timestamp: "2024-01-01T00:01:00Z", UUID: "u2"},
	}

	result := FormatForExport(turns, "jsonl", "sess1", "/project")
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d", len(lines))
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &obj); err != nil {
		t.Fatalf("invalid JSON in first line: %v", err)
	}
	if obj["type"] != "user" {
		t.Errorf("expected type 'user', got %v", obj["type"])
	}
}

func TestFormatForExport_CleanJSONL(t *testing.T) {
	turns := []Turn{
		{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "hello"},
				map[string]interface{}{"type": "tool_result", "content": "result"},
				map[string]interface{}{"type": "tool_use", "name": "read", "input": "file.go", "id": "t1"},
			},
			ContentText: "hello",
		},
	}

	result := FormatForExport(turns, "clean", "sess1", "/project")

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	blocks, ok := obj["content"].([]interface{})
	if !ok {
		t.Fatal("content should be an array")
	}

	// tool_result should be filtered out
	for _, b := range blocks {
		m := b.(map[string]interface{})
		if m["type"] == "tool_result" {
			t.Error("tool_result should be filtered out in clean format")
		}
	}

	// text and tool_use should remain
	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks (text + tool_use), got %d", len(blocks))
	}
}

func TestFormatForExport_Markdown(t *testing.T) {
	turns := []Turn{
		{Role: "user", ContentText: "explain X", Timestamp: "2024-01-15T14:30:00Z"},
		{Role: "assistant", ContentText: "X is a thing"},
	}

	result := FormatForExport(turns, "markdown", "sess1", "/project")

	if !strings.Contains(result, "# Conversation Export") {
		t.Error("markdown should contain header")
	}
	if !strings.Contains(result, "sess1") {
		t.Error("markdown should contain session ID")
	}
	if !strings.Contains(result, "explain X") {
		t.Error("markdown should contain turn content")
	}
	if !strings.Contains(result, "User") {
		t.Error("markdown should contain User role")
	}
	if !strings.Contains(result, "Assistant") {
		t.Error("markdown should contain Assistant role")
	}
}

func TestFormatForExport_Turns(t *testing.T) {
	turns := []Turn{
		{Role: "user", ContentText: "hello", Timestamp: "2024-01-01T00:00:00Z"},
		{Role: "assistant", ContentText: "hi"},
	}

	result := FormatForExport(turns, "turns", "sess1", "/project")

	if !strings.Contains(result, "<conversation>") {
		t.Error("turns format should contain <conversation> tag")
	}
	if !strings.Contains(result, "<turn role=\"user\"") {
		t.Error("turns format should contain <turn> tags")
	}
	if !strings.Contains(result, "</conversation>") {
		t.Error("turns format should contain closing </conversation> tag")
	}
}

func TestFormatForExport_DefaultFormat(t *testing.T) {
	turns := []Turn{
		{Role: "user", ContentText: "hello"},
	}

	// Unknown format falls through to structured turns
	result := FormatForExport(turns, "unknown_format", "s1", "/p")
	if !strings.Contains(result, "<conversation>") {
		t.Error("unknown format should fall back to structured turns")
	}
}

// --- cleanContent ---

func TestCleanContent_StringContent(t *testing.T) {
	result := cleanContent("plain text")
	if result != "plain text" {
		t.Errorf("string content should pass through, got %v", result)
	}
}

func TestCleanContent_FiltersToolResult(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "hello"},
		map[string]interface{}{"type": "tool_result", "content": "output"},
	}

	result := cleanContent(content)
	blocks := result.([]interface{})
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block after filtering, got %d", len(blocks))
	}
}

func TestCleanContent_KeepsThinking(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "thinking", "thinking": "hmm", "signature": "sig123"},
	}

	result := cleanContent(content)
	blocks := result.([]interface{})
	if len(blocks) != 1 {
		t.Fatalf("expected 1 thinking block, got %d", len(blocks))
	}
	m := blocks[0].(map[string]interface{})
	if _, hasSignature := m["signature"]; hasSignature {
		t.Error("signature should be stripped from thinking blocks")
	}
}

func TestCleanContent_EmptyBlocks(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "tool_result", "content": "output"},
	}

	result := cleanContent(content)
	if result != nil {
		t.Errorf("all-filtered content should return nil, got %v", result)
	}
}

// --- formatMarkdown timestamp ---

func TestFormatMarkdown_WithTimestamp(t *testing.T) {
	turns := []Turn{
		{Role: "user", ContentText: "hello", Timestamp: "2024-06-15T14:30:00.123Z"},
	}

	result := formatMarkdown(turns, "", "")
	if !strings.Contains(result, "Jun 15, 2024") {
		t.Errorf("expected formatted timestamp, got: %s", result)
	}
}

func TestFormatMarkdown_WithoutTimestamp(t *testing.T) {
	turns := []Turn{
		{Role: "assistant", ContentText: "response"},
	}

	result := formatMarkdown(turns, "", "")
	if !strings.Contains(result, "Assistant") {
		t.Error("should contain Assistant role")
	}
}
