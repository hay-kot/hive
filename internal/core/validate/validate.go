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
