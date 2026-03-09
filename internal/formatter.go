package internal

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExportStats tracks items dropped or skipped during formatting.
type ExportStats struct {
	TurnsInput    int // total turns passed in
	TurnsOutput   int // turns that made it to output
	TurnsDropped  int // turns dropped (empty after cleaning, marshal errors)
	MarshalErrors int // turns skipped due to JSON marshal failures
}

// FormatForExport formats turns in the requested output format.
func FormatForExport(turns []Turn, format string, sessionID, projectPath string) (string, ExportStats) {
	_ = sessionID
	_ = projectPath
	switch format {
	case "jsonl":
		return formatJSONL(turns)
	case "clean":
		return formatCleanJSONL(turns)
	default:
		return formatJSONL(turns)
	}
}

// --- JSONL (native Claude Code format) ---

func formatJSONL(turns []Turn) (string, ExportStats) {
	stats := ExportStats{TurnsInput: len(turns)}
	var lines []string

	for _, turn := range turns {
		obj := map[string]interface{}{
			"type": turn.Role,
			"message": map[string]interface{}{
				"role":    turn.Role,
				"content": turn.Content,
			},
			"timestamp":   turn.Timestamp,
			"uuid":        turn.UUID,
			"parentUuid":  turn.ParentUUID,
			"isSidechain": turn.IsSidechain,
		}

		data, err := json.Marshal(obj)
		if err != nil {
			stats.MarshalErrors++
			continue
		}
		lines = append(lines, string(data))
	}

	stats.TurnsOutput = len(lines)
	stats.TurnsDropped = stats.TurnsInput - stats.TurnsOutput
	return strings.Join(lines, "\n"), stats
}

// --- clean JSONL (text + thinking + tool_use, no tool_result/signatures) ---

func formatCleanJSONL(turns []Turn) (string, ExportStats) {
	stats := ExportStats{TurnsInput: len(turns)}
	var lines []string

	for _, turn := range turns {
		cleaned := cleanContent(turn.Content)
		if cleaned == nil {
			stats.TurnsDropped++
			continue
		}

		obj := map[string]interface{}{
			"role":    turn.Role,
			"content": cleaned,
		}

		data, err := json.Marshal(obj)
		if err != nil {
			stats.MarshalErrors++
			stats.TurnsDropped++
			continue
		}
		lines = append(lines, string(data))
	}

	stats.TurnsOutput = len(lines)
	return strings.Join(lines, "\n"), stats
}

func cleanContent(content interface{}) interface{} {
	blocks, ok := content.([]interface{})
	if !ok {
		// plain string content — keep as-is
		return content
	}

	var kept []interface{}
	for _, block := range blocks {
		m, ok := block.(map[string]interface{})
		if !ok {
			continue
		}

		blockType, _ := m["type"].(string)
		switch blockType {
		case "text":
			kept = append(kept, map[string]interface{}{
				"type": "text",
				"text": m["text"],
			})
		case "thinking":
			// Keep thinking text, drop signature
			kept = append(kept, map[string]interface{}{
				"type":     "thinking",
				"thinking": m["thinking"],
			})
		case "tool_use":
			toolName, _ := m["name"].(string)
			kept = append(kept, map[string]interface{}{
				"type":  "tool_use",
				"name":  toolName,
				"input": condenseToolInput(toolName, m["input"]),
			})
			// tool_result — skip entirely
		}
	}

	if len(kept) == 0 {
		return nil
	}
	return kept
}

// condenseToolInput strips heavy content from tool inputs, keeping just
// enough to understand what action was taken without the full payload.
func condenseToolInput(toolName string, input interface{}) interface{} {
	m, ok := input.(map[string]interface{})
	if !ok {
		return input
	}

	switch toolName {
	case "Edit":
		condensed := map[string]interface{}{
			"file_path": m["file_path"],
		}
		if old, ok := m["old_string"].(string); ok {
			condensed["old_string"] = TruncateField(old, 80)
		}
		if nw, ok := m["new_string"].(string); ok {
			condensed["new_string"] = TruncateField(nw, 80)
		}
		if ra, ok := m["replace_all"]; ok {
			condensed["replace_all"] = ra
		}
		return condensed

	case "Write", "NotebookEdit":
		condensed := map[string]interface{}{
			"file_path": m["file_path"],
		}
		if content, ok := m["content"].(string); ok {
			lines := strings.Count(content, "\n") + 1
			condensed["content"] = fmt.Sprintf("[%d lines]", lines)
		}
		return condensed

	case "Read":
		condensed := map[string]interface{}{
			"file_path": m["file_path"],
		}
		if v, ok := m["offset"]; ok {
			condensed["offset"] = v
		}
		if v, ok := m["limit"]; ok {
			condensed["limit"] = v
		}
		return condensed

	case "Bash":
		condensed := map[string]interface{}{}
		if cmd, ok := m["command"].(string); ok {
			condensed["command"] = TruncateField(cmd, 200)
		}
		if desc, ok := m["description"].(string); ok {
			condensed["description"] = desc
		}
		return condensed

	case "Grep":
		condensed := map[string]interface{}{
			"pattern": m["pattern"],
		}
		if v, ok := m["path"]; ok {
			condensed["path"] = v
		}
		if v, ok := m["glob"]; ok {
			condensed["glob"] = v
		}
		return condensed

	case "Glob":
		condensed := map[string]interface{}{
			"pattern": m["pattern"],
		}
		if v, ok := m["path"]; ok {
			condensed["path"] = v
		}
		return condensed

	default:
		// Unknown tool — truncate any string values over 200 chars
		condensed := make(map[string]interface{}, len(m))
		for k, v := range m {
			if s, ok := v.(string); ok && len(s) > 200 {
				condensed[k] = TruncateField(s, 200)
			} else {
				condensed[k] = v
			}
		}
		return condensed
	}
}
