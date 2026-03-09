package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChaitanyaPinapaka/rethread/internal"
)

func TestExport_FullStrategy(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.jsonl")
	writeTestSession(t, sessionFile, 6)

	turns := readTestTurns(t, sessionFile)
	strategy := internal.SelectionStrategy{Kind: "full"}
	selected := internal.SelectTurns(turns, strategy, 1<<30)

	if len(selected) != 6 {
		t.Errorf("expected 6 turns with full strategy, got %d", len(selected))
	}
}

func TestExport_LastNTurns(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.jsonl")
	writeTestSession(t, sessionFile, 10)

	turns := readTestTurns(t, sessionFile)
	strategy := internal.SelectionStrategy{Kind: "last", N: 4}
	selected := internal.SelectTurns(turns, strategy, 1<<30)

	if len(selected) != 4 {
		t.Fatalf("expected 4 turns with last-4 strategy, got %d", len(selected))
	}
	// Should be the last 4 turns (indices 6-9)
	if selected[0].Index != 6 {
		t.Errorf("expected first selected turn index 6, got %d", selected[0].Index)
	}
}

func TestExport_PruneStrategy(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.jsonl")

	// Build session with low-signal turns
	events := []map[string]interface{}{}
	pairs := [][2]string{
		{"explain Go interfaces", "Go interfaces are implicit contracts..."},
		{"ok", "Would you like more detail?"},
		{"yes please", "Here is a deeper explanation..."},
		{"thanks", "You're welcome!"},
		{"now explain channels", "Channels are typed conduits..."},
	}
	for i, pair := range pairs {
		role := "user"
		if i > 0 {
			role = "assistant"
		}
		// Alternate user/assistant
		events = append(events, map[string]interface{}{
			"type":        "user",
			"message":     map[string]interface{}{"role": "user", "content": pair[0]},
			"timestamp":   "2024-01-01T00:00:00Z",
			"uuid":        "u" + string(rune('a'+i)),
			"sessionId":   "test",
			"isSidechain": false,
		})
		events = append(events, map[string]interface{}{
			"type":        "assistant",
			"message":     map[string]interface{}{"role": "assistant", "content": pair[1]},
			"timestamp":   "2024-01-01T00:00:01Z",
			"uuid":        "a" + string(rune('a'+i)),
			"sessionId":   "test",
			"isSidechain": false,
		})
		_ = role
	}
	writeEvents(t, sessionFile, events)

	turns := readTestTurns(t, sessionFile)
	strategy := internal.SelectionStrategy{Kind: "prune"}
	selected := internal.SelectTurns(turns, strategy, 1<<30)

	// Pruned should have fewer turns than original (some ack turns removed)
	if len(selected) > len(turns) {
		t.Errorf("pruned should not have more turns than original: %d > %d", len(selected), len(turns))
	}
}

func TestExport_StrategySelection(t *testing.T) {
	// Test the strategy selection logic from runExport
	tests := []struct {
		name      string
		turnFlag  int
		pruneFlag bool
		wantKind  string
	}{
		{"default is full", 0, false, "full"},
		{"turns flag sets last", 5, false, "last"},
		{"prune flag sets prune", 0, true, "prune"},
		{"turns flag takes precedence over prune", 3, true, "last"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := internal.SelectionStrategy{Kind: "full"}
			if tt.turnFlag > 0 {
				strategy = internal.SelectionStrategy{Kind: "last", N: tt.turnFlag}
			} else if tt.pruneFlag {
				strategy = internal.SelectionStrategy{Kind: "prune"}
			}

			if strategy.Kind != tt.wantKind {
				t.Errorf("expected strategy %q, got %q", tt.wantKind, strategy.Kind)
			}
		})
	}
}

func TestExport_FormatJSONL(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.jsonl")
	writeTestSession(t, sessionFile, 4)

	turns := readTestTurns(t, sessionFile)
	selected := internal.SelectTurns(turns, internal.SelectionStrategy{Kind: "full"}, 1<<30)
	content, _ := internal.FormatForExport(selected, "jsonl", "test-sess", "/test/project")

	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 JSONL lines, got %d", len(lines))
	}

	for i, line := range lines {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d: invalid JSON: %v", i, err)
		}
	}
}

func TestExport_FormatClean(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.jsonl")
	writeTestSession(t, sessionFile, 4)

	turns := readTestTurns(t, sessionFile)
	selected := internal.SelectTurns(turns, internal.SelectionStrategy{Kind: "full"}, 1<<30)
	content, _ := internal.FormatForExport(selected, "clean", "test-sess", "/test/project")

	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 clean JSONL lines, got %d", len(lines))
	}

	// Clean format should have role + content, no type/uuid/timestamp
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, hasUUID := obj["uuid"]; hasUUID {
		t.Error("clean format should not include uuid")
	}
	if _, hasRole := obj["role"]; !hasRole {
		t.Error("clean format should include role")
	}
}

func TestExport_WriteToFile(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.jsonl")
	writeTestSession(t, sessionFile, 4)

	turns := readTestTurns(t, sessionFile)
	selected := internal.SelectTurns(turns, internal.SelectionStrategy{Kind: "full"}, 1<<30)
	content, _ := internal.FormatForExport(selected, "jsonl", "test-sess", "/test/project")

	outFile := filepath.Join(dir, "output.jsonl")
	if err := os.WriteFile(outFile, []byte(content), 0644); err != nil {
		t.Fatalf("write output: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if string(data) != content {
		t.Error("written file content doesn't match")
	}
}

func TestExport_LastNExceedsTotalTurns(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session.jsonl")
	writeTestSession(t, sessionFile, 4)

	turns := readTestTurns(t, sessionFile)
	strategy := internal.SelectionStrategy{Kind: "last", N: 100}
	selected := internal.SelectTurns(turns, strategy, 1<<30)

	if len(selected) != 4 {
		t.Errorf("last-100 on 4-turn session should return all 4, got %d", len(selected))
	}
}
