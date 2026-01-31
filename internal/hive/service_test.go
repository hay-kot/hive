package hive

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/hay-kot/hive/internal/core/config"
	"github.com/hay-kot/hive/internal/core/git"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore implements session.Store for testing.
type mockStore struct {
	sessions map[string]session.Session
}

func newMockStore() *mockStore {
	return &mockStore{sessions: make(map[string]session.Session)}
}

func (m *mockStore) List(_ context.Context) ([]session.Session, error) {
	var result []session.Session
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockStore) Get(_ context.Context, id string) (session.Session, error) {
	s, ok := m.sessions[id]
	if !ok {
		return session.Session{}, session.ErrNotFound
	}
	return s, nil
}

func (m *mockStore) Save(_ context.Context, s session.Session) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockStore) Delete(_ context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockStore) FindRecyclable(_ context.Context, remote string) (session.Session, error) {
	for _, s := range m.sessions {
		if s.State == session.StateRecycled && s.Remote == remote {
			return s, nil
		}
	}
	return session.Session{}, session.ErrNoRecyclable
}

// mockGit implements git.Git for testing.
type mockGit struct{}

func (m *mockGit) Clone(_ context.Context, _, _ string) error            { return nil }
func (m *mockGit) Checkout(_ context.Context, _, _ string) error         { return nil }
func (m *mockGit) Pull(_ context.Context, _ string) error                { return nil }
func (m *mockGit) ResetHard(_ context.Context, _ string) error           { return nil }
func (m *mockGit) RemoteURL(_ context.Context, _ string) (string, error) { return "", nil }
func (m *mockGit) IsClean(_ context.Context, _ string) (bool, error)     { return true, nil }
func (m *mockGit) Branch(_ context.Context, _ string) (string, error)    { return "main", nil }
func (m *mockGit) DefaultBranch(_ context.Context, _ string) (string, error) {
	return "main", nil
}
func (m *mockGit) DiffStats(_ context.Context, _ string) (int, int, error) { return 0, 0, nil }
func (m *mockGit) IsValidRepo(_ context.Context, _ string) error           { return nil }

func newTestService(t *testing.T, store session.Store, cfg *config.Config) *Service {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{
			DataDir: t.TempDir(),
			GitPath: "git",
		}
	}
	log := zerolog.New(io.Discard)
	return New(store, &mockGit{}, cfg, nil, log, io.Discard, io.Discard)
}

func TestEnforceMaxRecycled(t *testing.T) {
	intPtr := func(n int) *int { return &n }

	t.Run("unlimited (0) does nothing", func(t *testing.T) {
		store := newMockStore()
		cfg := &config.Config{
			DataDir: t.TempDir(),
			GitPath: "git",
			Rules:   []config.Rule{{Pattern: "", MaxRecycled: intPtr(0)}},
		}
		svc := newTestService(t, store, cfg)

		// Add sessions
		remote := "https://github.com/test/repo"
		for i := 0; i < 10; i++ {
			sess := session.Session{
				ID:        string(rune('a' + i)),
				Remote:    remote,
				State:     session.StateRecycled,
				Path:      t.TempDir(),
				UpdatedAt: time.Now().Add(time.Duration(i) * time.Hour),
			}
			store.sessions[sess.ID] = sess
		}

		err := svc.enforceMaxRecycled(context.Background(), remote)
		require.NoError(t, err)

		// All 10 should still exist
		sessions, _ := store.List(context.Background())
		assert.Len(t, sessions, 10)
	})

	t.Run("deletes oldest beyond limit", func(t *testing.T) {
		store := newMockStore()
		cfg := &config.Config{
			DataDir: t.TempDir(),
			GitPath: "git",
			Rules:   []config.Rule{{Pattern: "", MaxRecycled: intPtr(3)}},
		}
		svc := newTestService(t, store, cfg)

		remote := "https://github.com/test/repo"
		baseTime := time.Now()

		// Create 5 sessions with different ages
		for i := 0; i < 5; i++ {
			sess := session.Session{
				ID:        string(rune('a' + i)),
				Remote:    remote,
				State:     session.StateRecycled,
				Path:      t.TempDir(),
				UpdatedAt: baseTime.Add(time.Duration(i) * time.Hour), // a=oldest, e=newest
			}
			store.sessions[sess.ID] = sess
		}

		err := svc.enforceMaxRecycled(context.Background(), remote)
		require.NoError(t, err)

		// Should keep 3 newest: c, d, e
		sessions, _ := store.List(context.Background())
		assert.Len(t, sessions, 3)

		// Verify the newest 3 remain
		for _, s := range sessions {
			assert.Contains(t, []string{"c", "d", "e"}, s.ID, "expected only newest sessions to remain")
		}
	})

	t.Run("only affects matching remote", func(t *testing.T) {
		store := newMockStore()
		cfg := &config.Config{
			DataDir: t.TempDir(),
			GitPath: "git",
			Rules:   []config.Rule{{Pattern: "", MaxRecycled: intPtr(1)}},
		}
		svc := newTestService(t, store, cfg)

		remote1 := "https://github.com/test/repo1"
		remote2 := "https://github.com/test/repo2"
		baseTime := time.Now()

		// 3 sessions for repo1
		for i := 0; i < 3; i++ {
			sess := session.Session{
				ID:        string(rune('a' + i)),
				Remote:    remote1,
				State:     session.StateRecycled,
				Path:      t.TempDir(),
				UpdatedAt: baseTime.Add(time.Duration(i) * time.Hour),
			}
			store.sessions[sess.ID] = sess
		}

		// 3 sessions for repo2
		for i := 0; i < 3; i++ {
			sess := session.Session{
				ID:        string(rune('x' + i)),
				Remote:    remote2,
				State:     session.StateRecycled,
				Path:      t.TempDir(),
				UpdatedAt: baseTime.Add(time.Duration(i) * time.Hour),
			}
			store.sessions[sess.ID] = sess
		}

		// Enforce only for repo1
		err := svc.enforceMaxRecycled(context.Background(), remote1)
		require.NoError(t, err)

		sessions, _ := store.List(context.Background())
		// 1 from repo1 + 3 from repo2 = 4
		assert.Len(t, sessions, 4)
	})
}

func TestPrune(t *testing.T) {
	intPtr := func(n int) *int { return &n }

	t.Run("all=true deletes all recycled", func(t *testing.T) {
		store := newMockStore()
		cfg := &config.Config{
			DataDir: t.TempDir(),
			GitPath: "git",
			Rules:   []config.Rule{{Pattern: "", MaxRecycled: intPtr(10)}}, // high limit shouldn't matter with all=true
		}
		svc := newTestService(t, store, cfg)

		remote := "https://github.com/test/repo"
		// Add recycled sessions
		for i := 0; i < 5; i++ {
			sess := session.Session{
				ID:        string(rune('a' + i)),
				Remote:    remote,
				State:     session.StateRecycled,
				Path:      t.TempDir(),
				UpdatedAt: time.Now(),
			}
			store.sessions[sess.ID] = sess
		}
		// Add active session
		store.sessions["active"] = session.Session{
			ID:     "active",
			Remote: remote,
			State:  session.StateActive,
			Path:   t.TempDir(),
		}

		count, err := svc.Prune(context.Background(), true)
		require.NoError(t, err)
		assert.Equal(t, 5, count)

		sessions, _ := store.List(context.Background())
		assert.Len(t, sessions, 1)
		assert.Equal(t, "active", sessions[0].ID)
	})

	t.Run("all=false respects max_recycled", func(t *testing.T) {
		store := newMockStore()
		cfg := &config.Config{
			DataDir: t.TempDir(),
			GitPath: "git",
			Rules:   []config.Rule{{Pattern: "", MaxRecycled: intPtr(2)}},
		}
		svc := newTestService(t, store, cfg)

		remote := "https://github.com/test/repo"
		baseTime := time.Now()

		// Add 5 recycled sessions
		for i := 0; i < 5; i++ {
			sess := session.Session{
				ID:        string(rune('a' + i)),
				Remote:    remote,
				State:     session.StateRecycled,
				Path:      t.TempDir(),
				UpdatedAt: baseTime.Add(time.Duration(i) * time.Hour),
			}
			store.sessions[sess.ID] = sess
		}

		count, err := svc.Prune(context.Background(), false)
		require.NoError(t, err)
		assert.Equal(t, 3, count) // Should delete 3 (5-2)

		sessions, _ := store.List(context.Background())
		assert.Len(t, sessions, 2)
	})

	t.Run("always deletes corrupted", func(t *testing.T) {
		store := newMockStore()
		cfg := &config.Config{
			DataDir: t.TempDir(),
			GitPath: "git",
			Rules:   []config.Rule{{Pattern: "", MaxRecycled: intPtr(10)}}, // high limit
		}
		svc := newTestService(t, store, cfg)

		// Add corrupted session
		store.sessions["corrupted"] = session.Session{
			ID:    "corrupted",
			State: session.StateCorrupted,
			Path:  t.TempDir(),
		}
		// Add active session
		store.sessions["active"] = session.Session{
			ID:    "active",
			State: session.StateActive,
			Path:  t.TempDir(),
		}

		count, err := svc.Prune(context.Background(), false)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		sessions, _ := store.List(context.Background())
		assert.Len(t, sessions, 1)
		assert.Equal(t, "active", sessions[0].ID)
	})

	t.Run("per-rule max_recycled", func(t *testing.T) {
		store := newMockStore()
		cfg := &config.Config{
			DataDir: t.TempDir(),
			GitPath: "git",
			Rules: []config.Rule{
				{Pattern: "", MaxRecycled: intPtr(5)},                     // catch-all default
				{Pattern: "github.com/strict/.*", MaxRecycled: intPtr(1)}, // strict override
			},
		}
		svc := newTestService(t, store, cfg)

		remoteStrict := "https://github.com/strict/repo"
		remoteNormal := "https://github.com/normal/repo"
		baseTime := time.Now()

		// Add 3 sessions for strict repo
		for i := 0; i < 3; i++ {
			sess := session.Session{
				ID:        string(rune('a' + i)),
				Remote:    remoteStrict,
				State:     session.StateRecycled,
				Path:      t.TempDir(),
				UpdatedAt: baseTime.Add(time.Duration(i) * time.Hour),
			}
			store.sessions[sess.ID] = sess
		}

		// Add 3 sessions for normal repo
		for i := 0; i < 3; i++ {
			sess := session.Session{
				ID:        string(rune('x' + i)),
				Remote:    remoteNormal,
				State:     session.StateRecycled,
				Path:      t.TempDir(),
				UpdatedAt: baseTime.Add(time.Duration(i) * time.Hour),
			}
			store.sessions[sess.ID] = sess
		}

		count, err := svc.Prune(context.Background(), false)
		require.NoError(t, err)
		// strict: keep 1, delete 2
		// normal: keep 3 (under limit of 5)
		assert.Equal(t, 2, count)

		sessions, _ := store.List(context.Background())
		assert.Len(t, sessions, 4)

		// Count per remote
		strictCount := 0
		normalCount := 0
		for _, s := range sessions {
			if s.Remote == remoteStrict {
				strictCount++
			} else {
				normalCount++
			}
		}
		assert.Equal(t, 1, strictCount, "strict repo should have 1 session")
		assert.Equal(t, 3, normalCount, "normal repo should have 3 sessions")
	})
}

// Ensure the mock implements the interface at compile time.
var (
	_ git.Git       = (*mockGit)(nil)
	_ session.Store = (*mockStore)(nil)
)
