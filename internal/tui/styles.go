// Package tui implements the Bubble Tea TUI for hive.
package tui

import (
	"github.com/charmbracelet/lipgloss"
	lipglossv2 "github.com/charmbracelet/lipgloss/v2"
)

// Tokyo Night color palette.
var (
	colorGreen  = lipgloss.Color("#9ece6a") // green
	colorYellow = lipgloss.Color("#e0af68") // yellow
	colorBlue   = lipgloss.Color("#7aa2f7") // blue
	/* colorPurple = lipgloss.Color("#bb9af7") // purple */
	/* colorCyan   = lipgloss.Color("#7dcfff") // cyan */
	colorGray  = lipgloss.Color("#565f89") // comment
	colorWhite = lipgloss.Color("#c0caf5") // foreground
)

// Styles used for rendering the TUI (lipgloss v1 for bubbles compatibility).
var (
	// Title style for the list header.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlue).PaddingLeft(1).PaddingBottom(1)

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

// Banner ASCII art for the header.
const banner = `
 ╦ ╦╦╦  ╦╔═╗
 ╠═╣║╚╗╔╝║╣
 ╩ ╩╩ ╚╝ ╚═╝`

// bannerStyle styles the ASCII art banner.
var bannerStyle = lipgloss.NewStyle().
	Foreground(colorBlue).
	Bold(true).
	PaddingLeft(1).
	PaddingBottom(1)

// Modal styles using lipgloss v2 for canvas/layer support.
var (
	modalStyle = lipglossv2.NewStyle().
			Border(lipglossv2.RoundedBorder()).
			BorderForeground(lipglossv2.Color("#7aa2f7")).
			Padding(1, 2)

	modalTitleStyle = lipglossv2.NewStyle().
			Bold(true).
			Foreground(lipglossv2.Color("#c0caf5"))

	modalHelpStyle = lipglossv2.NewStyle().
			Foreground(lipglossv2.Color("#565f89")).
			MarginTop(1)

	// Spinner style.
	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorBlue)
)
