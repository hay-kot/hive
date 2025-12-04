package tui

import (
	lipgloss "github.com/charmbracelet/lipgloss/v2"
)

// Modal represents a confirmation dialog.
type Modal struct {
	title   string
	message string
	visible bool
}

// NewModal creates a new modal with the given title and message.
func NewModal(title, message string) Modal {
	return Modal{
		title:   title,
		message: message,
		visible: true,
	}
}

// Visible returns whether the modal should be displayed.
func (m Modal) Visible() bool {
	return m.visible
}

// Overlay renders the modal as a layer over the given background content.
func (m Modal) Overlay(background string, width, height int) string {
	if !m.visible {
		return background
	}

	// Build the modal content
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		modalTitleStyle.Render(m.title),
		"",
		m.message,
		modalHelpStyle.Render("[y] confirm  [n/esc] cancel"),
	)

	modal := modalStyle.Render(content)

	// Use lipgloss.Place to center the modal in the full screen area
	// This replaces the background completely (simpler approach)
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		modal,
	)
}
