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
	colorGray   = lipgloss.Color("#565f89") // comment
	colorWhite  = lipgloss.Color("#c0caf5") // foreground
)

// Styles used for rendering the TUI (lipgloss v1 for bubbles compatibility).
var (
	// Title style for the list header.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlue).
			PaddingLeft(1)

	// Active session state style.
	activeStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	// Recycled session state style.
	recycledStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	// Selected item style (matches border color).
	selectedStyle = lipgloss.NewStyle().
			Foreground(colorBlue).
			Bold(true)

	// Normal item style (no color, uses terminal default).
	normalStyle = lipgloss.NewStyle()

	// Path style for subtle directory text.
	pathStyle = lipgloss.NewStyle().
			Foreground(colorGray)

	// Prompt style for session prompt text.
	promptStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			Italic(true)

	// Selected border style for left accent bar.
	selectedBorderStyle = lipgloss.NewStyle().
				Foreground(colorBlue)
)

// Icons and symbols.
const (
	iconGit = "\ue702" // Nerd Font git icon
	iconDot = "•"      // Unicode bullet separator
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

	modalButtonStyle = lipglossv2.NewStyle().
				Padding(0, 1).
				Background(lipglossv2.Color("#3b4261")).
				Foreground(lipglossv2.Color("#a9b1d6"))

	modalButtonSelectedStyle = lipglossv2.NewStyle().
					Padding(0, 1).
					Background(lipglossv2.Color("#7aa2f7")).
					Foreground(lipglossv2.Color("#1a1b26")).
					Bold(true)

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
			Foreground(colorGray)

	gitDirtyStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	gitLoadingStyle = lipgloss.NewStyle().
			Foreground(colorGray)
)
