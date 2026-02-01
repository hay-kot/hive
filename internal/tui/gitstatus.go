package tui

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/hay-kot/hive/internal/core/git"
)

const gitStatusTimeout = 5 * time.Second

// GitStatus holds the git status information for a session.
type GitStatus struct {
	Branch     string
	Additions  int
	Deletions  int
	HasChanges bool
	IsLoading  bool
	Error      error
}

// gitStatusBatchCompleteMsg is sent when all git status fetches complete.
type gitStatusBatchCompleteMsg struct {
	Results map[string]GitStatus
}

// fetchGitStatusForPath fetches git status for a single path.
func fetchGitStatusForPath(ctx context.Context, g git.Git, path string) GitStatus {
	status := GitStatus{}

	// Get branch name
	branch, err := g.Branch(ctx, path)
	if err != nil {
		status.Error = err
		return status
	}
	status.Branch = branch

	// Get diff stats
	additions, deletions, err := g.DiffStats(ctx, path)
	if err != nil {
		status.Error = err
		return status
	}
	status.Additions = additions
	status.Deletions = deletions

	// Check if clean
	isClean, err := g.IsClean(ctx, path)
	if err != nil {
		status.Error = err
		return status
	}
	status.HasChanges = !isClean

	return status
}

// fetchGitStatusBatch returns a command that fetches git status for multiple paths
// using a bounded worker pool.
func fetchGitStatusBatch(g git.Git, paths []string, workers int) tea.Cmd {
	if len(paths) == 0 {
		return nil
	}

	return func() tea.Msg {
		results := make(map[string]GitStatus)
		var mu sync.Mutex

		// Create a semaphore to limit concurrency
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup

		for _, path := range paths {
			wg.Add(1)
			go func(p string) {
				defer wg.Done()

				// Acquire semaphore
				sem <- struct{}{}
				defer func() { <-sem }()

				ctx, cancel := context.WithTimeout(context.Background(), gitStatusTimeout)
				defer cancel()

				status := fetchGitStatusForPath(ctx, g, p)

				mu.Lock()
				results[p] = status
				mu.Unlock()
			}(path)
		}

		wg.Wait()
		return gitStatusBatchCompleteMsg{Results: results}
	}
}
