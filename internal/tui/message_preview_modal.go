package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	lipglossv1 "github.com/charmbracelet/lipgloss"
	lipgloss "github.com/charmbracelet/lipgloss/v2"
	"github.com/hay-kot/hive/internal/core/messaging"
)

// Message preview modal layout constants.
const (
	previewModalMaxWidth  = 100 // maximum modal width in columns
	previewModalMaxHeight = 30  // maximum modal height in rows
	previewModalMargin    = 4   // margin from screen edges
	previewModalChrome    = 8   // rows for title, metadata, help, and spacing
	previewModalPadding   = 4   // padding inside content area
)

// MessagePreviewModal displays a message with markdown rendering.
type MessagePreviewModal struct {
	message    messaging.Message
	viewport   viewport.Model
	ready      bool
	copyStatus string // feedback message after copy ("Copied!" or error)
}

// NewMessagePreviewModal creates a new preview modal for the given message.
func NewMessagePreviewModal(msg messaging.Message, width, height int) MessagePreviewModal {
	modalWidth := min(width-previewModalMargin, previewModalMaxWidth)
	modalHeight := min(height-previewModalMargin, previewModalMaxHeight)
	contentHeight := modalHeight - previewModalChrome

	vp := viewport.New(modalWidth-previewModalPadding, contentHeight)
	vp.Style = lipglossv1.NewStyle()

	m := MessagePreviewModal{
		message:  msg,
		viewport: vp,
		ready:    false,
	}

	// Render markdown content
	m.renderContent(modalWidth - previewModalPadding)

	return m
}

// renderContent renders the message payload as markdown.
func (m *MessagePreviewModal) renderContent(width int) {
	// Use tokyo-night style but with no document margin
	style := styles.TokyoNightStyleConfig
	noMargin := uint(0)
	style.Document.Margin = &noMargin

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		m.viewport.SetContent(m.message.Payload)
		m.ready = true
		return
	}

	rendered, err := renderer.Render(m.message.Payload)
	if err != nil {
		m.viewport.SetContent(m.message.Payload)
		m.ready = true
		return
	}

	// Trim whitespace and glamour's decorative margins
	content := strings.TrimSpace(rendered)
	// Glamour adds a decorative rule at the start - strip lines that are only
	// horizontal rules (accounting for ANSI escape codes)
	content = stripLeadingDecorative(content)
	content = stripTrailingDecorative(content)
	m.viewport.SetContent(content)
	m.ready = true
}

// UpdateViewport updates the viewport with a message (for scrolling).
func (m *MessagePreviewModal) UpdateViewport(msg any) {
	m.viewport, _ = m.viewport.Update(msg)
}

// ScrollUp scrolls the viewport up.
func (m *MessagePreviewModal) ScrollUp() {
	m.viewport.ScrollUp(1)
}

// ScrollDown scrolls the viewport down.
func (m *MessagePreviewModal) ScrollDown() {
	m.viewport.ScrollDown(1)
}

// Payload returns the raw message payload for copying.
func (m *MessagePreviewModal) Payload() string {
	return m.message.Payload
}

// SetCopyStatus sets the copy feedback message.
func (m *MessagePreviewModal) SetCopyStatus(status string) {
	m.copyStatus = status
}

// ClearCopyStatus clears the copy feedback message.
func (m *MessagePreviewModal) ClearCopyStatus() {
	m.copyStatus = ""
}

// Overlay renders the preview modal centered over the background.
func (m MessagePreviewModal) Overlay(background string, width, height int) string {
	modalWidth := min(width-previewModalMargin, previewModalMaxWidth)
	modalHeight := min(height-previewModalMargin, previewModalMaxHeight)

	// Build metadata header
	sender := m.message.Sender
	if sender == "" {
		sender = "unknown"
	}
	topicStr := previewTopicStyle.Render(fmt.Sprintf("[%s]", m.message.Topic))
	senderStr := previewSenderStyle.Render(sender)
	timeStr := previewTimeStyle.Render(m.message.CreatedAt.Format("2006-01-02 15:04:05"))
	metadata := fmt.Sprintf("%s %s %s %s", topicStr, senderStr, iconDot, timeStr)

	// Add session ID if present
	if m.message.SessionID != "" {
		sessionStr := previewSessionStyle.Render(fmt.Sprintf("session: %s", m.message.SessionID))
		metadata = fmt.Sprintf("%s\n%s", metadata, sessionStr)
	}

	// Build scroll indicator
	scrollInfo := ""
	if m.viewport.TotalLineCount() > m.viewport.VisibleLineCount() {
		scrollInfo = previewScrollStyle.Render(fmt.Sprintf(" (%.0f%%)", m.viewport.ScrollPercent()*100))
	}

	// Build help line with copy status
	helpText := "[↑/↓/j/k] scroll  [c] copy  [enter/esc] close"
	if m.copyStatus != "" {
		helpText = previewCopiedStyle.Render(m.copyStatus)
	}

	// Assemble modal content
	divider := previewDividerStyle.Render("────────────────────────────────────────")
	modalContent := lipgloss.JoinVertical(
		lipgloss.Left,
		modalTitleStyle.Render("Message Preview"+scrollInfo),
		"",
		metadata,
		divider,
		m.viewport.View(),
		modalHelpStyle.Render(helpText),
	)

	modal := modalStyle.
		Width(modalWidth).
		Height(modalHeight).
		Render(modalContent)

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		modal,
	)
}

// Preview modal specific styles.
var (
	previewTopicStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7aa2f7")).
				Bold(true)

	previewSenderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ece6a"))

	previewTimeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565f89"))

	previewSessionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565f89")).
				Italic(true)

	previewDividerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#3b4261"))

	previewScrollStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565f89"))

	previewCopiedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ece6a")) // green for success
)

// ansiPattern matches ANSI escape sequences.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// isDecorativeLine checks if a line contains only decorative characters
// (horizontal rules, spaces) after stripping ANSI codes.
func isDecorativeLine(line string) bool {
	stripped := ansiPattern.ReplaceAllString(line, "")
	stripped = strings.TrimSpace(stripped)
	if stripped == "" {
		return true
	}
	// Check if it's only horizontal rule characters
	for _, r := range stripped {
		if r != '─' && r != '━' && r != '-' && r != '=' {
			return false
		}
	}
	return true
}

// stripLeadingDecorative removes leading decorative lines from content.
func stripLeadingDecorative(content string) string {
	lines := strings.Split(content, "\n")
	start := 0
	for start < len(lines) && isDecorativeLine(lines[start]) {
		start++
	}
	if start > 0 {
		return strings.Join(lines[start:], "\n")
	}
	return content
}

// stripTrailingDecorative removes trailing decorative lines from content.
func stripTrailingDecorative(content string) string {
	lines := strings.Split(content, "\n")
	end := len(lines)
	for end > 0 && isDecorativeLine(lines[end-1]) {
		end--
	}
	if end < len(lines) {
		return strings.Join(lines[:end], "\n")
	}
	return content
}
