package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hay-kot/hive/internal/core/session"
)

// OrphanCheck detects directories in the repos folder without session records.
type OrphanCheck struct {
	sessions session.Store
	reposDir string
	fix      bool
}

// NewOrphanCheck creates a new orphan worktree check.
// If fix is true, orphaned directories will be deleted.
func NewOrphanCheck(sessions session.Store, reposDir string, fix bool) *OrphanCheck {
	return &OrphanCheck{
		sessions: sessions,
		reposDir: reposDir,
		fix:      fix,
	}
}

func (c *OrphanCheck) Name() string {
	return "Orphan Worktrees"
}

func (c *OrphanCheck) Run(ctx context.Context) Result {
	result := Result{Name: c.Name()}

	// Get all known session paths
	sessions, err := c.sessions.List(ctx)
	if err != nil {
		result.Items = append(result.Items, CheckItem{
			Label:  "List sessions",
			Status: StatusFail,
			Detail: err.Error(),
		})
		return result
	}

	knownPaths := make(map[string]bool, len(sessions))
	for _, sess := range sessions {
		knownPaths[sess.Path] = true
	}

	// Check if repos directory exists
	if _, err := os.Stat(c.reposDir); os.IsNotExist(err) {
		result.Items = append(result.Items, CheckItem{
			Label:  "Repos directory",
			Status: StatusPass,
			Detail: "no repos directory yet",
		})
		return result
	}

	// List all directories in repos folder
	entries, err := os.ReadDir(c.reposDir)
	if err != nil {
		result.Items = append(result.Items, CheckItem{
			Label:  "Read repos directory",
			Status: StatusFail,
			Detail: err.Error(),
		})
		return result
	}

	var orphans []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(c.reposDir, entry.Name())
		if !knownPaths[dirPath] {
			orphans = append(orphans, entry.Name())
		}
	}

	if len(orphans) == 0 {
		result.Items = append(result.Items, CheckItem{
			Label:  "No orphans",
			Status: StatusPass,
			Detail: "all worktrees have session records",
		})
		return result
	}

	// Handle orphans - either report or fix
	for _, name := range orphans {
		dirPath := filepath.Join(c.reposDir, name)

		if c.fix {
			if err := os.RemoveAll(dirPath); err != nil {
				result.Items = append(result.Items, CheckItem{
					Label:  name,
					Status: StatusFail,
					Detail: fmt.Sprintf("failed to delete: %v", err),
				})
			} else {
				result.Items = append(result.Items, CheckItem{
					Label:  name,
					Status: StatusPass,
					Detail: "deleted orphaned worktree",
				})
			}
		} else {
			result.Items = append(result.Items, CheckItem{
				Label:   name,
				Status:  StatusWarn,
				Detail:  "orphaned worktree (no session record)",
				Fixable: true,
			})
		}
	}

	return result
}
