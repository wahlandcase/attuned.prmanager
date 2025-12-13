package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Banner returns the ASCII art banner for the application header
var Banner = []string{
	"    _  _____ _____ _   _ _   _ _____ ____       ____  ____       __  __    _    _   _    _    ____ _____ ____  ",
	"   / \\|_   _|_   _| | | | \\ | | ____|  _ \\     |  _ \\|  _ \\     |  \\/  |  / \\  | \\ | |  / \\  / ___| ____|  _ \\ ",
	"  / _ \\ | |   | | | | | |  \\| |  _| | | | |    | |_) | |_) |    | |\\/| | / _ \\ |  \\| | / _ \\| |  _|  _| | |_) |",
	" / ___ \\| |   | | | |_| | |\\  | |___| |_| |    |  __/|  _ <     | |  | |/ ___ \\| |\\  |/ ___ \\ |_| | |___|  _ < ",
	"/_/   \\_\\_|   |_|  \\___/|_| \\_|_____|____/     |_|   |_| \\_\\    |_|  |_/_/   \\_\\_| \\_/_/   \\_\\____|_____|_| \\_\\",
}

// RenderBanner returns the styled banner as a string
func RenderBanner(dryRun bool) string {
	bannerStyle := lipgloss.NewStyle().
		Foreground(ColorCyan).
		Align(lipgloss.Center)

	var lines []string
	for _, line := range Banner {
		lines = append(lines, bannerStyle.Render(line))
	}

	// Add dry run warning if enabled
	if dryRun {
		lines = append(lines, "")
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorYellow).
			Bold(true).
			Align(lipgloss.Center)
		lines = append(lines, warningStyle.Render("⚠ DRY RUN MODE"))
	}

	return strings.Join(lines, "\n")
}

// RenderBannerLines returns the banner as individual lines for more control
func RenderBannerLines(dryRun bool) []string {
	bannerStyle := lipgloss.NewStyle().Foreground(ColorCyan)

	var lines []string
	for _, line := range Banner {
		lines = append(lines, bannerStyle.Render(line))
	}

	if dryRun {
		lines = append(lines, "")
		warningStyle := lipgloss.NewStyle().
			Foreground(ColorYellow).
			Bold(true)
		lines = append(lines, warningStyle.Render("⚠ DRY RUN MODE"))
	}

	return lines
}
