package cmd

import (
	"fmt"

	"github.com/ChaitanyaPinapaka/rethread/internal"
	"github.com/spf13/cobra"
)

var forkCmd = &cobra.Command{
	Use:   "fork [session-id]",
	Short: "Fork a session into a new Claude Code session",
	Long: `Fork replays selected turns from an existing session into a new one.
The model sees structured conversation turns — not a document.

Examples:
  rethread fork abc123              Full replay
  rethread fork --last              Most recent session
  rethread fork abc123 --turns 20   Last 20 turns only
  rethread fork abc123 --prune      Drop acknowledgments and filler
  rethread fork abc123 --dry-run    Preview without launching`,
	Args: cobra.MaximumNArgs(1),
	RunE: runFork,
}

var (
	forkLast           bool
	forkTurns          int
	forkPrune          bool
	forkPruneThreshold int
	forkDryRun         bool
	forkPrompt         string
	forkOutput         string
	forkFormat         string
)

func init() {
	forkCmd.Flags().BoolVar(&forkLast, "last", false, "Use the most recent session")
	forkCmd.Flags().IntVarP(&forkTurns, "turns", "t", 0, "Replay only the last N turns")
	forkCmd.Flags().BoolVar(&forkPrune, "prune", false, "Auto-prune low-signal turns")
	forkCmd.Flags().IntVar(&forkPruneThreshold, "prune-threshold", 30, "Token threshold for pruning")
	forkCmd.Flags().BoolVar(&forkDryRun, "dry-run", false, "Show what would be replayed without launching")
	forkCmd.Flags().StringVarP(&forkPrompt, "prompt", "p", "", "Initial prompt for the new session")
	forkCmd.Flags().StringVarP(&forkOutput, "output", "o", "", "Write context to file instead of launching")
	forkCmd.Flags().StringVarP(&forkFormat, "format", "f", "turns", "Output format: turns, jsonl, clean, markdown")
}

func runFork(cmd *cobra.Command, args []string) error {
	// Resolve session
	var session *internal.SessionMeta
	var err error

	if forkLast || len(args) == 0 {
		session, err = internal.GetLastSession("")
	} else {
		session, err = internal.FindSession(args[0])
	}
	if err != nil {
		return err
	}

	// Read turns
	turns, err := internal.ReadTurns(session.FilePath, false)
	if err != nil {
		return err
	}

	if len(turns) == 0 {
		return fmt.Errorf("session has no conversation turns")
	}

	// Build strategy
	strategy := internal.SelectionStrategy{Kind: "full"}
	if forkTurns > 0 {
		strategy = internal.SelectionStrategy{Kind: "last", N: forkTurns}
	} else if forkPrune {
		strategy = internal.SelectionStrategy{Kind: "prune", MinTokens: forkPruneThreshold}
	}

	// Select
	selected := internal.SelectTurns(turns, strategy, 0)

	// Build context
	totalTokens := 0
	for _, t := range selected {
		totalTokens += t.TokenEstimate
	}

	ctx := internal.ReplayContext{
		SourceSessionID:    session.ID,
		SourceProject:      session.ProjectPath,
		Turns:              selected,
		Strategy:           strategy,
		TotalTokenEstimate: totalTokens,
	}

	// Report
	pruned := len(turns) - len(selected)
	shortID := session.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	fmt.Printf("\n  Source:    %s (%s)\n", shortID, session.ProjectPath)
	fmt.Printf("  Turns:     %d of %d", len(selected), len(turns))
	if pruned > 0 {
		fmt.Printf(" (%d pruned)", pruned)
	}
	fmt.Printf("\n  Tokens:    ~%d\n", totalTokens)
	fmt.Printf("  Strategy:  %s\n\n", strategy.Kind)

	// Dry run
	if forkDryRun {
		fmt.Println("  [dry run] No session launched.")
		fmt.Println()

		limit := 3
		if len(selected) < limit {
			limit = len(selected)
		}
		for _, turn := range selected[:limit] {
			text := turn.ContentText
			if len(text) > 120 {
				text = text[:120] + "..."
			}
			fmt.Printf("  [%s] %s\n", turn.Role, text)
		}
		if len(selected) > 3 {
			fmt.Printf("  ... and %d more turns\n", len(selected)-3)
		}
		fmt.Println()
		return nil
	}

	// Write to file
	if forkOutput != "" {
		path, err := internal.WriteContextFile(ctx, forkOutput, forkFormat)
		if err != nil {
			return err
		}
		fmt.Printf("  Written to: %s\n\n", path)
		return nil
	}

	// Launch session
	result, err := internal.Inject(ctx, forkPrompt, false, "")
	if err != nil {
		return err
	}

	fmt.Printf("  Mode: %s\n", result.Mode)
	if result.ContextFile != "" {
		fmt.Printf("  Context file: %s\n", result.ContextFile)
	}
	fmt.Println()

	return nil
}
