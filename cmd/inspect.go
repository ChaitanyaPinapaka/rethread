package cmd

import (
	"fmt"

	"github.com/ChaitanyaPinapaka/rethread/internal"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <session-id>",
	Short: "Analyze a session's turns and recommend a replay strategy",
	Args:  cobra.ExactArgs(1),
	RunE:  runInspect,
}

func runInspect(cmd *cobra.Command, args []string) error {
	session, err := findSession(args[0])
	if err != nil {
		return err
	}

	turns, err := readTurns(session)
	if err != nil {
		return err
	}

	a := internal.AnalyzeTurns(turns)

	fmt.Printf("\n  Session: %s\n", session.ID)
	fmt.Printf("  Project: %s\n\n", session.ProjectPath)
	fmt.Printf("  Total turns:      %d\n", a.TotalTurns)
	fmt.Printf("    User:           %d\n", a.UserTurns)
	fmt.Printf("    Assistant:      %d\n", a.AssistantTurns)
	fmt.Printf("    Sidechain:      %d\n", a.SidechainTurns)
	fmt.Printf("    Low-signal:     %d\n", a.LowSignalTurns)
	fmt.Printf("  Token estimate:   ~%d\n", a.TotalTokenEstimate)

	fits := "yes"
	if !a.FitsInContext {
		fits = "no"
	}
	fmt.Printf("  Fits in context:  %s\n\n", fits)

	rec := a.RecommendedStrategy
	fmt.Println("  Recommended export strategy:")
	switch rec.Kind {
	case "full":
		fmt.Println("  - Full export (entire conversation)")
		fmt.Printf("    rethread export %s -f clean -o %s-full.jsonl\n", args[0], args[0])
	case "prune":
		fmt.Printf("  - Pruned export (removes %d low-signal turns)\n", a.LowSignalTurns)
		fmt.Printf("    rethread export %s --prune -f clean -o %s-pruned.jsonl\n", args[0], args[0])
	case "last":
		fmt.Printf("  - Partial export (last %d turns)\n", rec.N)
		fmt.Printf("    rethread export %s --turns %d -f clean -o %s-last%d.jsonl\n", args[0], rec.N, args[0], rec.N)
	}

	fmt.Println()
	return nil
}
