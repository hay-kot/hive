// Package validate provides shared validation functions.
package validate

import (
	"fmt"
	"strings"
)

// SessionName validates a session name is non-empty after trimming whitespace.
func SessionName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

// SessionID validates a session ID follows the expected format:
// - Non-empty
// - Lowercase alphanumeric only (a-z, 0-9)
// - No spaces or special characters
func SessionID(id string) error {
	if id == "" {
		return fmt.Errorf("session ID is required")
	}
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return fmt.Errorf("session ID must be lowercase alphanumeric only, got %q", id)
		}
	}
	return nil
}
