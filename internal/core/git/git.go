// Package git provides an abstraction for git operations.
package git

import (
	"context"
	"strings"
)

// Git defines git operations needed by hive.
type Git interface {
	// Clone clones a repository from url to dest.
	Clone(ctx context.Context, url, dest string) error
	// Checkout switches to the specified branch in dir.
	Checkout(ctx context.Context, dir, branch string) error
	// Pull fetches and merges changes in dir.
	Pull(ctx context.Context, dir string) error
	// ResetHard discards all local changes in dir.
	ResetHard(ctx context.Context, dir string) error
	// RemoteURL returns the origin remote URL for dir.
	RemoteURL(ctx context.Context, dir string) (string, error)
	// IsClean returns true if there are no uncommitted changes in dir.
	IsClean(ctx context.Context, dir string) (bool, error)
	// Branch returns the current branch name, or short commit SHA if in detached HEAD state.
	Branch(ctx context.Context, dir string) (string, error)
	// DefaultBranch returns the default branch name (e.g., "main" or "master") for the repository.
	DefaultBranch(ctx context.Context, dir string) (string, error)
	// DiffStats returns the number of lines added and deleted compared to the default branch.
	DiffStats(ctx context.Context, dir string) (additions, deletions int, err error)
	// IsValidRepo checks if dir contains a valid git repository.
	IsValidRepo(ctx context.Context, dir string) error
}

// ExtractRepoName extracts the repository name from a git remote URL.
// Handles both SSH (git@github.com:user/repo.git) and HTTPS (https://github.com/user/repo.git) formats.
func ExtractRepoName(remote string) string {
	remote = strings.TrimSuffix(remote, ".git")

	if idx := strings.LastIndex(remote, "/"); idx != -1 {
		return remote[idx+1:]
	}

	if idx := strings.LastIndex(remote, ":"); idx != -1 {
		part := remote[idx+1:]
		if slashIdx := strings.LastIndex(part, "/"); slashIdx != -1 {
			return part[slashIdx+1:]
		}
		return part
	}

	return remote
}
