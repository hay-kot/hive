// Package history defines command history domain types and interfaces.
package history

import (
	"strings"
	"time"
)

// NewOptions contains the parsed options for a "new" command.
type NewOptions struct {
	Name   string `json:"name"`
	Remote string `json:"remote,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

// Entry represents a recorded command execution.
type Entry struct {
	ID        string      `json:"id"`
	Command   string      `json:"command"`
	Args      []string    `json:"args"`               // Raw args for display
	Options   *NewOptions `json:"options,omitempty"`  // Parsed options for replay
	ExitCode  int         `json:"exit_code"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// Failed returns true if the command exited with a non-zero exit code.
func (e *Entry) Failed() bool {
	return e.ExitCode != 0
}

// CommandString returns the full command string (command + args).
func (e *Entry) CommandString() string {
	if len(e.Args) == 0 {
		return e.Command
	}
	return e.Command + " " + strings.Join(e.Args, " ")
}
