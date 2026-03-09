package internal

import (
	"testing"
)

func makeTurns(roles []string, texts []string, tokens []int) []Turn {
	turns := make([]Turn, len(roles))
	for i := range roles {
		text := ""
		if i < len(texts) {
			text = texts[i]
		}
		tok := 10
		if i < len(tokens) {
			tok = tokens[i]
		}
		turns[i] = Turn{
			Index:         i,
			Role:          roles[i],
			ContentText:   text,
			TokenEstimate: tok,
		}
	}
	return turns
}

func TestSelectTurns_Full(t *testing.T) {
	turns := makeTurns(
		[]string{"user", "assistant", "user", "assistant"},
		[]string{"hello", "hi", "how?", "fine"},
		[]int{10, 10, 10, 10},
	)

	selected := SelectTurns(turns, SelectionStrategy{Kind: "full"}, 1<<30)
	if len(selected) != 4 {
		t.Fatalf("expected 4 turns, got %d", len(selected))
	}
}

func TestSelectTurns_Last(t *testing.T) {
	turns := makeTurns(
		[]string{"user", "assistant", "user", "assistant", "user", "assistant"},
		[]string{"a", "b", "c", "d", "e", "f"},
		[]int{10, 10, 10, 10, 10, 10},
	)

	selected := SelectTurns(turns, SelectionStrategy{Kind: "last", N: 2}, 1<<30)
	if len(selected) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(selected))
	}
	if selected[0].ContentText != "e" || selected[1].ContentText != "f" {
		t.Errorf("expected last 2 turns, got %q and %q", selected[0].ContentText, selected[1].ContentText)
	}
}

func TestSelectTurns_LastExceedsLength(t *testing.T) {
	turns := makeTurns([]string{"user", "assistant"}, []string{"a", "b"}, []int{10, 10})

	selected := SelectTurns(turns, SelectionStrategy{Kind: "last", N: 10}, 1<<30)
	if len(selected) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(selected))
	}
}

func TestSelectTurns_Prune(t *testing.T) {
	turns := makeTurns(
		[]string{"user", "assistant", "user", "assistant", "user", "assistant"},
		[]string{"start question", "detailed answer", "ok", "more detail", "thanks", "final answer"},
		[]int{10, 10, 10, 10, 10, 10},
	)

	selected := SelectTurns(turns, SelectionStrategy{Kind: "prune"}, 1<<30)
	// "ok" and "thanks" are low-signal, but pruning keeps first 2 and last 2
	if len(selected) < 4 {
		t.Errorf("expected at least 4 turns after pruning, got %d", len(selected))
	}
}

func TestSelectTurns_Range(t *testing.T) {
	turns := makeTurns(
		[]string{"user", "assistant", "user", "assistant", "user"},
		[]string{"a", "b", "c", "d", "e"},
		[]int{10, 10, 10, 10, 10},
	)

	selected := SelectTurns(turns, SelectionStrategy{Kind: "range", From: 1, To: 3}, 1<<30)
	if len(selected) != 3 {
		t.Fatalf("expected 3 turns, got %d", len(selected))
	}
	if selected[0].ContentText != "b" {
		t.Errorf("expected first selected turn to be 'b', got %q", selected[0].ContentText)
	}
}

func TestSelectTurns_RangeOutOfBoundsReturnsEmpty(t *testing.T) {
	turns := makeTurns(
		[]string{"user", "assistant", "user"},
		[]string{"a", "b", "c"},
		[]int{10, 10, 10},
	)

	selected := SelectTurns(turns, SelectionStrategy{Kind: "range", From: 10, To: 20}, 1<<30)
	if len(selected) != 0 {
		t.Fatalf("expected 0 turns for out-of-bounds range, got %d", len(selected))
	}
}

func TestSelectTurns_DefaultStrategy(t *testing.T) {
	turns := makeTurns([]string{"user", "assistant"}, []string{"a", "b"}, []int{10, 10})

	selected := SelectTurns(turns, SelectionStrategy{Kind: "unknown"}, 1<<30)
	if len(selected) != 2 {
		t.Fatalf("unknown strategy should return all turns, got %d", len(selected))
	}
}

func TestSelectTurns_TokenBudget(t *testing.T) {
	turns := makeTurns(
		[]string{"user", "assistant", "user", "assistant", "user", "assistant"},
		[]string{"a", "b", "c", "d", "e", "f"},
		[]int{100, 100, 100, 100, 100, 100},
	)

	// Budget of 350 can't fit all 600 tokens
	selected := SelectTurns(turns, SelectionStrategy{Kind: "full"}, 350)
	totalTokens := 0
	for _, s := range selected {
		totalTokens += s.TokenEstimate
	}
	if totalTokens > 350 {
		t.Errorf("token budget exceeded: %d > 350", totalTokens)
	}
}

func TestSelectTurns_ZeroBudgetUsesDefault(t *testing.T) {
	turns := makeTurns([]string{"user", "assistant"}, []string{"a", "b"}, []int{10, 10})

	selected := SelectTurns(turns, SelectionStrategy{Kind: "full"}, 0)
	if len(selected) != 2 {
		t.Fatalf("expected 2 turns with default budget, got %d", len(selected))
	}
}

// --- AnalyzeTurns ---

func TestAnalyzeTurns_Basic(t *testing.T) {
	turns := []Turn{
		{Role: "user", ContentText: "hello world", TokenEstimate: 3},
		{Role: "assistant", ContentText: "hi there friend", TokenEstimate: 4},
		{Role: "user", ContentText: "ok", TokenEstimate: 1},
		{Role: "assistant", ContentText: "response here", TokenEstimate: 3, IsSidechain: true},
	}

	a := AnalyzeTurns(turns)

	if a.TotalTurns != 4 {
		t.Errorf("TotalTurns: expected 4, got %d", a.TotalTurns)
	}
	if a.UserTurns != 2 {
		t.Errorf("UserTurns: expected 2, got %d", a.UserTurns)
	}
	if a.AssistantTurns != 2 {
		t.Errorf("AssistantTurns: expected 2, got %d", a.AssistantTurns)
	}
	if a.SidechainTurns != 1 {
		t.Errorf("SidechainTurns: expected 1, got %d", a.SidechainTurns)
	}
	if a.TotalTokenEstimate != 11 {
		t.Errorf("TotalTokenEstimate: expected 11, got %d", a.TotalTokenEstimate)
	}
	if !a.FitsInContext {
		t.Error("expected FitsInContext to be true for small session")
	}
}

func TestAnalyzeTurns_LowSignalDetection(t *testing.T) {
	turns := []Turn{
		{Role: "user", ContentText: "explain X", TokenEstimate: 5},
		{Role: "assistant", ContentText: "X is...", TokenEstimate: 10},
		{Role: "user", ContentText: "ok", TokenEstimate: 1},
		{Role: "assistant", ContentText: "anything else?", TokenEstimate: 3},
	}

	a := AnalyzeTurns(turns)
	if a.LowSignalTurns != 1 {
		t.Errorf("expected 1 low-signal turn, got %d", a.LowSignalTurns)
	}
}

func TestAnalyzeTurns_RecommendsFull(t *testing.T) {
	turns := makeTurns([]string{"user", "assistant"}, []string{"a", "b"}, []int{10, 10})
	a := AnalyzeTurns(turns)
	if a.RecommendedStrategy.Kind != "full" {
		t.Errorf("expected 'full' strategy, got %q", a.RecommendedStrategy.Kind)
	}
}

func TestAnalyzeTurns_RecommendsLastForLargeSessions(t *testing.T) {
	// Create a session that exceeds DefaultTokenBudget and can't be pruned
	roles := make([]string, 100)
	texts := make([]string, 100)
	tokens := make([]int, 100)
	for i := range roles {
		if i%2 == 0 {
			roles[i] = "user"
			texts[i] = "detailed question with lots of context about the problem"
		} else {
			roles[i] = "assistant"
			texts[i] = "detailed answer with explanation and code examples"
		}
		tokens[i] = 5000
	}
	turns := makeTurns(roles, texts, tokens)

	a := AnalyzeTurns(turns)
	if a.RecommendedStrategy.Kind != "last" {
		t.Errorf("expected 'last' strategy for large session, got %q", a.RecommendedStrategy.Kind)
	}
	if a.RecommendedStrategy.N < 4 {
		t.Errorf("expected N >= 4, got %d", a.RecommendedStrategy.N)
	}
}

// --- isLowSignal ---

func TestIsLowSignal(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"ok", true},
		{"OK", true},
		{"yes", true},
		{"Yeah", true},
		{"thanks", true},
		{"Thank you", true},
		{"sounds good", true},
		{"lgtm", true},
		{"looks good", true},
		{"understood", true},
		{"ack", true},
		// Not low-signal
		{"please refactor the auth module", false},
		{"can you add error handling?", false},
		{"", false},
		{"here is the code:\n```go\nfunc main() {}\n```", false},
	}

	for _, tt := range tests {
		turn := Turn{ContentText: tt.text}
		got := isLowSignal(turn)
		if got != tt.want {
			t.Errorf("isLowSignal(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

// --- pruneLowSignal ---

func TestPruneLowSignal_KeepsSmallSessions(t *testing.T) {
	turns := makeTurns([]string{"user", "assistant"}, []string{"ok", "sure"}, []int{1, 1})
	result := pruneLowSignal(turns, 0)
	if len(result) != 2 {
		t.Errorf("small sessions should not be pruned, got %d turns", len(result))
	}
}

func TestPruneLowSignal_RemovesAckTurns(t *testing.T) {
	turns := []Turn{
		{Index: 0, Role: "user", ContentText: "explain X"},
		{Index: 1, Role: "assistant", ContentText: "X is a thing"},
		{Index: 2, Role: "user", ContentText: "ok"},
		{Index: 3, Role: "assistant", ContentText: "anything else?"},
		{Index: 4, Role: "user", ContentText: "now do Y"},
		{Index: 5, Role: "assistant", ContentText: "Y is done"},
	}

	result := pruneLowSignal(turns, 0)
	// "ok" is low-signal but the pruner keeps context around it
	// At minimum, first 2 and last 2 are always kept
	if len(result) < 4 {
		t.Errorf("expected at least 4 turns, got %d", len(result))
	}
}

func TestPruneLowSignal_KeepsCodeTurns(t *testing.T) {
	turns := []Turn{
		{Index: 0, Role: "user", ContentText: "start"},
		{Index: 1, Role: "assistant", ContentText: "ok"},
		{Index: 2, Role: "user", ContentText: "here is code:\n```go\nfunc main() {}\n```"},
		{Index: 3, Role: "assistant", ContentText: "I see the code"},
		{Index: 4, Role: "user", ContentText: "end"},
		{Index: 5, Role: "assistant", ContentText: "done"},
	}

	result := pruneLowSignal(turns, 0)
	// Turn 2 has code (```), should be kept
	found := false
	for _, r := range result {
		if r.Index == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("turn with code block should be kept after pruning")
	}
}

// --- enforceTokenBudget ---

func TestEnforceTokenBudget_FitsWithin(t *testing.T) {
	turns := makeTurns([]string{"user", "assistant"}, []string{"a", "b"}, []int{50, 50})
	result := enforceTokenBudget(turns, 200)
	if len(result) != 2 {
		t.Errorf("expected 2 turns when within budget, got %d", len(result))
	}
}

func TestEnforceTokenBudget_KeepsSkeleton(t *testing.T) {
	turns := makeTurns(
		[]string{"user", "assistant", "user", "assistant", "user", "assistant"},
		[]string{"a", "b", "c", "d", "e", "f"},
		[]int{10, 10, 100, 100, 10, 10},
	)

	// Budget allows skeleton (20) + last 2 (20) but not middle (200)
	result := enforceTokenBudget(turns, 50)

	// Should keep first 2 (skeleton) and some recent turns
	if len(result) < 2 {
		t.Errorf("expected at least skeleton turns, got %d", len(result))
	}
	if result[0].ContentText != "a" {
		t.Errorf("first turn should be skeleton, got %q", result[0].ContentText)
	}
}

func TestEnforceTokenBudget_TinyBudget(t *testing.T) {
	turns := makeTurns([]string{"user", "assistant"}, []string{"a", "b"}, []int{100, 100})

	result := enforceTokenBudget(turns, 100)
	// Should trim from front
	if len(result) == 0 {
		t.Error("should return at least one turn")
	}
	total := 0
	for _, r := range result {
		total += r.TokenEstimate
	}
	if total > 100 {
		t.Errorf("budget exceeded: %d > 100", total)
	}
}
