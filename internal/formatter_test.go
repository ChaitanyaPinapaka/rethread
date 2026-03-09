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

	result, _ := FormatForExport(turns, "jsonl", "sess1", "/project")
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

	result, _ := FormatForExport(turns, "clean", "sess1", "/project")

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

	// id should be stripped in clean format
	for _, b := range blocks {
		m := b.(map[string]interface{})
		if m["type"] == "tool_use" {
			if _, hasID := m["id"]; hasID {
				t.Error("clean format should strip tool_use id")
			}
		}
	}
}

func TestFormatForExport_UnknownFormatFallsBackToJSONL(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "hello", ContentText: "hello"},
	}

	result, _ := FormatForExport(turns, "unknown_format", "s1", "/p")
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(result), &obj); err != nil {
		t.Fatalf("unknown format should still produce JSONL, got error: %v", err)
	}
	if obj["type"] != "user" {
		t.Errorf("expected user JSONL output, got %v", obj["type"])
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

// --- clean format condensing ---

func TestCleanContent_CondensesEdit(t *testing.T) {
	longOld := strings.Repeat("x", 500)
	longNew := strings.Repeat("y", 500)
	content := []interface{}{
		map[string]interface{}{
			"type":  "tool_use",
			"name":  "Edit",
			"id":    "t1",
			"input": map[string]interface{}{"file_path": "main.go", "old_string": longOld, "new_string": longNew},
		},
	}

	result := cleanContent(content)
	blocks := result.([]interface{})
	m := blocks[0].(map[string]interface{})
	input := m["input"].(map[string]interface{})

	// old_string and new_string should be truncated
	old := input["old_string"].(string)
	if len(old) >= 500 {
		t.Errorf("clean should truncate old_string, got len %d", len(old))
	}
	if !strings.Contains(old, "more chars") {
		t.Error("truncated field should have [...N more chars] marker")
	}

	// id should be stripped
	if _, hasID := m["id"]; hasID {
		t.Error("clean format should strip tool_use id")
	}
}

func TestCleanContent_CondensesWrite(t *testing.T) {
	bigContent := strings.Repeat("line\n", 100)
	content := []interface{}{
		map[string]interface{}{
			"type":  "tool_use",
			"name":  "Write",
			"input": map[string]interface{}{"file_path": "out.go", "content": bigContent},
		},
	}

	result := cleanContent(content)
	blocks := result.([]interface{})
	m := blocks[0].(map[string]interface{})
	input := m["input"].(map[string]interface{})

	c := input["content"].(string)
	if !strings.Contains(c, "lines]") {
		t.Errorf("clean Write should show line count, got: %s", c)
	}
}

func TestCleanContent_KeepsReadPath(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{
			"type":  "tool_use",
			"name":  "Read",
			"input": map[string]interface{}{"file_path": "/src/main.go", "offset": 10.0, "limit": 50.0},
		},
	}

	result := cleanContent(content)
	blocks := result.([]interface{})
	m := blocks[0].(map[string]interface{})
	input := m["input"].(map[string]interface{})

	if input["file_path"] != "/src/main.go" {
		t.Error("clean Read should keep file_path")
	}
	if input["offset"] != 10.0 {
		t.Error("clean Read should keep offset")
	}
}

func TestCondenseToolInput_UnknownTool(t *testing.T) {
	longVal := strings.Repeat("z", 300)
	input := map[string]interface{}{
		"short_key": "small",
		"long_key":  longVal,
	}

	result := condenseToolInput("SomeCustomTool", input)
	m := result.(map[string]interface{})

	if m["short_key"] != "small" {
		t.Error("short values should be kept as-is")
	}
	truncated := m["long_key"].(string)
	if len(truncated) >= 300 {
		t.Errorf("long values in unknown tools should be truncated, got len %d", len(truncated))
	}
}

func TestTruncateField(t *testing.T) {
	short := "hello"
	if TruncateField(short, 80) != "hello" {
		t.Error("short strings should not be truncated")
	}

	long := strings.Repeat("a", 200)
	result := TruncateField(long, 80)
	if len(result) > 120 { // 80 + marker
		t.Errorf("truncated string too long: %d", len(result))
	}
	if !strings.Contains(result, "120 more chars") {
		t.Errorf("expected marker with 120 more chars, got: %s", result)
	}
}

// --- ExportStats ---

func TestFormatForExport_CleanStats(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "hello", ContentText: "hello"},
		{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{"type": "tool_result", "content": "ok"},
			},
			ContentText: "",
		},
		{Role: "assistant", Content: "bye", ContentText: "bye"},
	}

	_, stats := FormatForExport(turns, "clean", "s1", "/p")
	if stats.TurnsInput != 3 {
		t.Errorf("expected 3 input turns, got %d", stats.TurnsInput)
	}
	if stats.TurnsOutput != 2 {
		t.Errorf("expected 2 output turns (tool_result-only dropped), got %d", stats.TurnsOutput)
	}
	if stats.TurnsDropped != 1 {
		t.Errorf("expected 1 dropped turn, got %d", stats.TurnsDropped)
	}
}

func TestFormatForExport_JSONLStats(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "hello", ContentText: "hello"},
		{Role: "assistant", Content: "bye", ContentText: "bye"},
	}

	_, stats := FormatForExport(turns, "jsonl", "s1", "/p")
	if stats.TurnsInput != 2 {
		t.Errorf("expected 2 input turns, got %d", stats.TurnsInput)
	}
	if stats.TurnsOutput != 2 {
		t.Errorf("expected 2 output turns, got %d", stats.TurnsOutput)
	}
	if stats.TurnsDropped != 0 {
		t.Errorf("expected 0 dropped turns, got %d", stats.TurnsDropped)
	}
}
