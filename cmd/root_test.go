package cmd

import (
	"strings"
	"testing"

	"github.com/ChaitanyaPinapaka/rethread/internal"
)

func TestResolvedSources_Claude(t *testing.T) {
	old := source
	defer func() { source = old }()

	source = "claude"
	sources := resolvedSources()
	if len(sources) != 1 || sources[0] != "claude" {
		t.Errorf("expected [claude], got %v", sources)
	}
}

func TestResolvedSources_Gemini(t *testing.T) {
	old := source
	defer func() { source = old }()

	source = "gemini"
	sources := resolvedSources()
	if len(sources) != 1 || sources[0] != "gemini" {
		t.Errorf("expected [gemini], got %v", sources)
	}
}

func TestResolvedSources_Auto(t *testing.T) {
	old := source
	defer func() { source = old }()

	source = "auto"
	sources := resolvedSources()
	if len(sources) != 2 {
		t.Errorf("expected 2 sources for auto, got %v", sources)
	}
}

func TestResolvedSources_Default(t *testing.T) {
	old := source
	defer func() { source = old }()

	source = "anything"
	sources := resolvedSources()
	if len(sources) != 2 {
		t.Errorf("unknown source should default to auto (both), got %v", sources)
	}
}

func TestIsGeminiSession(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"gemini:abc123", true},
		{"gemini:xyz", true},
		{"/Users/alice/project", false},
		{"C:\\Work\\project", false},
		{"", false},
	}

	for _, tt := range tests {
		session := &internal.SessionMeta{ProjectPath: tt.path}
		got := isGeminiSession(session)
		if got != tt.want {
			t.Errorf("isGeminiSession(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestFindSession_NotFound(t *testing.T) {
	old := source
	defer func() { source = old }()

	// Use a source that won't find anything in auto mode
	source = "auto"
	_, err := findSession("nonexistent-session-id-xyz")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
	if err != nil && !strings.Contains(err.Error(), "not found") {
		// Could also be "directory not found" if Claude/Gemini aren't installed
		// That's fine — the point is it returns an error
		t.Logf("got error (expected): %v", err)
	}
}
