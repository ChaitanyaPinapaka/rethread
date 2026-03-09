package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ChaitanyaPinapaka/rethread/internal"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export <session-id>",
	Short: "Export a session to a file",
	Args:  cobra.ExactArgs(1),
	RunE:  runExport,
}

var (
	exportFormat string
	exportOutput string
	exportTurns  int
	exportPrune  bool
	exportRange  string
)

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "jsonl", "Output format: jsonl, clean")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path (default: stdout)")
	exportCmd.Flags().IntVarP(&exportTurns, "turns", "t", 0, "Export only the last N turns")
	exportCmd.Flags().BoolVar(&exportPrune, "prune", false, "Prune low-signal turns before export")
	exportCmd.Flags().StringVar(&exportRange, "range", "", "Export a range of turns (e.g. 10-20)")
}

func runExport(cmd *cobra.Command, args []string) error {
	session, err := findSession(args[0])
	if err != nil {
		return err
	}

	turns, err := readTurns(session)
	if err != nil {
		return err
	}

	if exportFormat != "jsonl" && exportFormat != "clean" {
		return fmt.Errorf("invalid format %q: supported formats are jsonl, clean", exportFormat)
	}

	strategy := internal.SelectionStrategy{Kind: "full"}
	if exportRange != "" {
		from, to, err := parseRange(exportRange)
		if err != nil {
			return err
		}
		strategy = internal.SelectionStrategy{Kind: "range", From: from, To: to}
	} else if exportTurns > 0 {
		strategy = internal.SelectionStrategy{Kind: "last", N: exportTurns}
	} else if exportPrune {
		strategy = internal.SelectionStrategy{Kind: "prune"}
	}

	selected := internal.SelectTurns(turns, strategy, 1<<30)

	content, stats := internal.FormatForExport(selected, exportFormat, session.ID, session.ProjectPath)

	if exportOutput != "" {
		outPath, err := filepath.Abs(exportOutput)
		if err != nil {
			return err
		}
		if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Exported %d turns to %s\n", stats.TurnsOutput, outPath)
		if stats.TurnsDropped > 0 {
			fmt.Fprintf(os.Stderr, "  Dropped %d turns (empty after cleaning: %d, marshal errors: %d)\n",
				stats.TurnsDropped, stats.TurnsDropped-stats.MarshalErrors, stats.MarshalErrors)
		}
		return nil
	}

	// stdout — print stats to stderr so they don't mix with output
	if stats.TurnsDropped > 0 {
		fmt.Fprintf(os.Stderr, "Dropped %d turns (empty after cleaning: %d, marshal errors: %d)\n",
			stats.TurnsDropped, stats.TurnsDropped-stats.MarshalErrors, stats.MarshalErrors)
	}
	fmt.Print(content)
	return nil
}

// parseRange parses a "FROM-TO" range string (0-indexed, inclusive).
func parseRange(s string) (int, int, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range %q: expected FROM-TO (e.g. 10-20)", s)
	}
	from, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range start %q: %w", parts[0], err)
	}
	to, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range end %q: %w", parts[1], err)
	}
	if from > to {
		return 0, 0, fmt.Errorf("invalid range: start (%d) > end (%d)", from, to)
	}
	return from, to, nil
}
