package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/amr/naqb/internal/log"
)

// GitCommit stages all changes in bookDir and creates a commit.
// It silently skips if git is not initialized or there's nothing to commit.
func GitCommit(bookDir, message string) error {
	// Check if git is initialized
	gitDir := filepath.Join(bookDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		log.Debug("git commit skipped: not a git repo", "dir", bookDir)
		return nil
	}

	// Stage all changes
	addCmd := exec.Command("git", "-C", bookDir, "add", "-A")
	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		log.Error("git add failed", "dir", bookDir, "err", err)
		return fmt.Errorf("git add failed: %w", err)
	}

	// Check if there's anything to commit
	statusCmd := exec.Command("git", "-C", bookDir, "status", "--porcelain")
	out, err := statusCmd.Output()
	if err != nil {
		log.Warn("git status failed", "dir", bookDir, "err", err)
		return nil
	}
	if len(out) == 0 {
		log.Debug("git commit skipped: nothing to commit", "dir", bookDir)
		return nil
	}

	// Commit
	log.Info("git commit", "dir", bookDir, "message", message)
	commitCmd := exec.Command("git", "-C", bookDir, "commit", "-m", message)
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		log.Error("git commit failed", "dir", bookDir, "message", message, "err", err)
		return err
	}
	return nil
}

// GitInit initializes a git repo in bookDir.
func GitInit(bookDir string) error {
	log.Info("git init", "dir", bookDir)
	cmd := exec.Command("git", "-C", bookDir, "init")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
