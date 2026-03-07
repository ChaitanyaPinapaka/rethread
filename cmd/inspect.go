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
	session, err := internal.FindSession(args[0])
	if err != nil {
		return err
	}

	turns, err := internal.ReadTurns(session.FilePath, false)
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
	switch rec.Kind {
	case "full":
		fmt.Println("  Recommended: full replay (fits in context window)")
		fmt.Printf("    rethread fork %s\n", args[0])
	case "prune":
		fmt.Printf("  Recommended: prune (remove %d low-signal turns)\n", a.LowSignalTurns)
		fmt.Printf("    rethread fork %s --prune\n", args[0])
	case "last":
		fmt.Printf("  Recommended: last %d turns (conversation too long for full replay)\n", rec.N)
		fmt.Printf("    rethread fork %s --turns %d\n", args[0], rec.N)
	}

	fmt.Println()
	return nil
}
