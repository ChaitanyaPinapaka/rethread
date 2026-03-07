package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Inject replays context into a new Claude Code session.
func Inject(ctx ReplayContext, prompt string, dryRun bool, cwd string) (InjectResult, error) {
	tokenEst := ctx.TotalTokenEstimate

	if dryRun {
		return InjectResult{
			Mode:          "dry-run",
			TokenEstimate: tokenEst,
			TurnCount:     len(ctx.Turns),
		}, nil
	}

	// Decide mode: native fork for full replay, context injection otherwise
	if ctx.Strategy.Kind == "full" && claudeAvailable() {
		return nativeFork(ctx, prompt, cwd)
	}

	return contextInjection(ctx, prompt, cwd)
}

// WriteContextFile writes the replay context to a file.
func WriteContextFile(ctx ReplayContext, outputPath, format string) (string, error) {
	content := FormatForExport(ctx.Turns, format, ctx.SourceSessionID, ctx.SourceProject)

	resolved, err := filepath.Abs(outputPath)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing context file: %w", err)
	}

	return resolved, nil
}

// --- native fork ---

func nativeFork(ctx ReplayContext, prompt, cwd string) (InjectResult, error) {
	args := []string{"--resume", ctx.SourceSessionID}
	if prompt != "" {
		args = append(args, "-p", prompt)
	}

	cmd := exec.Command("claude", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if cwd != "" {
		cmd.Dir = cwd
	}

	err := cmd.Run()
	if err != nil {
		// Fall back to context injection
		return contextInjection(ctx, prompt, cwd)
	}

	return InjectResult{
		Mode:          "native-fork",
		SessionID:     ctx.SourceSessionID,
		TokenEstimate: ctx.TotalTokenEstimate,
		TurnCount:     len(ctx.Turns),
	}, nil
}

// --- context injection ---

func contextInjection(ctx ReplayContext, prompt, cwd string) (InjectResult, error) {
	// Write context to temp file
	idPrefix := ctx.SourceSessionID
	if len(idPrefix) > 8 {
		idPrefix = idPrefix[:8]
	}
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("rethread-%s.txt", idPrefix))

	promptText := BuildInjectionPrompt(ctx, prompt)
	if err := os.WriteFile(tmpFile, []byte(promptText), 0644); err != nil {
		return InjectResult{}, fmt.Errorf("writing temp context: %w", err)
	}

	if claudeAvailable() {
		cmd := exec.Command("claude", "-p", promptText)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if cwd != "" {
			cmd.Dir = cwd
		}

		_ = cmd.Run()
	} else {
		fmt.Printf("Context written to: %s\n", tmpFile)
		fmt.Printf("Run manually: claude -p \"$(cat %s)\"\n", tmpFile)
	}

	return InjectResult{
		Mode:          "context-injection",
		ContextFile:   tmpFile,
		TokenEstimate: ctx.TotalTokenEstimate,
		TurnCount:     len(ctx.Turns),
	}, nil
}

func claudeAvailable() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}
