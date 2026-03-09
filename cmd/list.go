package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available AI CLI sessions",
	RunE:  runList,
}

var (
	listProject string
	listLimit   int
	listVerbose bool
)

func init() {
	listCmd.Flags().StringVarP(&listProject, "project", "p", "", "Filter by project path (substring match)")
	listCmd.Flags().IntVarP(&listLimit, "limit", "n", 15, "Max sessions to show")
	listCmd.Flags().BoolVarP(&listVerbose, "verbose", "v", false, "Show additional details")
}

func runList(cmd *cobra.Command, args []string) error {
	sessions, err := listAllSessions(listProject)
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		if listProject != "" {
			fmt.Printf("  (filtered by project: %q)\n", listProject)
		}
		return nil
	}

	// Sort combined results by most recent first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastTimestamp.After(sessions[j].LastTimestamp)
	})

	display := sessions
	if listLimit > 0 && listLimit < len(display) {
		display = display[:listLimit]
	}

	fmt.Printf("\n  Sessions (%d of %d):\n\n", len(display), len(sessions))

	for _, s := range display {
		date := s.LastTimestamp.Format("Jan 02 15:04")
		project := shortProject(s.ProjectPath)
		id := s.ID
		if len(id) > 8 {
			id = id[:8]
		}

		src := "claude"
		if isGeminiSession(&s) {
			src = "gemini"
		}

		fmt.Printf("  %s  %-18s  %3d turns  %-8s %s\n", id, date, s.TurnCount, src, project)

		if listVerbose {
			fmt.Printf("           %s\n", s.Preview)
			fmt.Printf("           %s\n\n", s.FilePath)
		}
	}

	fmt.Println()
	fmt.Println("  Use \"rethread inspect <id>\" to analyze a session's turns.")
	fmt.Println("  Use \"rethread export <id>\" to export a session to a file.")
	fmt.Println()

	return nil
}

func shortProject(p string) string {
	normalized := filepath.ToSlash(p)
	parts := strings.Split(normalized, "/")
	if len(parts) <= 2 {
		return normalized
	}
	return strings.Join(parts[len(parts)-2:], "/")
}
