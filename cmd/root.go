package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rethread",
	Short: "Selective replay of Claude Code conversations",
	Long: `rethread — selective replay of Claude Code conversations into new sessions.

Preserves turn structure, reasoning texture, and conversation shape.
No summarization, no flattening. RAG for selection, full replay for injection.`,
	Version: "0.1.0",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(forkCmd)
	rootCmd.AddCommand(exportCmd)
}
