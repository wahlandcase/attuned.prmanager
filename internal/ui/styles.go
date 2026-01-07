package ui

import "github.com/charmbracelet/lipgloss"

// Note: Warp terminal fix is in internal/termfix package, imported first in main.go

var (
	ColorCyan       = lipgloss.Color("#00FFFF")
	ColorGreen      = lipgloss.Color("#00FF00")
	ColorYellow     = lipgloss.Color("#FFFF00")
	ColorRed        = lipgloss.Color("#FF0000")
	ColorMagenta    = lipgloss.Color("#FF00FF")
	ColorBlue       = lipgloss.Color("#5555FF")
	ColorPurple     = lipgloss.Color("#AA55FF")
	ColorOrange     = lipgloss.Color("#FFA500")
	ColorLightGreen = lipgloss.Color("#90EE90")
	ColorWhite      = lipgloss.Color("#FFFFFF")
	ColorDarkGray   = lipgloss.Color("8") // ANSI 8 - matches ratatui's DarkGray
)

func BranchColor(branch string) lipgloss.Color {
	switch branch {
	case "dev":
		return ColorGreen
	case "staging":
		return ColorYellow
	case "main", "master":
		return ColorRed
	default:
		return ColorWhite
	}
}
