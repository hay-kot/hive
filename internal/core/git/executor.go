package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hay-kot/hive/pkg/executil"
)

// Executor implements Git using the git command-line tool.
type Executor struct {
	gitPath string
	exec    executil.Executor
}

// NewExecutor creates a new git executor with the specified git binary path.
func NewExecutor(gitPath string, exec executil.Executor) *Executor {
	return &Executor{gitPath: gitPath, exec: exec}
}

func (e *Executor) Clone(ctx context.Context, url, dest string) error {
	if _, err := e.exec.Run(ctx, e.gitPath, "clone", url, dest); err != nil {
		return fmt.Errorf("clone %s to %s: %w", url, dest, err)
	}
	return nil
}

func (e *Executor) Checkout(ctx context.Context, dir, branch string) error {
	if _, err := e.exec.RunDir(ctx, dir, e.gitPath, "checkout", branch); err != nil {
		return fmt.Errorf("checkout %s: %w", branch, err)
	}
	return nil
}

func (e *Executor) Pull(ctx context.Context, dir string) error {
	if _, err := e.exec.RunDir(ctx, dir, e.gitPath, "pull"); err != nil {
		return fmt.Errorf("pull: %w", err)
	}
	return nil
}

func (e *Executor) ResetHard(ctx context.Context, dir string) error {
	if _, err := e.exec.RunDir(ctx, dir, e.gitPath, "reset", "--hard"); err != nil {
		return fmt.Errorf("reset --hard: %w", err)
	}
	return nil
}

func (e *Executor) RemoteURL(ctx context.Context, dir string) (string, error) {
	out, err := e.exec.RunDir(ctx, dir, e.gitPath, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("get remote url: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (e *Executor) IsClean(ctx context.Context, dir string) (bool, error) {
	out, err := e.exec.RunDir(ctx, dir, e.gitPath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return len(strings.TrimSpace(string(out))) == 0, nil
}

func (e *Executor) Branch(ctx context.Context, dir string) (string, error) {
	// Try to get branch name first
	out, err := e.exec.RunDir(ctx, dir, e.gitPath, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("git branch: %w", err)
	}

	branch := strings.TrimSpace(string(out))
	if branch != "" {
		return branch, nil
	}

	// Empty branch name means detached HEAD - get short commit SHA
	out, err = e.exec.RunDir(ctx, dir, e.gitPath, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

func (e *Executor) DefaultBranch(ctx context.Context, dir string) (string, error) {
	// Get the default branch from origin's HEAD reference
	out, err := e.exec.RunDir(ctx, dir, e.gitPath, "symbolic-ref", "refs/remotes/origin/HEAD", "--short")
	if err != nil {
		return "", fmt.Errorf("git symbolic-ref: %w", err)
	}

	// Output is "origin/main" or "origin/master", strip the "origin/" prefix
	branch := strings.TrimSpace(string(out))
	branch = strings.TrimPrefix(branch, "origin/")

	return branch, nil
}

func (e *Executor) DiffStats(ctx context.Context, dir string) (additions, deletions int, err error) {
	// Get the default branch to compare against
	defaultBranch, err := e.DefaultBranch(ctx, dir)
	if err != nil {
		// Fallback to comparing against HEAD if we can't determine default branch
		defaultBranch = "HEAD"
	}

	var out []byte
	if defaultBranch == "HEAD" {
		// Compare working directory against HEAD
		out, err = e.exec.RunDir(ctx, dir, e.gitPath, "diff", "--shortstat", "HEAD")
	} else {
		// Compare current branch against default branch (e.g., main...HEAD)
		out, err = e.exec.RunDir(ctx, dir, e.gitPath, "diff", "--shortstat", defaultBranch+"...HEAD")
	}

	if err != nil {
		return 0, 0, fmt.Errorf("git diff: %w", err)
	}

	return parseDiffStats(string(out))
}

// parseDiffStats parses git diff --shortstat output.
// Example: " 3 files changed, 10 insertions(+), 5 deletions(-)"
func parseDiffStats(output string) (additions, deletions int, err error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return 0, 0, nil
	}

	// Parse insertions
	if idx := strings.Index(output, "insertion"); idx != -1 {
		// Find the number before "insertion"
		start := strings.LastIndex(output[:idx], ",")
		if start == -1 {
			start = strings.LastIndex(output[:idx], "changed")
		}
		if start != -1 {
			numStr := strings.TrimSpace(output[start+1 : idx])
			numStr = strings.Fields(numStr)[0]
			additions, _ = parseInt(numStr)
		}
	}

	// Parse deletions
	if idx := strings.Index(output, "deletion"); idx != -1 {
		// Find the number before "deletion"
		start := strings.LastIndex(output[:idx], ",")
		if start != -1 {
			numStr := strings.TrimSpace(output[start+1 : idx])
			numStr = strings.Fields(numStr)[0]
			deletions, _ = parseInt(numStr)
		}
	}

	return additions, deletions, nil
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n, nil
}

func (e *Executor) IsValidRepo(ctx context.Context, dir string) error {
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf(".git directory missing")
	}

	if _, err := e.exec.RunDir(ctx, dir, e.gitPath, "rev-parse", "--git-dir"); err != nil {
		return fmt.Errorf("git rev-parse failed: %w", err)
	}

	return nil
}
