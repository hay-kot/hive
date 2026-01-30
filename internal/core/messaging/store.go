package messaging

import (
	"context"
	"errors"
	"time"
)

var ErrTopicNotFound = errors.New("topic not found")

// Store defines the interface for message persistence.
type Store interface {
	// Publish adds a message to a topic, creating the topic if it doesn't exist.
	Publish(ctx context.Context, msg Message) error

	// Subscribe returns all messages for a topic, optionally filtered by since timestamp.
	// Returns ErrTopicNotFound if the topic doesn't exist.
	Subscribe(ctx context.Context, topic string, since time.Time) ([]Message, error)

	// List returns all topic names.
	List(ctx context.Context) ([]string, error)

	// Prune removes messages older than the given duration across all topics.
	// Returns the number of messages removed.
	Prune(ctx context.Context, olderThan time.Duration) (int, error)
}
