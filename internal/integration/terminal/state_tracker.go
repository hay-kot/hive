package terminal

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
)

// StateTracker tracks terminal activity state across poll cycles.
// Implements spike detection to filter cursor blinks and terminal redraws.
//
// Three-state model:
//   - GREEN (active)  = Explicit busy indicator found (spinner, "ctrl+c to interrupt")
//   - YELLOW (waiting) = Prompt detected, needs user input
//   - GRAY (idle)     = No busy indicator, no prompt, AND user has acknowledged
type StateTracker struct {
	// Content tracking
	lastHash       string    // SHA256 of normalized content
	lastChangeTime time.Time // When sustained activity was last confirmed

	// Acknowledge tracking: distinguishes idle (user saw) from waiting (needs attention)
	acknowledged   bool      // User has seen this state (yellow vs gray)
	acknowledgedAt time.Time // When acknowledged was set (for grace period)
	waitingSince   time.Time // When session transitioned to waiting status

	// Activity timestamp tracking (from tmux window_activity)
	lastActivityTimestamp int64 // Previous activity timestamp

	// Spike detection: track activity changes across poll cycles
	// Requires 2+ timestamp changes within 1 second to confirm sustained activity
	activityCheckStart  time.Time // When we started tracking for sustained activity
	activityChangeCount int       // How many timestamp changes seen in current window

	// Last stable status (returned during spike detection window)
	lastStableStatus Status
}

// SpikeWindow is how long we wait to confirm sustained activity.
const SpikeWindow = 1 * time.Second

// NewStateTracker creates a new state tracker with initial acknowledged state.
// If acknowledged is true, starts as idle. Otherwise starts as waiting.
func NewStateTracker(acknowledged bool) *StateTracker {
	st := &StateTracker{
		acknowledged:   acknowledged,
		acknowledgedAt: time.Now(),
	}
	if acknowledged {
		st.lastStableStatus = StatusIdle
	} else {
		st.lastStableStatus = StatusWaiting
		st.waitingSince = time.Now()
	}
	return st
}

// Acknowledge marks the session as seen by the user.
// Transitions waiting → idle.
func (st *StateTracker) Acknowledge() {
	st.acknowledged = true
	st.acknowledgedAt = time.Now()
	st.lastStableStatus = StatusIdle
}

// ResetAcknowledged marks the session as needing attention.
// Called when new activity detected or prompt appears.
// Transitions idle → waiting.
func (st *StateTracker) ResetAcknowledged() {
	st.acknowledged = false
	st.waitingSince = time.Now()
	st.lastStableStatus = StatusWaiting
}

// IsAcknowledged returns whether the user has seen the current state.
func (st *StateTracker) IsAcknowledged() bool {
	return st.acknowledged
}

// WaitingSince returns when the session became waiting.
func (st *StateTracker) WaitingSince() time.Time {
	return st.waitingSince
}

// Update processes new activity data and returns the detected status.
// content is the terminal content (for busy/prompt detection).
// activityTS is the tmux window_activity timestamp.
// detector is used to check busy/waiting patterns.
func (st *StateTracker) Update(content string, activityTS int64, detector *Detector) Status {
	now := time.Now()

	// Check for explicit busy indicator (most reliable)
	isBusy := detector.IsBusy(content)
	isWaiting := detector.IsWaiting(content)

	// Explicit busy indicator = definitely active
	// Reset acknowledged since new activity detected
	if isBusy && !isWaiting {
		st.lastChangeTime = now
		st.acknowledged = false
		st.lastStableStatus = StatusActive
		st.resetSpikeDetection()
		return StatusActive
	}

	// Prompt takes priority over busy (Claude can show spinner with question UI)
	if isWaiting {
		// Reset acknowledged - needs user attention
		st.acknowledged = false
		if st.lastStableStatus != StatusWaiting {
			st.waitingSince = now
		}
		st.lastStableStatus = StatusWaiting
		st.resetSpikeDetection()
		return StatusWaiting
	}

	// No explicit indicators - use spike detection on activity timestamp
	if st.lastActivityTimestamp == 0 {
		// First poll - initialize and return waiting (not idle) until acknowledged
		st.lastActivityTimestamp = activityTS
		if st.acknowledged {
			st.lastStableStatus = StatusIdle
			return StatusIdle
		}
		st.lastStableStatus = StatusWaiting
		st.waitingSince = now
		return StatusWaiting
	}

	// Activity timestamp changed
	if st.lastActivityTimestamp != activityTS {
		st.lastActivityTimestamp = activityTS

		// Check if we're in a detection window
		if st.activityCheckStart.IsZero() || now.Sub(st.activityCheckStart) > SpikeWindow {
			// Start new detection window
			st.activityCheckStart = now
			st.activityChangeCount = 1
		} else {
			// Within detection window - count this change
			st.activityChangeCount++

			// 2+ changes within 1 second = potential sustained activity
			// BUT we must confirm with content check
			if st.activityChangeCount >= 2 {
				// Confirmed sustained activity - but still need busy indicator
				// Content hash changes alone are NOT reliable (cursor blinks, status bar updates)
				// Only go green if we also detect busy indicator
				if isBusy {
					st.lastChangeTime = now
					st.acknowledged = false
					st.lastStableStatus = StatusActive
					st.resetSpikeDetection()
					return StatusActive
				}
				// No busy indicator - spike was false positive
				st.resetSpikeDetection()
			}
		}

		// Not enough changes yet or no busy indicator - keep previous status
		return st.lastStableStatus
	}

	// No timestamp change
	// Check if spike window expired with only 1 change (filter single spike)
	if st.activityChangeCount == 1 && !st.activityCheckStart.IsZero() {
		if now.Sub(st.activityCheckStart) > SpikeWindow {
			st.resetSpikeDetection()
		}
	}

	// During spike detection window, keep previous stable status
	if !st.activityCheckStart.IsZero() && now.Sub(st.activityCheckStart) < SpikeWindow {
		return st.lastStableStatus
	}

	// No activity, no busy indicator, no prompt
	// Return idle only if acknowledged, otherwise waiting
	if st.acknowledged {
		st.lastStableStatus = StatusIdle
		return StatusIdle
	}

	// Not acknowledged = still needs attention
	if st.lastStableStatus != StatusWaiting {
		st.waitingSince = now
	}
	st.lastStableStatus = StatusWaiting
	return StatusWaiting
}

// resetSpikeDetection clears the spike detection window.
func (st *StateTracker) resetSpikeDetection() {
	st.activityCheckStart = time.Time{}
	st.activityChangeCount = 0
}

// UpdateHash updates the content hash and returns true if content changed.
func (st *StateTracker) UpdateHash(content string) bool {
	normalized := NormalizeContent(content)
	hash := HashContent(normalized)
	if hash == st.lastHash {
		return false
	}
	st.lastHash = hash
	return true
}

// spinnerRunes are characters stripped during content normalization.
var spinnerRunes = []rune{
	'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏', // braille
	'·', '✳', '✽', '✶', '✻', '✢', // asterisk spinners
}

// Patterns for normalizing dynamic content.
var (
	// Dynamic status counters: "(45s · 1234 tokens · ctrl+c to interrupt)" or "(35s · ↑ 673 tokens)"
	dynamicStatusPattern = regexp.MustCompile(`\([^)]*\d+s\s*·[^)]*(?:tokens|↑|↓)[^)]*\)`)

	// Progress bar patterns: [====>   ] 45%
	progressBarPattern = regexp.MustCompile(`\[=*>?\s*\]\s*\d+%`)

	// Time patterns like 12:34 or 12:34:56
	timePattern = regexp.MustCompile(`\b\d{1,2}:\d{2}(:\d{2})?\b`)

	// Progress percentages like 45%
	percentagePattern = regexp.MustCompile(`\b\d{1,3}%`)

	// Download progress like 1.2MB/5.6MB
	downloadPattern = regexp.MustCompile(`\d+(\.\d+)?[KMGT]?B/\d+(\.\d+)?[KMGT]?B`)

	// Multiple blank lines
	blankLinesPattern = regexp.MustCompile(`\n{3,}`)

	// Thinking pattern with spinner + ellipsis + status: "✳ Gusting… (35s · ↑ 673 tokens)"
	thinkingPatternEllipsis = regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏·✳✽✶✻✢]\s*.+…\s*\([^)]*\)`)
)

// NormalizeContent prepares content for hashing by removing dynamic elements.
// This prevents false hash changes from animations and counters.
func NormalizeContent(content string) string {
	result := stripANSI(content)

	// Strip control characters (keep tab, newline, carriage return)
	result = stripControlChars(result)

	// Strip spinner characters that animate
	for _, r := range spinnerRunes {
		result = strings.ReplaceAll(result, string(r), "")
	}

	// Normalize Claude Code dynamic status: "(45s · 1234 tokens)" → "(STATUS)"
	result = dynamicStatusPattern.ReplaceAllString(result, "(STATUS)")

	// Normalize thinking spinner patterns: "✳ Gusting… (35s · ↑ 673 tokens)" → "THINKING…"
	result = thinkingPatternEllipsis.ReplaceAllString(result, "THINKING…")

	// Normalize progress indicators
	result = progressBarPattern.ReplaceAllString(result, "[PROGRESS]")
	result = downloadPattern.ReplaceAllString(result, "X.XMB/Y.YMB")
	result = percentagePattern.ReplaceAllString(result, "N%")

	// Normalize time patterns that change every second
	result = timePattern.ReplaceAllString(result, "HH:MM:SS")

	// Trim trailing whitespace per line (fixes resize false positives)
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	result = strings.Join(lines, "\n")

	// Collapse multiple blank lines
	result = blankLinesPattern.ReplaceAllString(result, "\n\n")

	return result
}

// stripControlChars removes ASCII control characters except tab, newline, CR.
func stripControlChars(content string) string {
	var result strings.Builder
	result.Grow(len(content))
	for _, r := range content {
		if (r >= 32 && r != 127) || r == '\t' || r == '\n' || r == '\r' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// HashContent generates SHA256 hash of content.
func HashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
