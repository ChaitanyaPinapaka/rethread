package internal

import (
	"strings"
	"testing"
)

func TestValidate_BasicSession(t *testing.T) {
	turns := []Turn{
		{
			Role:        "user",
			Content:     "explain Go interfaces",
			ContentText: "explain Go interfaces",
		},
		{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "Go interfaces are implicit contracts..."},
			},
			ContentText: "Go interfaces are implicit contracts...",
		},
	}

	report := Validate(turns, "test", "/project")

	if !report.Passed {
		for _, c := range report.Checks {
			if !c.Passed {
				t.Errorf("check %q failed: %s", c.Name, c.Detail)
			}
		}
	}
}

func TestValidate_WithToolUse(t *testing.T) {
	turns := []Turn{
		{
			Role:        "user",
			Content:     "read the main file",
			ContentText: "read the main file",
		},
		{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "I'll read the file for you."},
				map[string]interface{}{
					"type":  "tool_use",
					"name":  "Read",
					"id":    "t1",
					"input": map[string]interface{}{"file_path": "/src/main.go"},
				},
			},
			ContentText: "I'll read the file for you.",
		},
		{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "The file contains a main function."},
			},
			ContentText: "The file contains a main function.",
		},
	}

	report := Validate(turns, "test", "/project")

	if !report.Passed {
		t.Fatal("all quality checks should pass with tool_use turns")
	}

	// File path should be preserved
	for _, c := range report.Checks {
		if c.Name == "file_paths_preserved" && !c.Passed {
			t.Errorf("file paths should be preserved: %s", c.Detail)
		}
	}
}

func TestValidate_CompactSavesBytes(t *testing.T) {
	bigOld := strings.Repeat("old code line\n", 50)
	bigNew := strings.Repeat("new code line\n", 50)

	turns := []Turn{
		{
			Role:        "user",
			Content:     "fix the bug",
			ContentText: "fix the bug",
		},
		{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "I'll fix the bug in main.go."},
				map[string]interface{}{
					"type": "tool_use",
					"name": "Edit",
					"id":   "t1",
					"input": map[string]interface{}{
						"file_path":  "/src/main.go",
						"old_string": bigOld,
						"new_string": bigNew,
					},
				},
			},
			ContentText: "I'll fix the bug in main.go.",
		},
	}

	report := Validate(turns, "test", "/project")

	if report.SavedBytes <= 0 {
		t.Errorf("compact should save bytes on Edit with large diffs, saved=%d", report.SavedBytes)
	}

	if report.Ratio >= 1.0 {
		t.Errorf("compact ratio should be < 1.0, got %.2f", report.Ratio)
	}

	if !report.Passed {
		for _, c := range report.Checks {
			if !c.Passed {
				t.Errorf("check %q failed: %s", c.Name, c.Detail)
			}
		}
	}
}

func TestValidate_BlockStats(t *testing.T) {
	turns := []Turn{
		{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{"type": "thinking", "thinking": "Let me think about this..."},
				map[string]interface{}{"type": "text", "text": "Here's the answer."},
				map[string]interface{}{
					"type":  "tool_use",
					"name":  "Write",
					"input": map[string]interface{}{"file_path": "out.go", "content": strings.Repeat("x\n", 100)},
				},
				map[string]interface{}{"type": "tool_result", "content": "success"},
			},
			ContentText: "Here's the answer.",
		},
	}

	report := Validate(turns, "test", "/project")

	// Should have stats for text, thinking, Write (tool name), tool_result
	if _, ok := report.BlockStats["text"]; !ok {
		t.Error("should have text block stats")
	}
	if _, ok := report.BlockStats["thinking"]; !ok {
		t.Error("should have thinking block stats")
	}
	if _, ok := report.BlockStats["Write"]; !ok {
		t.Error("should have Write block stats")
	}

	// Write tool with big content should have significant savings
	writeStats := report.BlockStats["Write"]
	if writeStats.Ratio >= 1.0 {
		t.Errorf("Write tool should have ratio < 1.0, got %.2f", writeStats.Ratio)
	}

	// tool_result should have 0 compact bytes (dropped in clean format)
	trStats := report.BlockStats["tool_result"]
	if trStats.CompactBytes != 0 {
		t.Errorf("tool_result should have 0 compact bytes, got %d", trStats.CompactBytes)
	}
}

func TestValidate_UserMessageIntegrity(t *testing.T) {
	turns := []Turn{
		{
			Role:        "user",
			Content:     "implement a cache with TTL support",
			ContentText: "implement a cache with TTL support",
		},
		{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "I'll implement the cache."},
			},
			ContentText: "I'll implement the cache.",
		},
		{
			Role:        "user",
			Content:     "add thread safety with sync.RWMutex",
			ContentText: "add thread safety with sync.RWMutex",
		},
		{
			Role: "assistant",
			Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "Done, added RWMutex guards."},
			},
			ContentText: "Done, added RWMutex guards.",
		},
	}

	report := Validate(turns, "test", "/project")

	for _, c := range report.Checks {
		if c.Name == "user_messages_preserved" && !c.Passed {
			t.Fatalf("user messages must be preserved: %s", c.Detail)
		}
		if c.Name == "assistant_text_preserved" && !c.Passed {
			t.Fatalf("assistant text must be preserved: %s", c.Detail)
		}
	}
}

func TestFormatReport(t *testing.T) {
	report := CompactReport{
		RawBytes:     1000,
		CompactBytes: 400,
		SavedBytes:   600,
		Ratio:        0.4,
		BlockStats: map[string]BlockStat{
			"text":     {Count: 5, RawBytes: 200, CompactBytes: 200, Ratio: 1.0},
			"tool_use": {Count: 3, RawBytes: 800, CompactBytes: 200, Ratio: 0.25},
		},
		Checks: []QualityCheck{
			{Name: "user_messages_preserved", Passed: true},
			{Name: "turn_count_matches", Passed: true},
		},
		Passed: true,
	}

	output := FormatReport(report)

	if !strings.Contains(output, "60%") {
		t.Error("should show 60% reduction")
	}
	if !strings.Contains(output, "ALL CHECKS PASSED") {
		t.Error("should show all passed")
	}
	if !strings.Contains(output, "tool_use") {
		t.Error("should show tool_use in breakdown")
	}
}

func TestFormatReport_WithFailure(t *testing.T) {
	report := CompactReport{
		Checks: []QualityCheck{
			{Name: "test_check", Passed: false, Detail: "something broke"},
		},
		Passed: false,
	}

	output := FormatReport(report)

	if !strings.Contains(output, "FAIL") {
		t.Error("should show FAIL for failed check")
	}
	if !strings.Contains(output, "SOME CHECKS FAILED") {
		t.Error("should show failure result")
	}
}
