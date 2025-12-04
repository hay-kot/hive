package session

import (
	"context"
	"errors"
)

// Sentinel errors for session operations.
var (
	ErrNotFound     = errors.New("session not found")
	ErrNoRecyclable = errors.New("no recyclable session found")
)

// Store defines persistence operations for sessions.
type Store interface {
	// List returns all sessions.
	List(ctx context.Context) ([]Session, error)
	// Get returns a session by ID. Returns ErrNotFound if not found.
	Get(ctx context.Context, id string) (Session, error)
	// Save creates or updates a session.
	Save(ctx context.Context, s Session) error
	// Delete removes a session by ID. Returns ErrNotFound if not found.
	Delete(ctx context.Context, id string) error
	// FindRecyclable returns a recyclable session for the given remote.
	// Returns ErrNoRecyclable if none available.
	FindRecyclable(ctx context.Context, remote string) (Session, error)
}
