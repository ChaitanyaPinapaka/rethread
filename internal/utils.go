package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HomeDir returns the user's home directory with cross-platform fallbacks.
func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		if home == "" {
			home = filepath.Join(os.Getenv("HOMEDRIVE"), os.Getenv("HOMEPATH"))
		}
	}
	return home
}

// ExtractTextBlocks returns all text values from a content field,
// whether it's a plain string or an array of content blocks.
func ExtractTextBlocks(content interface{}) []string {
	switch v := content.(type) {
	case string:
		return []string{v}
	case nil:
		return nil
	case []interface{}:
		var texts []string
		for _, block := range v {
			m, ok := block.(map[string]interface{})
			if !ok {
				continue
			}
			if m["type"] == "text" {
				if text, ok := m["text"].(string); ok {
					texts = append(texts, text)
				}
			}
		}
		return texts
	default:
		return []string{fmt.Sprintf("%v", content)}
	}
}

// ExtractText pulls plain text from a message content field.
func ExtractText(content interface{}) string {
	return strings.Join(ExtractTextBlocks(content), "\n")
}

// TruncatePreview returns the first maxLen characters of s.
func TruncatePreview(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ParseJSONLLines splits a JSONL string and unmarshals each non-empty line.
// Returns parsed objects and a count of lines that failed to parse.
func ParseJSONLLines(jsonl string) ([]map[string]interface{}, int) {
	lines := strings.Split(strings.TrimSpace(jsonl), "\n")
	var objs []map[string]interface{}
	parseErrors := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			parseErrors++
			continue
		}
		objs = append(objs, obj)
	}
	return objs, parseErrors
}

// ContentBlocks extracts typed block maps from a content field.
// Returns nil if content is a plain string.
func ContentBlocks(content interface{}) []map[string]interface{} {
	blocks, ok := content.([]interface{})
	if !ok {
		return nil
	}
	var result []map[string]interface{}
	for _, block := range blocks {
		if m, ok := block.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

// TruncateField returns the first maxLen characters of s, appending a
// "[...N more chars]" marker if truncation occurred.
func TruncateField(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf(" [...%d more chars]", len(s)-maxLen)
}
