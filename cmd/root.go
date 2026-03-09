package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/ChaitanyaPinapaka/rethread/internal"
	"github.com/spf13/cobra"
)

var source string
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "rethread",
	Short: "Selective replay of AI CLI conversations",
	Long: `rethread — selective replay of AI CLI conversations into new sessions.

Supports Claude Code and Gemini CLI.
Preserves turn structure, reasoning texture, and conversation shape.
No summarization, no flattening. Select, prune, and export.`,
	Version: version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&source, "source", "auto", "Session source: claude, gemini, auto")
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(validateCmd)
}

// resolvedSource returns the effective source(s) to query.
func resolvedSources() []string {
	switch strings.ToLower(source) {
	case "claude":
		return []string{"claude"}
	case "gemini":
		return []string{"gemini"}
	default:
		return []string{"claude", "gemini"}
	}
}

// listAllSessions lists sessions from all resolved sources.
func listAllSessions(projectFilter string) ([]internal.SessionMeta, error) {
	sources := resolvedSources()
	var all []internal.SessionMeta

	for _, src := range sources {
		var sessions []internal.SessionMeta
		var err error
		switch src {
		case "claude":
			sessions, err = internal.ListSessions(projectFilter)
		case "gemini":
			sessions, err = internal.ListGeminiSessions(projectFilter)
		}
		if err != nil {
			// If auto-detecting, skip sources that aren't installed
			if source == "auto" {
				continue
			}
			return nil, err
		}
		all = append(all, sessions...)
	}

	return all, nil
}

// findSession finds a session by ID across all resolved sources.
func findSession(sessionID string) (*internal.SessionMeta, error) {
	sessions, err := listAllSessions("")
	if err != nil {
		return nil, err
	}

	// Exact match
	for i := range sessions {
		if sessions[i].ID == sessionID {
			return &sessions[i], nil
		}
	}

	// Prefix match
	var matches []internal.SessionMeta
	for _, s := range sessions {
		if strings.HasPrefix(s.ID, sessionID) {
			matches = append(matches, s)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("session not found: %q", sessionID)
	case 1:
		return &matches[0], nil
	default:
		lines := make([]string, len(matches))
		for i, m := range matches {
			lines[i] = fmt.Sprintf("  %s (%s)", m.ID, m.ProjectPath)
		}
		return nil, fmt.Errorf("ambiguous session ID %q. Matches:\n%s", sessionID, strings.Join(lines, "\n"))
	}
}

// readTurns reads turns from a session, auto-detecting the source.
func readTurns(session *internal.SessionMeta) ([]internal.Turn, error) {
	if isGeminiSession(session) {
		return internal.ReadGeminiTurns(session.FilePath, false)
	}
	return internal.ReadTurns(session.FilePath, false)
}

func isGeminiSession(session *internal.SessionMeta) bool {
	return strings.HasPrefix(session.ProjectPath, "gemini:")
}
