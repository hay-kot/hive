package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSession_CanRecycle(t *testing.T) {
	tests := []struct {
		name  string
		state State
		want  bool
	}{
		{
			name:  "active session can be recycled",
			state: StateActive,
			want:  true,
		},
		{
			name:  "recycled session cannot be recycled",
			state: StateRecycled,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Session{State: tt.state}
			assert.Equal(t, tt.want, s.CanRecycle())
		})
	}
}

func TestSession_MarkRecycled(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	s := Session{
		ID:        "test-id",
		State:     StateActive,
		UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	s.MarkRecycled(now)

	assert.Equal(t, StateRecycled, s.State)
	assert.Equal(t, now, s.UpdatedAt)
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "My Session", "my-session"},
		{"multiple spaces", "My   Session   Name", "my-session-name"},
		{"special chars", "Feature: Add Login!", "feature-add-login"},
		{"already slug", "my-session", "my-session"},
		{"leading/trailing spaces", "  My Session  ", "my-session"},
		{"numbers", "Session 123", "session-123"},
		{"underscores", "my_session_name", "my-session-name"},
		{"mixed case", "MySessionName", "mysessionname"},
		{"empty after trim", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Slugify(tt.in))
		})
	}
}
