package tui

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	lipgloss "charm.land/lipgloss/v2"
)

// Output modal layout constants.
const (
	outputModalMaxWidth   = 100 // maximum modal width in columns
	outputModalMaxHeight  = 20  // maximum modal height in rows
	outputModalMargin     = 4   // margin from screen edges
	outputModalChrome     = 6   // rows for title, status, help, and spacing
	outputModalPadding    = 4   // padding inside content area
	outputModalTruncation = 7   // space for "..." when truncating lines
	outputModalMaxLines   = 100 // max lines to buffer
)

// OutputModal displays streaming command output in a modal dialog.
type OutputModal struct {
	title    string
	lines    []string
	running  bool
	err      error
	spinner  spinner.Model
	maxLines int // max lines to keep in buffer
}

// NewOutputModal creates a new output modal with the given title.
func NewOutputModal(title string) OutputModal {
	s := spinner.New()
	s.Spinner = spinner.Dot

	return OutputModal{
		title:    title,
		lines:    make([]string, 0),
		running:  true,
		spinner:  s,
		maxLines: outputModalMaxLines,
	}
}

// AddLine appends a line of output to the modal.
func (m *OutputModal) AddLine(line string) {
	// Split on newlines in case multiple lines come at once
	newLines := strings.Split(strings.TrimRight(line, "\n"), "\n")
	m.lines = append(m.lines, newLines...)

	// Trim to max lines
	if len(m.lines) > m.maxLines {
		m.lines = m.lines[len(m.lines)-m.maxLines:]
	}
}

// SetComplete marks the modal as complete with optional error.
func (m *OutputModal) SetComplete(err error) {
	m.running = false
	m.err = err
}

// IsRunning returns true if the command is still running.
func (m *OutputModal) IsRunning() bool {
	return m.running
}

// Spinner returns the spinner model for tick updates.
func (m *OutputModal) Spinner() spinner.Model {
	return m.spinner
}

// SetSpinner updates the spinner model.
func (m *OutputModal) SetSpinner(s spinner.Model) {
	m.spinner = s
}

// Overlay renders the output modal centered over the background.
func (m OutputModal) Overlay(background string, width, height int) string {
	// Calculate modal dimensions - use most of the screen
	modalWidth := min(width-outputModalMargin, outputModalMaxWidth)
	modalHeight := min(height-outputModalMargin, outputModalMaxHeight)
	contentHeight := modalHeight - outputModalChrome

	// Build content lines
	var contentBuilder strings.Builder

	// Show last N lines that fit
	startIdx := 0
	if len(m.lines) > contentHeight {
		startIdx = len(m.lines) - contentHeight
	}

	for i := startIdx; i < len(m.lines); i++ {
		line := m.lines[i]
		// Truncate long lines
		if len(line) > modalWidth-outputModalPadding {
			line = line[:modalWidth-outputModalTruncation] + "..."
		}
		contentBuilder.WriteString(line)
		if i < len(m.lines)-1 {
			contentBuilder.WriteString("\n")
		}
	}

	// Pad with empty lines if needed
	lineCount := len(m.lines) - startIdx
	for i := lineCount; i < contentHeight; i++ {
		contentBuilder.WriteString("\n")
	}

	content := contentBuilder.String()

	// Build status line
	var status string
	switch {
	case m.running:
		status = m.spinner.View() + " Running..."
	case m.err != nil:
		status = outputErrorStyle.Render("✗ Error: " + m.err.Error())
	default:
		status = outputSuccessStyle.Render("✓ Complete")
	}

	// Build help line
	var help string
	if m.running {
		help = "[esc] cancel"
	} else {
		help = "[enter/esc] close"
	}

	// Assemble modal content
	modalContent := lipgloss.JoinVertical(
		lipgloss.Left,
		modalTitleStyle.Render(m.title),
		"",
		outputContentStyle.Width(modalWidth-outputModalPadding).Render(content),
		"",
		status,
		modalHelpStyle.Render(help),
	)

	modal := modalStyle.Render(modalContent)

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

// Output modal specific styles.
var (
	outputContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#a9b1d6"))

	outputErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f38ba8"))

	outputSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ece6a"))
)
