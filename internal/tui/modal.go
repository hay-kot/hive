package tui

import (
	lipgloss "charm.land/lipgloss/v2"
)

// Modal represents a confirmation dialog.
type Modal struct {
	title           string
	message         string
	visible         bool
	confirmSelected bool // true = confirm button selected, false = cancel button selected
}

// NewModal creates a new modal with the given title and message.
func NewModal(title, message string) Modal {
	return Modal{
		title:           title,
		message:         message,
		visible:         true,
		confirmSelected: true, // default to confirm button
	}
}

// ToggleSelection switches the selected button.
func (m *Modal) ToggleSelection() {
	m.confirmSelected = !m.confirmSelected
}

// ConfirmSelected returns true if the confirm button is selected.
func (m Modal) ConfirmSelected() bool {
	return m.confirmSelected
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

	// Render buttons with selection state
	var confirmBtn, cancelBtn string
	if m.confirmSelected {
		confirmBtn = modalButtonSelectedStyle.Render("Confirm")
		cancelBtn = modalButtonStyle.Render("Cancel")
	} else {
		confirmBtn = modalButtonStyle.Render("Confirm")
		cancelBtn = modalButtonSelectedStyle.Render("Cancel")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, confirmBtn, "  ", cancelBtn)
	buttonRow := lipgloss.NewStyle().MarginTop(1).Render(buttons)

	// Build the modal content
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		modalTitleStyle.Render(m.title),
		"",
		m.message,
		buttonRow,
		modalHelpStyle.Render("←/→ select  enter confirm  esc cancel"),
	)

	modal := modalStyle.Render(content)

	// Use Compositor/Layer for true overlay (background remains visible)
	bgLayer := lipgloss.NewLayer(background)
	modalLayer := lipgloss.NewLayer(modal)

	// Center the modal
	modalW := lipgloss.Width(modal)
	modalH := lipgloss.Height(modal)
	centerX := (width - modalW) / 2
	centerY := (height - modalH) / 2
	modalLayer.X(centerX).Y(centerY).Z(1)

	compositor := lipgloss.NewCompositor(bgLayer, modalLayer)
	return compositor.Render()
}
