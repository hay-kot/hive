// Package styles provides shared lipgloss styles for CLI and TUI components.
package styles

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Tokyo Night color palette.
var (
	ColorGreen  = lipgloss.Color("#9ece6a")
	ColorYellow = lipgloss.Color("#e0af68")
	ColorBlue   = lipgloss.Color("#7aa2f7")
	ColorGray   = lipgloss.Color("#565f89")
	ColorWhite  = lipgloss.Color("#c0caf5")
)

// Banner ASCII art for the header.
const Banner = `
 ╦ ╦╦╦  ╦╔═╗
 ╠═╣║╚╗╔╝║╣
 ╩ ╩╩ ╚╝ ╚═╝`

// BannerStyle styles the ASCII art banner.
var BannerStyle = lipgloss.NewStyle().
	Foreground(ColorBlue).
	Bold(true)

// CommandHeaderStyle styles the hook command headers.
var CommandHeaderStyle = lipgloss.NewStyle().
	Foreground(ColorBlue).
	Bold(true)

// CommandStyle styles the command text.
var CommandStyle = lipgloss.NewStyle().
	Foreground(ColorWhite)

// DividerStyle styles horizontal dividers.
var DividerStyle = lipgloss.NewStyle().
	Foreground(ColorGray)

// FormTheme returns a huh form theme using Tokyo Night colors.
func FormTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Title = lipgloss.NewStyle().
		Foreground(ColorBlue).
		Bold(true)

	t.Focused.Description = lipgloss.NewStyle().
		Foreground(ColorGray)

	t.Focused.TextInput.Cursor = lipgloss.NewStyle().
		Foreground(ColorBlue)

	t.Focused.TextInput.Placeholder = lipgloss.NewStyle().
		Foreground(ColorGray)

	t.Focused.TextInput.Prompt = lipgloss.NewStyle().
		Foreground(ColorBlue)

	t.Focused.FocusedButton = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1b26")).
		Background(ColorBlue).
		Bold(true).
		Padding(0, 1)

	t.Focused.BlurredButton = lipgloss.NewStyle().
		Foreground(ColorWhite).
		Background(lipgloss.Color("#3b4261")).
		Padding(0, 1)

	t.Blurred.Title = lipgloss.NewStyle().
		Foreground(ColorGray)

	t.Blurred.Description = lipgloss.NewStyle().
		Foreground(ColorGray)

	t.Blurred.TextInput.Placeholder = lipgloss.NewStyle().
		Foreground(ColorGray)

	return t
}
