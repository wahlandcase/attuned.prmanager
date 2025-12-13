package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Warp terminal on WSL2 hangs on terminal capability queries.
// This init() runs before main() so it catches all package initialization.
func init() {
	if os.Getenv("TERM_PROGRAM") == "WarpTerminal" {
		os.Setenv("TERM", "dumb")
		os.Setenv("COLORTERM", "truecolor")
	}
}

// Color palette matching the Rust app
var (
	// Primary colors
	ColorCyan       = lipgloss.Color("#00FFFF")
	ColorGreen      = lipgloss.Color("#00FF00")
	ColorYellow     = lipgloss.Color("#FFFF00")
	ColorRed        = lipgloss.Color("#FF0000")
	ColorMagenta    = lipgloss.Color("#FF00FF")
	ColorBlue       = lipgloss.Color("#5555FF")
	ColorPurple     = lipgloss.Color("#AA55FF")
	ColorOrange     = lipgloss.Color("#FFA500")
	ColorLightGreen = lipgloss.Color("#90EE90")

	// Neutral colors
	ColorWhite    = lipgloss.Color("#FFFFFF")
	ColorDarkGray = lipgloss.Color("8") // ANSI color 8 (dark gray) - matches ratatui's DarkGray
	ColorGray     = lipgloss.Color("#808080")
	ColorBlack    = lipgloss.Color("#000000")
)

// Base styles for common UI elements

// TitleStyle is used for main titles and headers
var TitleStyle = lipgloss.NewStyle().
	Foreground(ColorCyan).
	Bold(true)

// SubtitleStyle is used for section headers
var SubtitleStyle = lipgloss.NewStyle().
	Foreground(ColorWhite).
	Bold(true)

// SelectedStyle highlights the selected item
var SelectedStyle = lipgloss.NewStyle().
	Background(ColorDarkGray).
	Bold(true)

// NormalStyle is the default text style
var NormalStyle = lipgloss.NewStyle().
	Foreground(ColorWhite)

// ErrorStyle is used for error messages
var ErrorStyle = lipgloss.NewStyle().
	Foreground(ColorRed).
	Bold(true)

// SuccessStyle is used for success messages
var SuccessStyle = lipgloss.NewStyle().
	Foreground(ColorGreen).
	Bold(true)

// DimStyle is used for less important text
var DimStyle = lipgloss.NewStyle().
	Foreground(ColorDarkGray)

// WarningStyle is used for warnings
var WarningStyle = lipgloss.NewStyle().
	Foreground(ColorYellow).
	Bold(true)

// InfoStyle is used for informational text
var InfoStyle = lipgloss.NewStyle().
	Foreground(ColorCyan)

// Branch-specific styles

// DevBranchStyle is used for dev branch references
var DevBranchStyle = lipgloss.NewStyle().
	Foreground(ColorGreen).
	Bold(true)

// StagingBranchStyle is used for staging branch references
var StagingBranchStyle = lipgloss.NewStyle().
	Foreground(ColorYellow).
	Bold(true)

// MainBranchStyle is used for main/master branch references
var MainBranchStyle = lipgloss.NewStyle().
	Foreground(ColorRed).
	Bold(true)

// UI Component styles

// BorderStyle is used for box borders
var BorderStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(ColorCyan).
	Padding(1, 2)

// InputBoxStyle is used for text input boxes
var InputBoxStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(ColorYellow).
	Padding(0, 1)

// ButtonStyle is used for buttons
var ButtonStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	Padding(0, 2)

// TicketStyle is used for Linear ticket references
var TicketStyle = lipgloss.NewStyle().
	Foreground(ColorYellow).
	Bold(true)

// CommitHashStyle is used for git commit hashes
var CommitHashStyle = lipgloss.NewStyle().
	Foreground(ColorMagenta)

// URLStyle is used for URLs
var URLStyle = lipgloss.NewStyle().
	Foreground(ColorCyan)

// BatchModeStyle is used for batch mode elements
var BatchModeStyle = lipgloss.NewStyle().
	Foreground(ColorMagenta).
	Bold(true)

// Helper functions for creating colored text

// BranchColor returns the appropriate color for a branch name
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

// BranchStyle returns the appropriate style for a branch name
func BranchStyle(branch string) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(BranchColor(branch)).
		Bold(true)
}
