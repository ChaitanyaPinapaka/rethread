package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ChaitanyaPinapaka/rethread/internal"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available Claude Code sessions",
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
	sessions, err := internal.ListSessions(listProject)
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

		fmt.Printf("  %s  %-18s  %3d turns  %s\n", id, date, s.TurnCount, project)

		if listVerbose {
			fmt.Printf("           %s\n", s.Preview)
			fmt.Printf("           %s\n\n", s.FilePath)
		}
	}

	fmt.Println()
	fmt.Println("  Use \"rethread fork <id>\" to replay a session into a new one.")
	fmt.Println("  Use \"rethread inspect <id>\" to analyze turns before forking.")
	fmt.Println()

	return nil
}

func shortProject(p string) string {
	parts := strings.Split(filepath.ToSlash(p), "/")
	if len(parts) <= 2 {
		return p
	}
	return strings.Join(parts[len(parts)-2:], "/")
}
