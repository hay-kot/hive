// Package context provides context management for inter-agent communication.
package context

import (
	"context"
	"errors"
	"time"
)

// ErrKeyNotFound is returned when a key does not exist.
var ErrKeyNotFound = errors.New("key not found")

// Entry represents a KV store entry with metadata.
type Entry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store defines persistence operations for context KV data.
type Store interface {
	Get(ctx context.Context, key string) (Entry, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]Entry, error)
	Watch(ctx context.Context, key string, after time.Time, timeout time.Duration) (Entry, error)
}
