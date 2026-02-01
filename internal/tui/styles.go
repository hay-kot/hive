// Package tui implements the Bubble Tea TUI for hive.
package tui

import (
	"image/color"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/hay-kot/hive/internal/styles"
)

// Re-export colors for local use (lipgloss v2 types).
var (
	colorGreen  = lipgloss.Color("#9ece6a")
	colorYellow = lipgloss.Color("#e0af68")
	colorBlue   = lipgloss.Color("#7aa2f7")
	colorCyan   = lipgloss.Color("#7dcfff") // cyan for ready status
	colorGray   = lipgloss.Color("#565f89")
	colorWhite  = lipgloss.Color("#c0caf5")
)

// Styles used for rendering the TUI.
var (
	// Selected border style for left accent bar.
	selectedBorderStyle = lipgloss.NewStyle().
		Foreground(colorBlue)
)

// Icons and symbols.
const (
	iconDot = "•" // Unicode bullet separator
)

// Use shared banner and style.
var (
	banner      = styles.Banner
	bannerStyle = styles.BannerStyle.PaddingLeft(1).PaddingBottom(1)
)

// Modal styles using lipgloss v2 for canvas/layer support.
var (
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7aa2f7")).
			Padding(1, 2)

	modalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c0caf5"))

	modalHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			MarginTop(1)

	modalButtonStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#3b4261")).
				Foreground(lipgloss.Color("#a9b1d6"))

	modalButtonSelectedStyle = lipgloss.NewStyle().
					Padding(0, 1).
					Background(lipgloss.Color("#7aa2f7")).
					Foreground(lipgloss.Color("#1a1b26")).
					Bold(true)
)

// Color pool for deterministic color hashing of topics and senders.
var colorPool = []color.Color{
	lipgloss.Color("#9ece6a"), // green
	lipgloss.Color("#7aa2f7"), // blue
	lipgloss.Color("#e0af68"), // yellow
	lipgloss.Color("#bb9af7"), // purple
	lipgloss.Color("#7dcfff"), // cyan
	lipgloss.Color("#f7768e"), // red/pink
	lipgloss.Color("#ff9e64"), // orange
	lipgloss.Color("#73daca"), // teal
}

// ColorForString returns a deterministic color for a given string.
// The same string always produces the same color.
func ColorForString(s string) color.Color {
	var hash uint32
	for _, c := range s {
		hash = hash*31 + uint32(c)
	}
	return colorPool[hash%uint32(len(colorPool))]
}

// Layout styles for tab views.
var (
	viewSelectedStyle = lipgloss.NewStyle().
				Foreground(colorBlue).
				Bold(true)

	viewNormalStyle = lipgloss.NewStyle().
			Foreground(colorGray)
)

// Git status styles.
var (
	colorRed = lipgloss.Color("#f38ba8")

	gitAdditionsStyle = lipgloss.NewStyle().Foreground(colorGreen)
	gitDeletionsStyle = lipgloss.NewStyle().Foreground(colorRed)
	gitCleanStyle     = lipgloss.NewStyle().Foreground(colorGray)
	gitDirtyStyle     = lipgloss.NewStyle().Foreground(colorYellow)
	gitLoadingStyle   = lipgloss.NewStyle().Foreground(colorGray)
)

// Form field styles (Tokyo Night themed).
var (
	formTitleStyle = lipgloss.NewStyle().
			Foreground(colorBlue).
			Bold(true)

	formTitleBlurredStyle = lipgloss.NewStyle().
				Foreground(colorGray)

	formFieldStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorGray).
			Padding(0, 1)

	formFieldFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBlue).
				Padding(0, 1)

	formErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f7768e")) // Tokyo Night red

	formHelpStyle = lipgloss.NewStyle().
			Foreground(colorGray)
)
