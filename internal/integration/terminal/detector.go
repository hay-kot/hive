package terminal

import (
	"strings"
	"time"
)

// Detector detects AI tool status from terminal content.
type Detector struct {
	tool string
}

// NewDetector creates a detector for the specified tool.
func NewDetector(tool string) *Detector {
	return &Detector{tool: strings.ToLower(tool)}
}

// spinnerChars are braille spinner characters used by CLI tools.
var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// IsBusy returns true if the terminal content indicates the agent is actively working.
func (d *Detector) IsBusy(content string) bool {
	lower := strings.ToLower(content)

	// Check for explicit busy indicators
	busyIndicators := []string{
		"ctrl+c to interrupt",
		"esc to interrupt",
	}
	for _, indicator := range busyIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}

	// Check for spinner characters in last few lines
	lines := getLastNonEmptyLines(content, 5)
	for _, line := range lines {
		for _, spinner := range spinnerChars {
			if strings.Contains(line, spinner) {
				return true
			}
		}
	}

	// Check for thinking indicator with timing info
	if strings.Contains(lower, "thinking") && strings.Contains(lower, "tokens") {
		return true
	}

	return false
}

// IsWaiting returns true if the terminal content indicates the agent needs user input.
func (d *Detector) IsWaiting(content string) bool {
	// If busy, not waiting
	if d.IsBusy(content) {
		return false
	}

	lines := getLastNonEmptyLines(content, 10)
	recentContent := strings.Join(lines, "\n")

	// Permission prompts (normal mode)
	permissionPrompts := []string{
		"Yes, allow once",
		"Yes, allow always",
		"No, and tell Claude what to do differently",
		"Do you trust the files in this folder?",
		"Allow this MCP server",
		"Run this command?",
		"Use arrow keys to navigate",
		"Press Enter to select",
	}
	for _, prompt := range permissionPrompts {
		if strings.Contains(recentContent, prompt) {
			return true
		}
	}

	// Check for standalone prompt character (user input expected)
	if len(lines) > 0 {
		lastLine := strings.TrimSpace(stripANSI(lines[len(lines)-1]))
		// Replace non-breaking space with regular space
		lastLine = strings.ReplaceAll(lastLine, "\u00A0", " ")

		// Claude Code shows ">" or "❯" when waiting for input
		if lastLine == ">" || lastLine == "❯" || lastLine == "> " || lastLine == "❯ " {
			return true
		}
	}

	// Yes/No confirmation prompts
	confirmPatterns := []string{
		"(Y/n)", "[Y/n]", "(y/N)", "[y/N]",
		"(yes/no)", "[yes/no]",
		"Continue?", "Proceed?",
	}
	for _, pattern := range confirmPatterns {
		if strings.Contains(recentContent, pattern) {
			return true
		}
	}

	return false
}

// activityThreshold is how recently activity must have occurred to consider
// the terminal as potentially active (even without explicit busy indicators).
const activityThreshold = 2 * time.Second

// DetectStatus returns the detected status based on terminal content and activity.
// lastActivity is the unix timestamp of the last terminal activity.
// hasActivity indicates if activity changed since the last check.
func (d *Detector) DetectStatus(content string, lastActivity int64, hasActivity bool) Status {
	if d.IsBusy(content) {
		return StatusActive
	}
	if d.IsWaiting(content) {
		return StatusWaiting
	}

	// Check for recent activity: if the terminal was active recently and we haven't
	// detected a waiting prompt, assume the agent is still working.
	// This handles cases where text is being output without spinners/interrupt indicators.
	if lastActivity > 0 {
		activityTime := time.Unix(lastActivity, 0)
		if time.Since(activityTime) < activityThreshold {
			return StatusActive
		}
	}

	// Also consider hasActivity - if activity changed between polls, likely still working
	if hasActivity {
		return StatusActive
	}

	return StatusIdle
}

// DetectTool attempts to identify the AI tool from terminal content.
func DetectTool(content string) string {
	lower := strings.ToLower(content)

	patterns := map[string][]string{
		"claude": {
			"claude",
			"anthropic",
			"ctrl+c to interrupt",
		},
		"gemini": {
			"gemini",
			"google ai",
		},
		"opencode": {
			"opencode",
			"open code",
		},
		"codex": {
			"codex",
			"openai",
		},
	}

	for tool, keywords := range patterns {
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				return tool
			}
		}
	}

	return "shell"
}

// getLastNonEmptyLines returns the last n non-empty lines from content.
func getLastNonEmptyLines(content string, n int) []string {
	lines := strings.Split(content, "\n")
	var result []string

	for i := len(lines) - 1; i >= 0 && len(result) < n; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			result = append([]string{lines[i]}, result...)
		}
	}

	return result
}

// stripANSI removes ANSI escape codes from content.
func stripANSI(content string) string {
	// Fast path: if no escape chars, return as-is
	if !strings.Contains(content, "\x1b") && !strings.Contains(content, "\x9B") {
		return content
	}

	var b strings.Builder
	b.Grow(len(content))

	i := 0
	for i < len(content) {
		if content[i] == '\x1b' {
			// CSI sequence: ESC [ ... letter
			if i+1 < len(content) && content[i+1] == '[' {
				j := i + 2
				for j < len(content) {
					c := content[j]
					if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
						j++
						break
					}
					j++
				}
				i = j
				continue
			}
			// OSC sequence: ESC ] ... BEL
			if i+1 < len(content) && content[i+1] == ']' {
				bellPos := strings.Index(content[i:], "\x07")
				if bellPos != -1 {
					i += bellPos + 1
					continue
				}
			}
			// Other escape: skip 2 chars
			if i+1 < len(content) {
				i += 2
				continue
			}
		}
		if content[i] == '\x9B' {
			j := i + 1
			for j < len(content) {
				c := content[j]
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					j++
					break
				}
				j++
			}
			i = j
			continue
		}
		b.WriteByte(content[i])
		i++
	}

	return b.String()
}
