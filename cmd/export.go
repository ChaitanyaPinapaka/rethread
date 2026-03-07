package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
)

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "jsonl", "Output format: jsonl, clean, markdown, turns")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path (default: stdout)")
	exportCmd.Flags().IntVarP(&exportTurns, "turns", "t", 0, "Export only the last N turns")
	exportCmd.Flags().BoolVar(&exportPrune, "prune", false, "Prune low-signal turns before export")
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

	strategy := internal.SelectionStrategy{Kind: "full"}
	if exportTurns > 0 {
		strategy = internal.SelectionStrategy{Kind: "last", N: exportTurns}
	} else if exportPrune {
		strategy = internal.SelectionStrategy{Kind: "prune"}
	}

	selected := internal.SelectTurns(turns, strategy, 1<<30)

	content := internal.FormatForExport(selected, exportFormat, session.ID, session.ProjectPath)

	if exportOutput != "" {
		outPath, err := filepath.Abs(exportOutput)
		if err != nil {
			return err
		}
		if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Exported %d turns to %s\n", len(selected), outPath)
		return nil
	}

	// stdout
	fmt.Print(content)
	return nil
}
