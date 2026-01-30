package messaging

import "time"

// Message represents a single message published to a topic.
type Message struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	Payload   string    `json:"payload"`
	Sender    string    `json:"sender,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Topic represents a named channel for messages.
type Topic struct {
	Name      string    `json:"name"`
	Messages  []Message `json:"messages"`
	UpdatedAt time.Time `json:"updated_at"`
}
