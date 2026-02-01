package tui

import (
	"fmt"
	"strings"
	"time"

	lipgloss "github.com/charmbracelet/lipgloss/v2"
	"github.com/hay-kot/hive/internal/core/messaging"
)

// MessagesView is a custom compact renderer for messages.
// It displays messages in a single-line format:
// timestamp [topic    ] message_preview...               sender
type MessagesView struct {
	messages   []messaging.Message
	cursor     int
	width      int
	height     int
	offset     int // scroll offset for viewport
	filtering  bool
	filter     string
	filterBuf  strings.Builder
	filteredAt []int // indices of messages matching filter
}

// NewMessagesView creates a new messages view.
func NewMessagesView() *MessagesView {
	return &MessagesView{
		filteredAt: make([]int, 0),
	}
}

// SetMessages sets the messages to display.
func (v *MessagesView) SetMessages(msgs []messaging.Message) {
	v.messages = msgs
	v.applyFilter()
	// Reset cursor if out of bounds
	if len(v.filteredAt) == 0 {
		v.cursor = 0
	} else if v.cursor >= len(v.filteredAt) {
		v.cursor = len(v.filteredAt) - 1
	}
	v.clampOffset()
}

// SetSize sets the viewport dimensions.
func (v *MessagesView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.clampOffset()
}

// visibleLines returns the number of visible message lines.
func (v *MessagesView) visibleLines() int {
	// Reserve lines for: column header (1), help (1)
	reserved := 2
	// Add filter line if active
	if v.filtering || v.filter != "" {
		reserved++
	}
	visible := v.height - reserved
	if visible < 1 {
		visible = 1
	}
	return visible
}

// clampOffset ensures the offset keeps the cursor visible.
func (v *MessagesView) clampOffset() {
	visible := v.visibleLines()
	total := len(v.filteredAt)

	// Ensure cursor is visible
	if v.cursor < v.offset {
		v.offset = v.cursor
	} else if v.cursor >= v.offset+visible {
		v.offset = v.cursor - visible + 1
	}

	// Clamp offset to valid range
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
func (v *MessagesView) MoveUp() {
	if v.cursor > 0 {
		v.cursor--
		v.clampOffset()
	}
}

// MoveDown moves cursor down.
func (v *MessagesView) MoveDown() {
	if v.cursor < len(v.filteredAt)-1 {
		v.cursor++
		v.clampOffset()
	}
}

// SelectedMessage returns the currently selected message, or nil if none.
func (v *MessagesView) SelectedMessage() *messaging.Message {
	if len(v.filteredAt) == 0 || v.cursor >= len(v.filteredAt) {
		return nil
	}
	idx := v.filteredAt[v.cursor]
	if idx >= len(v.messages) {
		return nil
	}
	return &v.messages[idx]
}

// StartFilter begins filter input mode.
func (v *MessagesView) StartFilter() {
	v.filtering = true
	v.filterBuf.Reset()
}

// StopFilter ends filter input mode.
func (v *MessagesView) StopFilter() {
	v.filtering = false
}

// CancelFilter cancels filtering and clears the filter.
func (v *MessagesView) CancelFilter() {
	v.filtering = false
	v.filter = ""
	v.filterBuf.Reset()
	v.applyFilter()
}

// IsFiltering returns true if filter input is active.
func (v *MessagesView) IsFiltering() bool {
	return v.filtering
}

// AddFilterRune adds a rune to the filter.
func (v *MessagesView) AddFilterRune(r rune) {
	v.filterBuf.WriteRune(r)
	v.filter = v.filterBuf.String()
	v.applyFilter()
}

// DeleteFilterRune removes the last rune from the filter.
func (v *MessagesView) DeleteFilterRune() {
	s := v.filterBuf.String()
	if len(s) > 0 {
		s = s[:len(s)-1]
		v.filterBuf.Reset()
		v.filterBuf.WriteString(s)
		v.filter = s
		v.applyFilter()
	}
}

// ConfirmFilter confirms the filter and exits filter mode.
func (v *MessagesView) ConfirmFilter() {
	v.filtering = false
	v.applyFilter()
}

// applyFilter updates filteredAt based on current filter.
func (v *MessagesView) applyFilter() {
	v.filteredAt = v.filteredAt[:0]
	filter := strings.ToLower(v.filter)

	for i := range v.messages {
		if filter == "" || v.matchesFilter(&v.messages[i], filter) {
			v.filteredAt = append(v.filteredAt, i)
		}
	}

	// Reset cursor if out of bounds
	if v.cursor >= len(v.filteredAt) {
		v.cursor = 0
	}
	v.clampOffset()
}

// matchesFilter checks if a message matches the filter.
func (v *MessagesView) matchesFilter(msg *messaging.Message, filter string) bool {
	return strings.Contains(strings.ToLower(msg.Topic), filter) ||
		strings.Contains(strings.ToLower(msg.Sender), filter) ||
		strings.Contains(strings.ToLower(msg.Payload), filter)
}

// View renders the messages view.
func (v *MessagesView) View() string {
	var b strings.Builder

	// Column widths (defined early for header and content)
	// Order: Time | Sender | Topic | Message | Age
	timeWidth := 8    // "14:32:01"
	senderWidth := 14 // "agent.XXXX" format
	topicWidth := 14  // topic name
	ageWidth := 4     // "2m", "1h", "3d"
	padding := 5      // spaces between columns
	contentWidth := v.width - timeWidth - senderWidth - topicWidth - ageWidth - padding - 4

	if contentWidth < 20 {
		contentWidth = 20
	}

	// Filter line (only shown when filtering or filter is active)
	if v.filtering {
		filterPrompt := lipgloss.NewStyle().Foreground(colorBlue).Bold(true).Render("Filter: ")
		b.WriteString(" ")
		b.WriteString(filterPrompt)
		b.WriteString(v.filter)
		b.WriteString("▎") // cursor
		b.WriteString("\n")
	} else if v.filter != "" {
		filterShow := lipgloss.NewStyle().Foreground(colorGray).Render(fmt.Sprintf("Filter: %s", v.filter))
		b.WriteString(" ")
		b.WriteString(filterShow)
		b.WriteString("\n")
	}

	// Column headers (Time | Sender | Topic | Message | Age)
	headerStyle := lipgloss.NewStyle().Foreground(colorGray)
	timeHeader := fmt.Sprintf("%-*s", timeWidth, "Time")
	senderHeader := fmt.Sprintf("%-*s", senderWidth, "Sender")
	topicHeader := fmt.Sprintf("%-*s", topicWidth, "Topic")
	msgHeader := fmt.Sprintf("%-*s", contentWidth, "Message")
	ageHeader := fmt.Sprintf("%*s", ageWidth, "Age")
	b.WriteString("  ") // align with content (selection indicator space)
	b.WriteString(headerStyle.Render(timeHeader + " " + senderHeader + " " + topicHeader + " " + msgHeader + " " + ageHeader))
	b.WriteString("\n")

	// Track lines rendered for padding calculation
	linesRendered := 0

	// No messages
	if len(v.filteredAt) == 0 {
		if len(v.messages) == 0 {
			noMsg := lipgloss.NewStyle().Foreground(colorGray).Render("  No messages")
			b.WriteString(noMsg)
			b.WriteString("\n")
		} else {
			noMatch := lipgloss.NewStyle().Foreground(colorGray).Render("  No matching messages")
			b.WriteString(noMatch)
			b.WriteString("\n")
		}
		linesRendered = 1
	} else {
		// Render visible messages
		visible := v.visibleLines()
		end := v.offset + visible
		if end > len(v.filteredAt) {
			end = len(v.filteredAt)
		}

		for i := v.offset; i < end; i++ {
			msgIdx := v.filteredAt[i]
			msg := &v.messages[msgIdx]
			isSelected := i == v.cursor

			line := v.renderMessageLine(msg, isSelected, timeWidth, senderWidth, topicWidth, contentWidth, ageWidth)
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

	// Help line (pinned to bottom, styled to match sessions view)
	help := lipgloss.NewStyle().Foreground(colorGray).PaddingLeft(1).Render("↑/↓ navigate • enter preview • / filter • tab switch view")
	b.WriteString(help)

	return b.String()
}

// renderMessageLine renders a single message line in compact format.
// Format: timestamp [sender] [topic] message_preview... age
func (v *MessagesView) renderMessageLine(msg *messaging.Message, selected bool, _, senderW, topicW, contentW, ageW int) string {
	var b strings.Builder

	// Selection indicator
	if selected {
		b.WriteString(selectedBorderStyle.Render("┃"))
		b.WriteString(" ")
	} else {
		b.WriteString("  ")
	}

	// Timestamp
	timeStr := msg.CreatedAt.Format("15:04:05")
	timeStyle := lipgloss.NewStyle().Foreground(colorGray)
	b.WriteString(timeStyle.Render(timeStr))
	b.WriteString(" ")

	// Sender (with color hashing, fixed width, in brackets)
	sender := msg.Sender
	if sender == "" {
		sender = "unknown"
	}
	if len(sender) > senderW-2 { // -2 for brackets
		sender = sender[:senderW-3] + "…"
	}
	senderColor := ColorForString(sender)
	senderStyle := lipgloss.NewStyle().Foreground(senderColor)
	senderPadded := fmt.Sprintf("[%-*s]", senderW-2, sender)
	b.WriteString(senderStyle.Render(senderPadded))
	b.WriteString(" ")

	// Topic (with color hashing, fixed width, in brackets)
	topicColor := ColorForString(msg.Topic)
	topicStyle := lipgloss.NewStyle().Foreground(topicColor)
	topic := msg.Topic
	if len(topic) > topicW-2 { // -2 for brackets
		topic = topic[:topicW-3] + "…"
	}
	topicPadded := fmt.Sprintf("[%-*s]", topicW-2, topic)
	b.WriteString(topicStyle.Render(topicPadded))
	b.WriteString(" ")

	// Message preview (truncated, fills remaining space)
	payload := strings.ReplaceAll(msg.Payload, "\n", " ")
	payload = strings.ReplaceAll(payload, "\t", " ")
	payloadRunes := []rune(payload)
	if len(payloadRunes) > contentW-1 {
		payload = string(payloadRunes[:contentW-1]) + "…"
	}
	payloadStyle := lipgloss.NewStyle().Foreground(colorWhite)
	if selected {
		payloadStyle = payloadStyle.Bold(true)
	}
	// Pad payload to fill content width
	payloadPadded := fmt.Sprintf("%-*s", contentW, payload)
	b.WriteString(payloadStyle.Render(payloadPadded))
	b.WriteString(" ")

	// Age (right-aligned, provides visual end cap)
	age := formatAge(msg.CreatedAt)
	ageStyle := lipgloss.NewStyle().Foreground(colorGray)
	agePadded := fmt.Sprintf("%*s", ageW, age)
	b.WriteString(ageStyle.Render(agePadded))

	return b.String()
}

// formatAge returns a human-readable relative time string.
func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
