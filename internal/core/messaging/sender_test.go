package messaging

import (
	"context"
	"testing"

	"github.com/hay-kot/hive/internal/core/session"
)

// mockSessionStore implements session.Store for testing.
type mockSessionStore struct {
	sessions []session.Session
}

func (m *mockSessionStore) List(_ context.Context) ([]session.Session, error) {
	return m.sessions, nil
}

func (m *mockSessionStore) Get(_ context.Context, id string) (session.Session, error) {
	for _, s := range m.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return session.Session{}, session.ErrNotFound
}

func (m *mockSessionStore) Save(_ context.Context, _ session.Session) error {
	return nil
}

func (m *mockSessionStore) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *mockSessionStore) FindRecyclable(_ context.Context, _ string) (session.Session, error) {
	return session.Session{}, session.ErrNoRecyclable
}

func TestSessionDetector_DetectSessionFromPath(t *testing.T) {
	store := &mockSessionStore{
		sessions: []session.Session{
			{ID: "sess-1", Path: "/home/user/projects/foo", State: session.StateActive},
			{ID: "sess-2", Path: "/home/user/projects/bar", State: session.StateActive},
			{ID: "sess-3", Path: "/home/user/projects/foo/nested", State: session.StateActive},
			{ID: "recycled", Path: "/home/user/projects/old", State: session.StateRecycled},
		},
	}

	detector := NewSessionDetector(store)
	ctx := context.Background()

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "exact match",
			path:     "/home/user/projects/foo",
			expected: "sess-1",
		},
		{
			name:     "subdirectory match",
			path:     "/home/user/projects/foo/src/main.go",
			expected: "sess-1",
		},
		{
			name:     "nested session takes precedence",
			path:     "/home/user/projects/foo/nested/deep/file.go",
			expected: "sess-3",
		},
		{
			name:     "different session",
			path:     "/home/user/projects/bar/code",
			expected: "sess-2",
		},
		{
			name:     "no match",
			path:     "/home/user/other/project",
			expected: "",
		},
		{
			name:     "recycled session ignored",
			path:     "/home/user/projects/old/file.txt",
			expected: "",
		},
		{
			name:     "partial name match is not a match",
			path:     "/home/user/projects/foobar",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detector.DetectSessionFromPath(ctx, tt.path)
			if err != nil {
				t.Fatalf("DetectSessionFromPath failed: %v", err)
			}
			if got != tt.expected {
				t.Errorf("DetectSessionFromPath(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestSessionDetector_EmptyStore(t *testing.T) {
	store := &mockSessionStore{sessions: nil}
	detector := NewSessionDetector(store)
	ctx := context.Background()

	got, err := detector.DetectSessionFromPath(ctx, "/any/path")
	if err != nil {
		t.Fatalf("DetectSessionFromPath failed: %v", err)
	}
	if got != "" {
		t.Errorf("DetectSessionFromPath with empty store = %q, want empty", got)
	}
}

func TestIsSubpath(t *testing.T) {
	tests := []struct {
		parent   string
		child    string
		expected bool
	}{
		{"/home/user", "/home/user/projects", true},
		{"/home/user", "/home/user", false}, // Equal, not sub
		{"/home/user", "/home/username", false},
		{"/home/user/projects", "/home/user", false},
		{"/a/b/c", "/a/b/c/d/e/f", true},
	}

	for _, tt := range tests {
		t.Run(tt.parent+"->"+tt.child, func(t *testing.T) {
			got := isSubpath(tt.parent, tt.child)
			if got != tt.expected {
				t.Errorf("isSubpath(%q, %q) = %v, want %v", tt.parent, tt.child, got, tt.expected)
			}
		})
	}
}
