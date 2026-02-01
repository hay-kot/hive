// Package terminal provides interfaces for terminal multiplexer integrations.
package terminal

import "context"

// Status represents the detected state of a terminal session.
type Status string

const (
	StatusActive   Status = "active"   // agent is actively working (spinner/busy indicator)
	StatusApproval Status = "approval" // agent needs permission (Yes/No dialog)
	StatusReady    Status = "ready"    // agent finished, waiting for next input (❯ prompt)
	StatusMissing  Status = "missing"  // terminal session not found
)

// SessionInfo holds information about a discovered terminal session.
type SessionInfo struct {
	Name         string // terminal session name (e.g., tmux session name)
	Pane         string // pane identifier if applicable
	Status       Status // current detected status
	DetectedTool string // detected AI tool (claude, gemini, etc.)
}

// Integration defines the interface for terminal multiplexer integrations.
type Integration interface {
	// Name returns the integration name (e.g., "tmux").
	Name() string

	// Available returns true if this integration is usable (e.g., tmux is installed).
	Available() bool

	// RefreshCache updates cached session data. Call once per poll cycle
	// to batch tmux queries efficiently.
	RefreshCache()

	// DiscoverSession finds a terminal session for the given slug and metadata.
	// Returns nil if no matching session is found.
	DiscoverSession(ctx context.Context, slug string, metadata map[string]string) (*SessionInfo, error)

	// GetStatus returns the current status of a previously discovered session.
	GetStatus(ctx context.Context, info *SessionInfo) (Status, error)
}
