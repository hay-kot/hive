package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hay-kot/hive/internal/core/messaging"
)

// ActivityView displays recent messaging activity (pub/sub events).
type ActivityView struct {
	activities []messaging.Activity
	cursor     int
	width      int
	height     int
	offset     int
}

// NewActivityView creates a new activity view.
func NewActivityView() *ActivityView {
	return &ActivityView{}
}

// SetActivities sets the activities to display.
func (v *ActivityView) SetActivities(activities []messaging.Activity) {
	v.activities = activities
	// Reset cursor if out of bounds
	if v.cursor >= len(v.activities) {
		v.cursor = 0
	}
	v.clampOffset()
}

// SetSize sets the viewport dimensions.
func (v *ActivityView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.clampOffset()
}

// visibleLines returns the number of visible activity lines.
func (v *ActivityView) visibleLines() int {
	// Reserve lines for: column header (1), help (1)
	reserved := 2
	visible := v.height - reserved
	if visible < 1 {
		visible = 1
	}
	return visible
}

// clampOffset ensures the offset keeps the cursor visible.
func (v *ActivityView) clampOffset() {
	visible := v.visibleLines()
	total := len(v.activities)

	if v.cursor < v.offset {
		v.offset = v.cursor
	} else if v.cursor >= v.offset+visible {
		v.offset = v.cursor - visible + 1
	}

	if v.offset < 0 {
		v.offset = 0
	}
	maxOffset := total - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if v.offset > maxOffset {
		v.offset = maxOffset
	}
}

// MoveUp moves cursor up.
func (v *ActivityView) MoveUp() {
	if v.cursor > 0 {
		v.cursor--
		v.clampOffset()
	}
}

// MoveDown moves cursor down.
func (v *ActivityView) MoveDown() {
	if v.cursor < len(v.activities)-1 {
		v.cursor++
		v.clampOffset()
	}
}

// View renders the activity view.
func (v *ActivityView) View() string {
	var b strings.Builder

	// Column widths
	timeWidth := 8     // "14:32:01"
	typeWidth := 4     // "pub" or "sub"
	topicWidth := 20   // topic name
	sessionWidth := 12 // session ID (truncated)
	senderWidth := 16  // sender name
	padding := 5

	// Adjust topic width based on available space
	availWidth := v.width - timeWidth - typeWidth - sessionWidth - senderWidth - padding - 4
	if availWidth > 10 {
		topicWidth = availWidth
	}

	// Column headers
	headerStyle := lipgloss.NewStyle().Foreground(colorGray)
	timeHeader := fmt.Sprintf("%-*s", timeWidth, "Time")
	typeHeader := fmt.Sprintf("%-*s", typeWidth, "Type")
	topicHeader := fmt.Sprintf("%-*s", topicWidth, "Topic")
	sessionHeader := fmt.Sprintf("%-*s", sessionWidth, "Session")
	senderHeader := fmt.Sprintf("%*s", senderWidth, "Sender")
	b.WriteString("  ")
	b.WriteString(headerStyle.Render(timeHeader + " " + typeHeader + " " + topicHeader + " " + sessionHeader + senderHeader))
	b.WriteString("\n")

	linesRendered := 0

	if len(v.activities) == 0 {
		noMsg := lipgloss.NewStyle().Foreground(colorGray).Render("  No activity yet")
		b.WriteString(noMsg)
		b.WriteString("\n")
		linesRendered = 1
	} else {
		visible := v.visibleLines()
		end := v.offset + visible
		if end > len(v.activities) {
			end = len(v.activities)
		}

		for i := v.offset; i < end; i++ {
			activity := &v.activities[i]
			isSelected := i == v.cursor
			line := v.renderActivityLine(activity, isSelected, timeWidth, typeWidth, topicWidth, sessionWidth, senderWidth)
			b.WriteString(line)
			b.WriteString("\n")
			linesRendered++
		}
	}

	// Pad to push help to bottom
	visible := v.visibleLines()
	for i := linesRendered; i < visible; i++ {
		b.WriteString("\n")
	}

	// Help line
	help := lipgloss.NewStyle().Foreground(colorGray).PaddingLeft(1).Render("↑/↓ navigate • tab switch view")
	b.WriteString(help)

	return b.String()
}

// renderActivityLine renders a single activity line.
func (v *ActivityView) renderActivityLine(activity *messaging.Activity, selected bool, timeW, typeW, topicW, sessionW, senderW int) string {
	var b strings.Builder

	// Selection indicator
	if selected {
		b.WriteString(selectedBorderStyle.Render("┃"))
		b.WriteString(" ")
	} else {
		b.WriteString("  ")
	}

	// Timestamp
	timeStr := activity.Timestamp.Format("15:04:05")
	timeStyle := lipgloss.NewStyle().Foreground(colorGray)
	b.WriteString(timeStyle.Render(timeStr))
	b.WriteString(" ")

	// Type (pub/sub) with color
	typeStr := "pub"
	typeColor := colorGreen
	if activity.Type == messaging.ActivitySubscribe {
		typeStr = "sub"
		typeColor = colorBlue
	}
	typeStyle := lipgloss.NewStyle().Foreground(typeColor).Bold(true)
	typePadded := fmt.Sprintf("%-*s", typeW, typeStr)
	b.WriteString(typeStyle.Render(typePadded))
	b.WriteString(" ")

	// Topic
	topic := activity.Topic
	if len(topic) > topicW {
		topic = topic[:topicW-1] + "…"
	}
	topicColor := ColorForString(activity.Topic)
	topicStyle := lipgloss.NewStyle().Foreground(topicColor)
	topicPadded := fmt.Sprintf("%-*s", topicW, topic)
	b.WriteString(topicStyle.Render(topicPadded))
	b.WriteString(" ")

	// Session ID (truncated)
	sessionID := activity.SessionID
	if sessionID == "" {
		sessionID = "-"
	} else if len(sessionID) > sessionW {
		sessionID = sessionID[:sessionW-1] + "…"
	}
	sessionStyle := lipgloss.NewStyle().Foreground(colorPurple)
	sessionPadded := fmt.Sprintf("%-*s", sessionW, sessionID)
	b.WriteString(sessionStyle.Render(sessionPadded))

	// Sender (right-aligned)
	sender := activity.Sender
	if sender == "" {
		sender = "-"
	}
	if len(sender) > senderW {
		sender = sender[:senderW-1] + "…"
	}
	senderColor := ColorForString(sender)
	senderStyle := lipgloss.NewStyle().Foreground(senderColor)
	senderPadded := fmt.Sprintf("%*s", senderW, sender)
	b.WriteString(senderStyle.Render(senderPadded))

	return b.String()
}
