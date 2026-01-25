package history

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a history entry is not found.
var ErrNotFound = errors.New("history entry not found")

// Store defines persistence operations for command history.
type Store interface {
	// List returns all history entries, newest first.
	List(ctx context.Context) ([]Entry, error)
	// Get returns a history entry by ID. Returns ErrNotFound if not found.
	Get(ctx context.Context, id string) (Entry, error)
	// Save adds a new history entry, pruning oldest entries if count exceeds the configured maximum.
	Save(ctx context.Context, entry Entry) error
	// Clear removes all history entries.
	Clear(ctx context.Context) error
	// LastFailed returns the most recent failed entry. Returns ErrNotFound if none.
	LastFailed(ctx context.Context) (Entry, error)
}
