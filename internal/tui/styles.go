// Package tui implements the Bubble Tea TUI for hive.
package tui

import (
	"github.com/charmbracelet/lipgloss"
	lipglossv2 "github.com/charmbracelet/lipgloss/v2"
)

// Colors used throughout the TUI (lipgloss v1 for bubbles compatibility).
var (
	colorGreen  = lipgloss.Color("#a6e3a1")
	colorYellow = lipgloss.Color("#f9e2af")
	colorBlue   = lipgloss.Color("#89b4fa")
	colorGray   = lipgloss.Color("#6c7086")
	colorWhite  = lipgloss.Color("#cdd6f4")
)

// Styles used for rendering the TUI (lipgloss v1 for bubbles compatibility).
var (
	// Title style for the list header.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlue).
			Padding(0, 1)

	// Active session state style.
	activeStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	// Recycled session state style.
	recycledStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	// Selected item style.
	selectedStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Bold(true)

	// Normal item style.
	normalStyle = lipgloss.NewStyle().
			Foreground(colorGray)
)

// Modal styles using lipgloss v2 for canvas/layer support.
var (
	modalStyle = lipglossv2.NewStyle().
			Border(lipglossv2.RoundedBorder()).
			BorderForeground(lipglossv2.Color("#89b4fa")).
			Padding(1, 2)

	modalTitleStyle = lipglossv2.NewStyle().
			Bold(true).
			Foreground(lipglossv2.Color("#cdd6f4"))

	modalHelpStyle = lipglossv2.NewStyle().
			Foreground(lipglossv2.Color("#6c7086")).
			MarginTop(1)

	// Spinner style.
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorBlue)
)

// Git status styles (lipgloss v1 for bubbles compatibility).
var (
	gitBranchStyle = lipgloss.NewStyle().
			Foreground(colorWhite)

	gitAdditionsStyle = lipgloss.NewStyle().
				Foreground(colorGreen)

	gitDeletionsStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f38ba8")) // red

	gitCleanStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	gitDirtyStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	gitLoadingStyle = lipgloss.NewStyle().
			Foreground(colorGray)
)
