package internal

import (
	"regexp"
	"strings"
)

const (
	DefaultTokenBudget = 150_000
)

// SelectTurns applies a selection strategy and enforces token budget.
func SelectTurns(turns []Turn, strategy SelectionStrategy, tokenBudget int) []Turn {
	if tokenBudget == 0 {
		tokenBudget = DefaultTokenBudget
	}

	var selected []Turn

	switch strategy.Kind {
	case "full":
		selected = make([]Turn, len(turns))
		copy(selected, turns)
	case "last":
		if strategy.N >= len(turns) {
			selected = make([]Turn, len(turns))
			copy(selected, turns)
		} else {
			selected = make([]Turn, strategy.N)
			copy(selected, turns[len(turns)-strategy.N:])
		}
	case "prune":
		selected = pruneLowSignal(turns, 0)
	case "range":
		to := strategy.To + 1
		if to > len(turns) {
			to = len(turns)
		}
		from := strategy.From
		if from < 0 {
			from = 0
		}
		selected = make([]Turn, to-from)
		copy(selected, turns[from:to])
	default:
		selected = make([]Turn, len(turns))
		copy(selected, turns)
	}

	return enforceTokenBudget(selected, tokenBudget)
}

// AnalyzeTurns returns stats about a session's turns.
func AnalyzeTurns(turns []Turn) TurnAnalysis {
	total := sumTokens(turns)
	var userCount, asstCount, sideCount, lowCount int

	for _, t := range turns {
		switch t.Role {
		case "user":
			userCount++
		case "assistant":
			asstCount++
		}
		if t.IsSidechain {
			sideCount++
		}
		if isLowSignal(t) {
			lowCount++
		}
	}

	return TurnAnalysis{
		TotalTurns:          len(turns),
		UserTurns:           userCount,
		AssistantTurns:      asstCount,
		SidechainTurns:      sideCount,
		LowSignalTurns:      lowCount,
		TotalTokenEstimate:  total,
		FitsInContext:        total <= DefaultTokenBudget,
		RecommendedStrategy: recommendStrategy(turns, total),
	}
}

// --- acknowledgment patterns ---

var filePathPattern = regexp.MustCompile(`[/\\]\w+\.\w+`)

var ackPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(ok|okay|yes|yeah|yep|sure|got it|thanks|thank you|cool|nice|great|good|perfect|right|agreed|exactly|correct)\.?$`),
	regexp.MustCompile(`(?i)^(sounds good|makes sense|let'?s do it|go ahead|lgtm|looks good|that works|works for me)\.?$`),
	regexp.MustCompile(`(?i)^(understood|noted|ack|roger|will do|on it)\.?$`),
}

func isLowSignal(turn Turn) bool {
	text := strings.TrimSpace(strings.ToLower(turn.ContentText))
	if text == "" {
		return false
	}

	for _, pat := range ackPatterns {
		if pat.MatchString(text) {
			return true
		}
	}

	return false
}

// --- pruning ---

func pruneLowSignal(turns []Turn, _ int) []Turn {
	if len(turns) <= 4 {
		result := make([]Turn, len(turns))
		copy(result, turns)
		return result
	}

	keep := make(map[int]bool)

	// Always keep first 2 and last 2
	keep[0] = true
	if len(turns) > 1 {
		keep[1] = true
	}
	keep[len(turns)-2] = true
	keep[len(turns)-1] = true

	for i, turn := range turns {
		if keep[i] {
			continue
		}

		text := turn.ContentText

		// Keep turns with code
		if strings.Contains(text, "```") || strings.Contains(text, "    ") {
			keep[i] = true
			continue
		}

		// Keep turns with URLs or file paths
		if strings.Contains(text, "http://") || strings.Contains(text, "https://") {
			keep[i] = true
			continue
		}
		if filePathPattern.MatchString(text) {
			keep[i] = true
			continue
		}

		// Only drop if it matches acknowledgment patterns
		if !isLowSignal(turn) {
			keep[i] = true
		}
	}

	// Don't orphan turns: if we keep an assistant turn, keep the preceding user turn
	final := make(map[int]bool)
	for k, v := range keep {
		final[k] = v
	}

	for idx := range keep {
		if idx > 0 && turns[idx].Role == "assistant" && !final[idx-1] {
			final[idx-1] = true
		}
		if idx < len(turns)-1 && turns[idx].Role == "user" && !final[idx+1] {
			final[idx+1] = true
		}
	}

	var result []Turn
	for i, turn := range turns {
		if final[i] {
			result = append(result, turn)
		}
	}
	return result
}

// --- token budget enforcement ---
//
// Strategy: skeleton + recent
// Keep first 2 turns (problem frame) + as many recent turns as fit.
// Maps to "Lost in the Middle" research: beginning sets frame, end has live context.

func enforceTokenBudget(turns []Turn, budget int) []Turn {
	total := sumTokens(turns)
	if total <= budget {
		return turns
	}

	if len(turns) <= 2 {
		return trimFromFront(turns, budget)
	}

	// Keep first 2 turns as skeleton
	skeletonEnd := 2
	if skeletonEnd > len(turns) {
		skeletonEnd = len(turns)
	}
	skeleton := turns[:skeletonEnd]
	skeletonTokens := sumTokens(skeleton)
	remaining := budget - skeletonTokens

	if remaining <= 0 {
		return trimFromFront(turns, budget)
	}

	// Fill from the end
	var recent []Turn
	used := 0
	for i := len(turns) - 1; i >= skeletonEnd; i-- {
		if used+turns[i].TokenEstimate > remaining {
			break
		}
		recent = append([]Turn{turns[i]}, recent...)
		used += turns[i].TokenEstimate
	}

	result := make([]Turn, 0, len(skeleton)+len(recent))
	result = append(result, skeleton...)
	result = append(result, recent...)
	return result
}

func trimFromFront(turns []Turn, budget int) []Turn {
	total := sumTokens(turns)
	start := 0
	for total > budget && start < len(turns)-1 {
		total -= turns[start].TokenEstimate
		start++
	}
	return turns[start:]
}

func recommendStrategy(turns []Turn, totalTokens int) SelectionStrategy {
	if totalTokens <= DefaultTokenBudget {
		return SelectionStrategy{Kind: "full"}
	}

	// Try pruning
	pruned := pruneLowSignal(turns, 0)
	if sumTokens(pruned) <= DefaultTokenBudget {
		return SelectionStrategy{Kind: "prune"}
	}

	// Fall back to last N
	n := 0
	tokens := 0
	for i := len(turns) - 1; i >= 0; i-- {
		if tokens+turns[i].TokenEstimate > DefaultTokenBudget {
			break
		}
		tokens += turns[i].TokenEstimate
		n++
	}
	if n < 4 {
		n = 4
	}

	return SelectionStrategy{Kind: "last", N: n}
}

func sumTokens(turns []Turn) int {
	total := 0
	for _, t := range turns {
		total += t.TokenEstimate
	}
	return total
}
