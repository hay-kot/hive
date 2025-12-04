// Package session defines session domain types and interfaces.
package session

import "time"

// State represents the lifecycle state of a session.
type State string

const (
	StateActive   State = "active"
	StateRecycled State = "recycled"
)

// Session represents an isolated git environment for an AI agent.
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Remote    string    `json:"remote"`
	State     State     `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CanRecycle returns true if the session can be marked for recycling.
func (s *Session) CanRecycle() bool {
	return s.State == StateActive
}

// MarkRecycled transitions the session to the recycled state.
func (s *Session) MarkRecycled(now time.Time) {
	s.State = StateRecycled
	s.UpdatedAt = now
}
