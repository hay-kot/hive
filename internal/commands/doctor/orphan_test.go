package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hay-kot/hive/internal/core/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	sessions []session.Session
}

func (m *mockStore) List(_ context.Context) ([]session.Session, error) {
	return m.sessions, nil
}

func (m *mockStore) Get(_ context.Context, _ string) (session.Session, error) {
	return session.Session{}, nil
}

func (m *mockStore) Save(_ context.Context, _ session.Session) error {
	return nil
}

func (m *mockStore) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *mockStore) FindRecyclable(_ context.Context, _ string) (session.Session, error) {
	return session.Session{}, nil
}

func TestOrphanCheck_NoOrphans(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directories
	dir1 := filepath.Join(tmpDir, "repo-session-abc123")
	dir2 := filepath.Join(tmpDir, "repo-session-def456")
	require.NoError(t, os.MkdirAll(dir1, 0o755))
	require.NoError(t, os.MkdirAll(dir2, 0o755))

	// Mock store with matching sessions
	store := &mockStore{
		sessions: []session.Session{
			{ID: "abc123", Path: dir1},
			{ID: "def456", Path: dir2},
		},
	}

	check := NewOrphanCheck(store, tmpDir, false)
	result := check.Run(context.Background())

	assert.Equal(t, "Orphan Worktrees", result.Name)
	require.Len(t, result.Items, 1)
	assert.Equal(t, StatusPass, result.Items[0].Status)
	assert.Equal(t, "No orphans", result.Items[0].Label)
}

func TestOrphanCheck_WithOrphans(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directories - one tracked, one orphaned
	trackedDir := filepath.Join(tmpDir, "repo-tracked-abc123")
	orphanDir := filepath.Join(tmpDir, "repo-orphan-xyz789")
	require.NoError(t, os.MkdirAll(trackedDir, 0o755))
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))

	// Mock store with only one session
	store := &mockStore{
		sessions: []session.Session{
			{ID: "abc123", Path: trackedDir},
		},
	}

	check := NewOrphanCheck(store, tmpDir, false)
	result := check.Run(context.Background())

	assert.Equal(t, "Orphan Worktrees", result.Name)
	require.Len(t, result.Items, 1)
	assert.Equal(t, StatusWarn, result.Items[0].Status)
	assert.Equal(t, "repo-orphan-xyz789", result.Items[0].Label)
	assert.Contains(t, result.Items[0].Detail, "orphaned")
}

func TestOrphanCheck_NonexistentReposDir(t *testing.T) {
	store := &mockStore{}
	check := NewOrphanCheck(store, "/nonexistent/path", false)
	result := check.Run(context.Background())

	assert.Equal(t, "Orphan Worktrees", result.Name)
	require.Len(t, result.Items, 1)
	assert.Equal(t, StatusPass, result.Items[0].Status)
	assert.Equal(t, "Repos directory", result.Items[0].Label)
}

func TestOrphanCheck_IgnoresFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file (should be ignored) and a directory (orphan)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "somefile.txt"), []byte("test"), 0o644))
	orphanDir := filepath.Join(tmpDir, "orphan-dir")
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))

	store := &mockStore{sessions: []session.Session{}}
	check := NewOrphanCheck(store, tmpDir, false)
	result := check.Run(context.Background())

	// Should only report the directory, not the file
	require.Len(t, result.Items, 1)
	assert.Equal(t, StatusWarn, result.Items[0].Status)
	assert.Equal(t, "orphan-dir", result.Items[0].Label)
}

func TestOrphanCheck_FixDeletesOrphans(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an orphaned directory
	orphanDir := filepath.Join(tmpDir, "orphan-to-delete")
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))

	// Create a file inside to verify full deletion
	require.NoError(t, os.WriteFile(filepath.Join(orphanDir, "file.txt"), []byte("test"), 0o644))

	store := &mockStore{sessions: []session.Session{}}
	check := NewOrphanCheck(store, tmpDir, true) // fix=true
	result := check.Run(context.Background())

	// Should report successful deletion
	require.Len(t, result.Items, 1)
	assert.Equal(t, StatusPass, result.Items[0].Status)
	assert.Equal(t, "orphan-to-delete", result.Items[0].Label)
	assert.Contains(t, result.Items[0].Detail, "deleted")

	// Directory should actually be deleted
	_, err := os.Stat(orphanDir)
	assert.True(t, os.IsNotExist(err), "orphan directory should be deleted")
}

func TestOrphanCheck_FixPreservesTracked(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directories - one tracked, one orphaned
	trackedDir := filepath.Join(tmpDir, "repo-tracked-abc123")
	orphanDir := filepath.Join(tmpDir, "repo-orphan-xyz789")
	require.NoError(t, os.MkdirAll(trackedDir, 0o755))
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))

	store := &mockStore{
		sessions: []session.Session{
			{ID: "abc123", Path: trackedDir},
		},
	}

	check := NewOrphanCheck(store, tmpDir, true) // fix=true
	result := check.Run(context.Background())

	// Should only delete the orphan
	require.Len(t, result.Items, 1)
	assert.Equal(t, StatusPass, result.Items[0].Status)
	assert.Equal(t, "repo-orphan-xyz789", result.Items[0].Label)

	// Tracked directory should still exist
	_, err := os.Stat(trackedDir)
	require.NoError(t, err, "tracked directory should still exist")

	// Orphan should be deleted
	_, err = os.Stat(orphanDir)
	assert.True(t, os.IsNotExist(err), "orphan directory should be deleted")
}
