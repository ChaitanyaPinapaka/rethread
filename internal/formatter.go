package internal

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FormatForInjection builds the full injection prompt with structured turns.
func FormatForInjection(ctx ReplayContext) string {
	var b strings.Builder

	b.WriteString(injectionHeader(ctx))
	b.WriteString("\n\n")

	for i, turn := range ctx.Turns {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(formatTurnStructured(turn))
	}

	b.WriteString("\n\n</prior_conversation>")

	return b.String()
}

// FormatForExport formats turns in the requested output format.
func FormatForExport(turns []Turn, format string, sessionID, projectPath string) string {
	switch format {
	case "jsonl":
		return formatJSONL(turns)
	case "clean":
		return formatCleanJSONL(turns)
	case "markdown":
		return formatMarkdown(turns, sessionID, projectPath)
	default:
		return formatStructuredTurns(turns, sessionID, projectPath)
	}
}

// BuildInjectionPrompt wraps the replay context with an optional new prompt.
func BuildInjectionPrompt(ctx ReplayContext, newPrompt string) string {
	formatted := FormatForInjection(ctx)

	if newPrompt == "" {
		return formatted + "\n\nContinue from this conversation context. What would you like to work on next?"
	}

	return formatted + "\n\n" + newPrompt
}

// --- structured turns (injection format) ---

func injectionHeader(ctx ReplayContext) string {
	return fmt.Sprintf(`<prior_conversation>
<!-- This is a continuation from a previous session (%s).
     Project: %s
     Turns: %d (%s)
     The conversation below is your prior context. Treat it as history -->`,
		ctx.SourceSessionID,
		ctx.SourceProject,
		len(ctx.Turns),
		describeStrategy(ctx.Strategy),
	)
}

func formatTurnStructured(turn Turn) string {
	ts := ""
	if turn.Timestamp != "" {
		ts = fmt.Sprintf(` timestamp="%s"`, turn.Timestamp)
	}
	return fmt.Sprintf("<turn role=\"%s\"%s>\n%s\n</turn>", turn.Role, ts, turn.ContentText)
}

func formatStructuredTurns(turns []Turn, sessionID, projectPath string) string {
	var b strings.Builder

	if sessionID != "" {
		b.WriteString(fmt.Sprintf("<!-- rethread export | session: %s | project: %s -->\n\n", sessionID, projectPath))
	}

	b.WriteString("<conversation>\n\n")
	for i, turn := range turns {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(formatTurnStructured(turn))
	}
	b.WriteString("\n\n</conversation>")

	return b.String()
}

// --- JSONL (native Claude Code format) ---

func formatJSONL(turns []Turn) string {
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
			continue
		}
		lines = append(lines, string(data))
	}

	return strings.Join(lines, "\n")
}

// --- clean JSONL (text + thinking + tool_use, no tool_result/signatures) ---

func formatCleanJSONL(turns []Turn) string {
	var lines []string

	for _, turn := range turns {
		cleaned := cleanContent(turn.Content)
		if cleaned == nil {
			continue
		}

		obj := map[string]interface{}{
			"role":    turn.Role,
			"content": cleaned,
		}

		data, err := json.Marshal(obj)
		if err != nil {
			continue
		}
		lines = append(lines, string(data))
	}

	return strings.Join(lines, "\n")
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
			cleaned := map[string]interface{}{
				"type":  "tool_use",
				"name":  m["name"],
				"input": m["input"],
			}
			if id, ok := m["id"]; ok {
				cleaned["id"] = id
			}
			kept = append(kept, cleaned)
		// tool_result — skip entirely
		}
	}

	if len(kept) == 0 {
		return nil
	}
	return kept
}

// --- markdown (human-readable) ---

func formatMarkdown(turns []Turn, sessionID, projectPath string) string {
	var b strings.Builder

	if sessionID != "" {
		b.WriteString("# Conversation Export\n\n")
		b.WriteString(fmt.Sprintf("- **Session:** `%s`\n", sessionID))
		b.WriteString(fmt.Sprintf("- **Project:** `%s`\n", projectPath))
		b.WriteString(fmt.Sprintf("- **Turns:** %d\n", len(turns)))
		b.WriteString("- **Exported by:** rethread\n\n---\n\n")
	}

	for _, turn := range turns {
		role := "👤 User"
		if turn.Role == "assistant" {
			role = "🤖 Assistant"
		}

		ts := ""
		if turn.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, turn.Timestamp); err == nil {
				ts = fmt.Sprintf(" _%s_", t.Format("Jan 2, 2006 3:04 PM"))
			}
		}

		b.WriteString(fmt.Sprintf("### %s%s\n\n", role, ts))
		b.WriteString(turn.ContentText)
		b.WriteString("\n\n")
	}

	return b.String()
}

// --- helpers ---

func describeStrategy(s SelectionStrategy) string {
	switch s.Kind {
	case "full":
		return "full replay"
	case "last":
		return fmt.Sprintf("last %d turns", s.N)
	case "prune":
		return "pruned (low-signal turns removed)"
	case "range":
		return fmt.Sprintf("turns %d–%d", s.From, s.To)
	default:
		return s.Kind
	}
}
