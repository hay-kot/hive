package messaging

import "time"

// ActivityType represents the type of messaging activity.
type ActivityType string

const (
	ActivityPublish   ActivityType = "publish"
	ActivitySubscribe ActivityType = "subscribe"
)

// Activity represents a messaging activity event.
type Activity struct {
	ID        string       `json:"id"`
	Type      ActivityType `json:"type"`
	Topic     string       `json:"topic"`
	SessionID string       `json:"session_id,omitempty"`
	Sender    string       `json:"sender,omitempty"`
	MessageID string       `json:"message_id,omitempty"` // For publish events
	Timestamp time.Time    `json:"timestamp"`
}

// ActivityStore defines persistence operations for activity events.
type ActivityStore interface {
	// Record records an activity event.
	Record(activity Activity) error
	// List returns recent activity events, newest first.
	// Limit of 0 returns all events.
	List(limit int) ([]Activity, error)
	// ListSince returns activity events since the given time, newest first.
	ListSince(since time.Time, limit int) ([]Activity, error)
}
