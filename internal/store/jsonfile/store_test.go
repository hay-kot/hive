package jsonfile

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/hay-kot/hive/internal/core/session"
)

func TestStore(t *testing.T) {
	ctx := context.Background()

	t.Run("save and get", func(t *testing.T) {
		store := New(filepath.Join(t.TempDir(), "sessions.json"))

		sess := session.Session{
			ID:        "test-id",
			Name:      "test-session",
			Path:      "/tmp/test",
			Remote:    "https://github.com/test/repo",
			State:     session.StateActive,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := store.Save(ctx, sess); err != nil {
			t.Fatalf("Save: %v", err)
		}

		got, err := store.Get(ctx, "test-id")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}

		if got.ID != sess.ID || got.Name != sess.Name {
			t.Errorf("got %+v, want %+v", got, sess)
		}
	})

	t.Run("get not found", func(t *testing.T) {
		store := New(filepath.Join(t.TempDir(), "sessions.json"))

		_, err := store.Get(ctx, "nonexistent")
		if !errors.Is(err, session.ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		store := New(filepath.Join(t.TempDir(), "sessions.json"))

		sessions, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(sessions) != 0 {
			t.Errorf("got %d sessions, want 0", len(sessions))
		}

		for _, name := range []string{"first", "second"} {
			if err := store.Save(ctx, session.Session{ID: name, Name: name}); err != nil {
				t.Fatalf("Save %s: %v", name, err)
			}
		}

		sessions, err = store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(sessions) != 2 {
			t.Errorf("got %d sessions, want 2", len(sessions))
		}
	})

	t.Run("save updates existing", func(t *testing.T) {
		store := New(filepath.Join(t.TempDir(), "sessions.json"))

		sess := session.Session{ID: "update-test", Name: "original"}
		if err := store.Save(ctx, sess); err != nil {
			t.Fatalf("Save: %v", err)
		}

		sess.Name = "updated"
		if err := store.Save(ctx, sess); err != nil {
			t.Fatalf("Save update: %v", err)
		}

		got, err := store.Get(ctx, "update-test")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Name != "updated" {
			t.Errorf("got name %q, want %q", got.Name, "updated")
		}

		sessions, _ := store.List(ctx)
		if len(sessions) != 1 {
			t.Errorf("got %d sessions, want 1", len(sessions))
		}
	})

	t.Run("delete", func(t *testing.T) {
		store := New(filepath.Join(t.TempDir(), "sessions.json"))

		if err := store.Save(ctx, session.Session{ID: "delete-me"}); err != nil {
			t.Fatalf("Save: %v", err)
		}

		if err := store.Delete(ctx, "delete-me"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		_, err := store.Get(ctx, "delete-me")
		if !errors.Is(err, session.ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("delete not found", func(t *testing.T) {
		store := New(filepath.Join(t.TempDir(), "sessions.json"))

		err := store.Delete(ctx, "nonexistent")
		if !errors.Is(err, session.ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("find recyclable", func(t *testing.T) {
		store := New(filepath.Join(t.TempDir(), "sessions.json"))
		remote := "https://github.com/test/repo"

		// No recyclable sessions
		_, err := store.FindRecyclable(ctx, remote)
		if !errors.Is(err, session.ErrNoRecyclable) {
			t.Errorf("empty store: got %v, want ErrNoRecyclable", err)
		}

		// Active session with matching remote - not recyclable
		if err := store.Save(ctx, session.Session{
			ID:     "active",
			Remote: remote,
			State:  session.StateActive,
		}); err != nil {
			t.Fatalf("Save: %v", err)
		}

		_, err = store.FindRecyclable(ctx, remote)
		if !errors.Is(err, session.ErrNoRecyclable) {
			t.Errorf("active session: got %v, want ErrNoRecyclable", err)
		}

		// Recycled session with different remote - not found
		if err := store.Save(ctx, session.Session{
			ID:     "different",
			Remote: "https://github.com/other/repo",
			State:  session.StateRecycled,
		}); err != nil {
			t.Fatalf("Save: %v", err)
		}

		_, err = store.FindRecyclable(ctx, remote)
		if !errors.Is(err, session.ErrNoRecyclable) {
			t.Errorf("different remote: got %v, want ErrNoRecyclable", err)
		}

		// Recycled session with matching remote - found
		if err := store.Save(ctx, session.Session{
			ID:     "recycled",
			Remote: remote,
			State:  session.StateRecycled,
		}); err != nil {
			t.Fatalf("Save: %v", err)
		}

		got, err := store.FindRecyclable(ctx, remote)
		if err != nil {
			t.Fatalf("FindRecyclable: %v", err)
		}
		if got.ID != "recycled" {
			t.Errorf("got ID %q, want %q", got.ID, "recycled")
		}
	})
}
