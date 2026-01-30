package messaging

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/hay-kot/hive/internal/core/session"
)

// SessionDetector finds the current session from the working directory.
type SessionDetector struct {
	store session.Store
}

// NewSessionDetector creates a new session detector.
func NewSessionDetector(store session.Store) *SessionDetector {
	return &SessionDetector{store: store}
}

// DetectSession returns the session ID for the current working directory.
// Returns empty string if not in a hive session.
func (d *SessionDetector) DetectSession(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil // Not an error - just can't detect
	}
	return d.DetectSessionFromPath(ctx, cwd)
}

// DetectSessionFromPath returns the session ID for the given path.
// Returns empty string if the path is not within a hive session.
func (d *SessionDetector) DetectSessionFromPath(ctx context.Context, path string) (string, error) {
	sessions, err := d.store.List(ctx)
	if err != nil {
		return "", nil // Not an error - just can't detect
	}

	// Clean and normalize the path
	path, err = filepath.Abs(path)
	if err != nil {
		return "", nil
	}
	path = filepath.Clean(path)

	// Find the longest matching session path (most specific match)
	var bestMatch session.Session
	var bestMatchLen int

	for _, sess := range sessions {
		if sess.State != session.StateActive {
			continue
		}

		sessPath := filepath.Clean(sess.Path)

		// Check if path equals or is within the session path
		if path == sessPath || isSubpath(sessPath, path) {
			if len(sessPath) > bestMatchLen {
				bestMatch = sess
				bestMatchLen = len(sessPath)
			}
		}
	}

	return bestMatch.ID, nil
}

// isSubpath returns true if child is a subdirectory of parent.
func isSubpath(parent, child string) bool {
	// Ensure parent ends with separator for correct prefix matching
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent += string(filepath.Separator)
	}
	return strings.HasPrefix(child, parent)
}
