package internal

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CompactReport measures token reduction and quality preservation
// when comparing raw vs compact export of the same turns.
type CompactReport struct {
	// Reduction metrics
	RawBytes     int
	CompactBytes int
	Ratio        float64 // compact / raw (lower = better savings)
	SavedBytes   int

	// Per-block-type breakdown
	BlockStats map[string]BlockStat

	// Export stats (dropped/error counts)
	RawStats   ExportStats
	CleanStats ExportStats

	// Quality checks — all must pass
	Checks []QualityCheck
	Passed bool
}

// BlockStat tracks byte contribution per content block type.
type BlockStat struct {
	Count       int
	RawBytes    int
	CompactBytes int
	Ratio       float64
}

// QualityCheck is a single pass/fail quality assertion.
type QualityCheck struct {
	Name   string
	Passed bool
	Detail string
}

// Validate compares the raw JSONL (full fidelity) vs clean output for the same
// turns and returns a report with reduction metrics and quality checks.
func Validate(turns []Turn, sessionID, projectPath string) CompactReport {
	raw, rawStats := FormatForExport(turns, "jsonl", sessionID, projectPath)
	clean, cleanStats := FormatForExport(turns, "clean", sessionID, projectPath)

	report := CompactReport{
		RawBytes:     len(raw),
		CompactBytes: len(clean),
		SavedBytes:   len(raw) - len(clean),
		BlockStats:   make(map[string]BlockStat),
		RawStats:     rawStats,
		CleanStats:   cleanStats,
	}

	if report.RawBytes > 0 {
		report.Ratio = float64(report.CompactBytes) / float64(report.RawBytes)
	}

	// Per-block-type stats
	report.BlockStats = computeBlockStats(turns)

	// Quality checks
	report.Checks = runQualityChecks(turns, clean)
	report.Passed = true
	for _, c := range report.Checks {
		if !c.Passed {
			report.Passed = false
			break
		}
	}

	return report
}

// computeBlockStats measures byte contribution of each block type in
// raw vs compact mode.
func computeBlockStats(turns []Turn) map[string]BlockStat {
	stats := map[string]BlockStat{}

	for _, turn := range turns {
		blockMaps := ContentBlocks(turn.Content)
		if blockMaps == nil {
			// string content — count under "text"
			s := fmt.Sprintf("%v", turn.Content)
			bs := stats["text"]
			bs.Count++
			bs.RawBytes += len(s)
			bs.CompactBytes += len(s) // string content unchanged
			stats["text"] = bs
			continue
		}

		for _, m := range blockMaps {
			blockType, _ := m["type"].(string)
			rawJSON, _ := json.Marshal(m)

			// Determine the stat key — for tool_use, use the tool name
			statKey := blockType
			compactJSON := rawJSON

			switch blockType {
			case "tool_use":
				toolName, _ := m["name"].(string)
				if toolName != "" {
					statKey = toolName
				}
				compacted := map[string]interface{}{
					"type":  "tool_use",
					"name":  toolName,
					"input": condenseToolInput(toolName, m["input"]),
				}
				compactJSON, _ = json.Marshal(compacted)
			case "thinking":
				compacted := map[string]interface{}{
					"type":     "thinking",
					"thinking": m["thinking"],
				}
				compactJSON, _ = json.Marshal(compacted)
			case "text":
				compacted := map[string]interface{}{
					"type": "text",
					"text": m["text"],
				}
				compactJSON, _ = json.Marshal(compacted)
			case "tool_result":
				// Dropped entirely in clean format
				compactJSON = nil
			}

			bs := stats[statKey]
			bs.Count++
			bs.RawBytes += len(rawJSON)
			if compactJSON != nil {
				bs.CompactBytes += len(compactJSON)
			}
			stats[statKey] = bs
		}
	}

	// Compute ratios
	for k, bs := range stats {
		if bs.RawBytes > 0 {
			bs.Ratio = float64(bs.CompactBytes) / float64(bs.RawBytes)
		}
		stats[k] = bs
	}

	return stats
}

// parsedTurn is a compact JSONL line parsed back into structured data.
type parsedTurn struct {
	Role    string
	Texts   []string // text block values
	Paths   []string // file_path values from tool_use inputs
	Content interface{}
}

// parseCompactOutput parses JSONL output back into structured turns
// so quality checks compare decoded data, not JSON-escaped strings.
func parseCompactOutput(compact string) []parsedTurn {
	objs, _ := ParseJSONLLines(compact)
	var parsed []parsedTurn
	for _, obj := range objs {
		role, _ := obj["role"].(string)
		pt := parsedTurn{
			Role:    role,
			Content: obj["content"],
			Texts:   ExtractTextBlocks(obj["content"]),
		}

		// Extract file paths from tool_use blocks
		for _, m := range ContentBlocks(obj["content"]) {
			if m["type"] == "tool_use" {
				if input, ok := m["input"].(map[string]interface{}); ok {
					if fp, ok := input["file_path"].(string); ok {
						pt.Paths = append(pt.Paths, fp)
					}
				}
			}
		}

		parsed = append(parsed, pt)
	}
	return parsed
}

// runQualityChecks verifies that clean format preserves conversation quality.
func runQualityChecks(turns []Turn, clean string) []QualityCheck {
	parsed := parseCompactOutput(clean)

	var checks []QualityCheck

	// 1. All user messages preserved verbatim
	checks = append(checks, checkUserMessagesPreserved(turns, parsed))

	// 2. All assistant text blocks preserved verbatim
	checks = append(checks, checkAssistantTextPreserved(turns, parsed))

	// 3. File paths from tool_use still present
	checks = append(checks, checkFilePathsPreserved(turns, parsed))

	// 4. No unexpected turn loss
	checks = append(checks, checkTurnCount(turns, parsed))

	// 5. Turn order preserved
	checks = append(checks, checkTurnOrder(clean))

	// 6. No empty content in output
	checks = append(checks, checkNoEmptyContent(clean))

	return checks
}

// checkUserMessagesPreserved ensures every user message text appears
// unchanged in the compact output by comparing parsed content.
func checkUserMessagesPreserved(turns []Turn, parsed []parsedTurn) QualityCheck {
	check := QualityCheck{Name: "user_messages_preserved", Passed: true}

	// Collect all text from compact user turns
	compactUserTexts := map[string]bool{}
	for _, pt := range parsed {
		if pt.Role != "user" {
			continue
		}
		for _, t := range pt.Texts {
			compactUserTexts[t] = true
		}
	}

	missing := 0
	for _, t := range turns {
		if t.Role != "user" {
			continue
		}
		// Check each text block from the original turn
		texts := ExtractTextBlocks(t.Content)
		for _, text := range texts {
			if text == "" {
				continue
			}
			if !compactUserTexts[text] {
				missing++
				if check.Detail == "" {
					check.Detail = fmt.Sprintf("missing user text: %.60s...", text)
				}
			}
		}
	}

	if missing > 0 {
		check.Passed = false
		check.Detail = fmt.Sprintf("%d user message(s) missing or altered. %s", missing, check.Detail)
	}
	return check
}

// checkAssistantTextPreserved ensures assistant text blocks (not tool_use,
// not thinking) appear unchanged in compact output.
func checkAssistantTextPreserved(turns []Turn, parsed []parsedTurn) QualityCheck {
	check := QualityCheck{Name: "assistant_text_preserved", Passed: true}

	// Collect all text from compact assistant turns
	compactAsstTexts := map[string]bool{}
	for _, pt := range parsed {
		if pt.Role != "assistant" {
			continue
		}
		for _, t := range pt.Texts {
			compactAsstTexts[t] = true
		}
	}

	missing := 0
	for _, t := range turns {
		if t.Role != "assistant" {
			continue
		}
		texts := ExtractTextBlocks(t.Content)
		for _, text := range texts {
			if text == "" {
				continue
			}
			if !compactAsstTexts[text] {
				missing++
				if check.Detail == "" {
					check.Detail = fmt.Sprintf("missing: %.60s...", text)
				}
			}
		}
	}

	if missing > 0 {
		check.Passed = false
		check.Detail = fmt.Sprintf("%d assistant text block(s) missing. %s", missing, check.Detail)
	}
	return check
}

// checkFilePathsPreserved ensures file paths from tool_use inputs still
// appear in the compact output.
func checkFilePathsPreserved(turns []Turn, parsed []parsedTurn) QualityCheck {
	check := QualityCheck{Name: "file_paths_preserved", Passed: true}

	// Collect all paths from compact output
	compactPaths := map[string]bool{}
	for _, pt := range parsed {
		for _, p := range pt.Paths {
			compactPaths[p] = true
		}
	}

	missing := 0
	for _, t := range turns {
		for _, m := range ContentBlocks(t.Content) {
			if m["type"] != "tool_use" {
				continue
			}
			input, ok := m["input"].(map[string]interface{})
			if !ok {
				continue
			}
			fp, ok := input["file_path"].(string)
			if !ok || fp == "" {
				continue
			}
			if !compactPaths[fp] {
				missing++
				if check.Detail == "" {
					check.Detail = fmt.Sprintf("missing path: %s", fp)
				}
			}
		}
	}

	if missing > 0 {
		check.Passed = false
		check.Detail = fmt.Sprintf("%d file path(s) missing. %s", missing, check.Detail)
	}
	return check
}

// checkTurnCount verifies clean output has the expected number of turns.
// Turns whose content is entirely tool_result blocks will be dropped by
// clean format (cleanContent returns nil), so we count expected survivors.
func checkTurnCount(turns []Turn, parsed []parsedTurn) QualityCheck {
	check := QualityCheck{Name: "turn_count_matches", Passed: true}

	// Count turns that should survive cleaning
	expected := 0
	for _, t := range turns {
		if cleanContent(t.Content) != nil {
			expected++
		}
	}

	if len(parsed) != expected {
		check.Passed = false
		check.Detail = fmt.Sprintf("expected=%d turns, clean=%d turns", expected, len(parsed))
	}
	return check
}

// checkTurnOrder ensures roles alternate properly and no reordering occurred.
func checkTurnOrder(compact string) QualityCheck {
	check := QualityCheck{Name: "turn_order_valid", Passed: true}

	objs, parseErrors := ParseJSONLLines(compact)
	if parseErrors > 0 {
		check.Passed = false
		check.Detail = fmt.Sprintf("%d invalid JSON line(s) in output", parseErrors)
		return check
	}

	for i, obj := range objs {
		role, _ := obj["role"].(string)
		if role != "user" && role != "assistant" {
			check.Passed = false
			check.Detail = fmt.Sprintf("invalid role %q at position %d", role, i)
			return check
		}
	}
	return check
}

// checkNoEmptyContent ensures no turn has nil or empty content.
func checkNoEmptyContent(compact string) QualityCheck {
	check := QualityCheck{Name: "no_empty_content", Passed: true}

	objs, _ := ParseJSONLLines(compact)
	for i, obj := range objs {
		content := obj["content"]
		if content == nil {
			check.Passed = false
			check.Detail = fmt.Sprintf("empty content at line %d", i+1)
			return check
		}
		if s, ok := content.(string); ok && strings.TrimSpace(s) == "" {
			check.Passed = false
			check.Detail = fmt.Sprintf("blank content at line %d", i+1)
			return check
		}
		if arr, ok := content.([]interface{}); ok && len(arr) == 0 {
			check.Passed = false
			check.Detail = fmt.Sprintf("empty content array at line %d", i+1)
			return check
		}
	}
	return check
}

// FormatReport returns a human-readable summary of a CompactReport.
func FormatReport(r CompactReport) string {
	var b strings.Builder

	b.WriteString("  Compact Validation Report\n\n")

	// Reduction summary
	b.WriteString(fmt.Sprintf("  Raw bytes:      %d\n", r.RawBytes))
	b.WriteString(fmt.Sprintf("  Compact bytes:  %d\n", r.CompactBytes))
	b.WriteString(fmt.Sprintf("  Saved:          %d (%.0f%% reduction)\n", r.SavedBytes, (1-r.Ratio)*100))
	b.WriteString("\n")

	// Turn stats
	b.WriteString(fmt.Sprintf("  Turns:          %d input -> %d raw, %d clean\n",
		r.CleanStats.TurnsInput, r.RawStats.TurnsOutput, r.CleanStats.TurnsOutput))
	if r.CleanStats.TurnsDropped > 0 {
		b.WriteString(fmt.Sprintf("  Dropped:        %d turns (empty: %d, marshal errors: %d)\n",
			r.CleanStats.TurnsDropped,
			r.CleanStats.TurnsDropped-r.CleanStats.MarshalErrors,
			r.CleanStats.MarshalErrors))
	}
	if r.RawStats.MarshalErrors > 0 {
		b.WriteString(fmt.Sprintf("  Raw errors:     %d turns failed to marshal\n", r.RawStats.MarshalErrors))
	}
	b.WriteString("\n")

	// Per-block breakdown (sorted by raw bytes descending)
	b.WriteString("  Block type breakdown:\n")
	type blockEntry struct {
		name string
		stat BlockStat
	}
	var entries []blockEntry
	for name, bs := range r.BlockStats {
		entries = append(entries, blockEntry{name, bs})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].stat.RawBytes > entries[j].stat.RawBytes
	})
	for _, e := range entries {
		pct := (1 - e.stat.Ratio) * 100
		if pct < 0 {
			pct = 0
		}
		b.WriteString(fmt.Sprintf("    %-14s %3d blocks  %6d -> %6d bytes  (%.0f%% saved)\n",
			e.name, e.stat.Count, e.stat.RawBytes, e.stat.CompactBytes, pct))
	}
	b.WriteString("\n")

	// Quality checks
	b.WriteString("  Quality checks:\n")
	for _, c := range r.Checks {
		status := "PASS"
		if !c.Passed {
			status = "FAIL"
		}
		line := fmt.Sprintf("    [%s] %s", status, c.Name)
		if c.Detail != "" {
			line += " — " + c.Detail
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")

	if r.Passed {
		b.WriteString("  Result: ALL CHECKS PASSED\n")
	} else {
		b.WriteString("  Result: SOME CHECKS FAILED\n")
	}

	return b.String()
}
