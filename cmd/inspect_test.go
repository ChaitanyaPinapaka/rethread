package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ChaitanyaPinapaka/rethread/internal"
)

func TestRunInspect_FullStrategy(t *testing.T) {
	// Create a temp session file
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	writeTestSession(t, filePath, 4)

	// Override findSession by calling runInspect with a session we control
	// We test the output logic indirectly via AnalyzeTurns + the print path
	turns := readTestTurns(t, filePath)
	a := internal.AnalyzeTurns(turns)

	if a.TotalTurns != 4 {
		t.Errorf("expected 4 total turns, got %d", a.TotalTurns)
	}
	if a.RecommendedStrategy.Kind != "full" {
		t.Errorf("expected 'full' strategy for small session, got %q", a.RecommendedStrategy.Kind)
	}
	if !a.FitsInContext {
		t.Error("small session should fit in context")
	}
}

func TestRunInspect_PruneStrategy(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	// Build a session with some low-signal turns that exceeds budget when not pruned
	events := buildLargeSession(200, true)
	writeEvents(t, filePath, events)

	turns := readTestTurns(t, filePath)
	a := internal.AnalyzeTurns(turns)

	if a.LowSignalTurns == 0 {
		t.Error("expected some low-signal turns")
	}
	if a.TotalTurns != 200 {
		t.Errorf("expected 200 turns, got %d", a.TotalTurns)
	}
}

func TestRunInspect_LastStrategy(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")

	// Build a very large session that can't be pruned to fit
	events := buildLargeSession(200, false)
	writeEvents(t, filePath, events)

	turns := readTestTurns(t, filePath)
	a := internal.AnalyzeTurns(turns)

	if a.RecommendedStrategy.Kind != "last" {
		t.Errorf("expected 'last' strategy for very large session, got %q", a.RecommendedStrategy.Kind)
	}
	if a.FitsInContext {
		t.Error("large session should not fit in context")
	}
	if a.RecommendedStrategy.N < 4 {
		t.Errorf("expected N >= 4, got %d", a.RecommendedStrategy.N)
	}
}

func TestRunInspect_TurnCounts(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	writeTestSession(t, filePath, 6)

	turns := readTestTurns(t, filePath)
	a := internal.AnalyzeTurns(turns)

	if a.UserTurns != 3 {
		t.Errorf("expected 3 user turns, got %d", a.UserTurns)
	}
	if a.AssistantTurns != 3 {
		t.Errorf("expected 3 assistant turns, got %d", a.AssistantTurns)
	}
}

func TestRunInspect_FitsInContext(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "session.jsonl")
	writeTestSession(t, filePath, 2)

	turns := readTestTurns(t, filePath)
	a := internal.AnalyzeTurns(turns)

	if !a.FitsInContext {
		t.Error("2-turn session should fit in context")
	}
	if a.TotalTokenEstimate <= 0 {
		t.Error("token estimate should be positive")
	}
}

// --- helpers ---

func writeTestSession(t *testing.T, filePath string, turnCount int) {
	t.Helper()
	events := buildSimpleSession(turnCount)
	writeEvents(t, filePath, events)
}

func buildSimpleSession(turnCount int) []map[string]interface{} {
	var events []map[string]interface{}
	for i := 0; i < turnCount; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		content := "This is a detailed message with enough content to be meaningful"
		events = append(events, map[string]interface{}{
			"type": role,
			"message": map[string]interface{}{
				"role":    role,
				"content": content,
			},
			"timestamp":   "2024-01-01T00:00:00Z",
			"uuid":        "u" + string(rune('0'+i)),
			"sessionId":   "test-session",
			"isSidechain": false,
		})
	}
	return events
}

func buildLargeSession(turnCount int, withLowSignal bool) []map[string]interface{} {
	var events []map[string]interface{}
	// Use long content so token estimates are high
	longText := ""
	for i := 0; i < 3000; i++ {
		longText += "word "
	}

	for i := 0; i < turnCount; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		content := longText
		if withLowSignal && i%10 == 4 {
			content = "ok"
		}
		events = append(events, map[string]interface{}{
			"type": role,
			"message": map[string]interface{}{
				"role":    role,
				"content": content,
			},
			"timestamp":   "2024-01-01T00:00:00Z",
			"uuid":        "u-large-" + string(rune(i)),
			"sessionId":   "test-large",
			"isSidechain": false,
		})
	}
	return events
}

func writeEvents(t *testing.T, filePath string, events []map[string]interface{}) {
	t.Helper()
	var lines []byte
	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}
		lines = append(lines, data...)
		lines = append(lines, '\n')
	}
	if err := os.WriteFile(filePath, lines, 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}
}

func readTestTurns(t *testing.T, filePath string) []internal.Turn {
	t.Helper()
	turns, err := internal.ReadTurns(filePath, false)
	if err != nil {
		t.Fatalf("ReadTurns failed: %v", err)
	}
	return turns
}
