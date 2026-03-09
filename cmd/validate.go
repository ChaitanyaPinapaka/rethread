package cmd

import (
	"fmt"
	"os"

	"github.com/ChaitanyaPinapaka/rethread/internal"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate <session-id>",
	Short: "Measure compact mode's token reduction and quality preservation",
	Args:  cobra.ExactArgs(1),
	RunE:  runValidate,
}

func runValidate(cmd *cobra.Command, args []string) error {
	session, err := findSession(args[0])
	if err != nil {
		return err
	}

	turns, err := readTurns(session)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\n  Session: %s\n", session.ID)
	fmt.Fprintf(os.Stderr, "  Project: %s\n", session.ProjectPath)
	fmt.Fprintf(os.Stderr, "  Turns:   %d\n\n", len(turns))

	report := internal.Validate(turns, session.ID, session.ProjectPath)
	fmt.Print(internal.FormatReport(report))

	if !report.Passed {
		os.Exit(1)
	}
	return nil
}
