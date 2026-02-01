// Package styles provides shared lipgloss styles for CLI and TUI components.
package styles

import (
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
