package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SectionHeader creates a styled section header with a title and color
// Example: "─── TITLE ───────────"
func SectionHeader(title string, color lipgloss.Color) string {
	dashes := strings.Repeat("─", max(25-len(title), 0))
	headerStyle := lipgloss.NewStyle().Foreground(color)
	titleStyle := lipgloss.NewStyle().Foreground(color).Bold(true)

	return fmt.Sprintf("%s%s%s",
		headerStyle.Render("  ─── "),
		titleStyle.Render(title),
		headerStyle.Render(" "+dashes),
	)
}

// BranchFlowDiagram creates a visual diagram showing branch flow
// Example: dev ====> staging
func BranchFlowDiagram(head, base string) string {
	headColor := BranchColor(head)
	baseColor := BranchColor(base)

	headStyle := lipgloss.NewStyle().Foreground(headColor)
	headBoldStyle := lipgloss.NewStyle().Foreground(headColor).Bold(true)
	baseStyle := lipgloss.NewStyle().Foreground(baseColor)
	baseBoldStyle := lipgloss.NewStyle().Foreground(baseColor).Bold(true)
	arrowStyle := lipgloss.NewStyle().Foreground(ColorCyan)

	// Center the text in the boxes (7 chars to fit "staging")
	headText := centerText(head, 7)
	baseText := centerText(base, 7)

	// Create box components (9 dashes = 7 chars + 2 padding)
	topLeft := headStyle.Render("  ┌─────────┐")
	topRight := baseStyle.Render("┌─────────┐")

	middleLeft := headStyle.Render("  │ ") + headBoldStyle.Render(headText) + headStyle.Render(" │")
	arrow := arrowStyle.Render("  ====>  ")
	middleRight := baseStyle.Render("│ ") + baseBoldStyle.Render(baseText) + baseStyle.Render(" │")

	bottomLeft := headStyle.Render("  └─────────┘")
	bottomRight := baseStyle.Render("└─────────┘")

	// Combine into lines
	line1 := topLeft + "         " + topRight
	line2 := middleLeft + arrow + middleRight
	line3 := bottomLeft + "         " + bottomRight

	return line1 + "\n" + line2 + "\n" + line3
}

// centerText centers a string within a given width
func centerText(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	leftPad := (width - len(s)) / 2
	rightPad := width - len(s) - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

// YesNoButtons creates interactive Yes/No buttons
// selection: 0 for Yes, 1 for No
func YesNoButtons(selection int) string {
	var yesBorder, yesText, yesIcon lipgloss.Color
	var noBorder, noText, noIcon lipgloss.Color

	if selection == 0 {
		yesBorder = ColorGreen
		yesText = ColorGreen
		yesIcon = ColorGreen
	} else {
		yesBorder = ColorDarkGray
		yesText = ColorWhite
		yesIcon = ColorDarkGray
	}

	if selection == 1 {
		noBorder = ColorRed
		noText = ColorRed
		noIcon = ColorRed
	} else {
		noBorder = ColorDarkGray
		noText = ColorWhite
		noIcon = ColorDarkGray
	}

	yesStyle := lipgloss.NewStyle().Foreground(yesBorder)
	yesTextStyle := lipgloss.NewStyle().Foreground(yesText).Bold(true)
	yesIconStyle := lipgloss.NewStyle().Foreground(yesIcon)

	noStyle := lipgloss.NewStyle().Foreground(noBorder)
	noTextStyle := lipgloss.NewStyle().Foreground(noText).Bold(true)
	noIconStyle := lipgloss.NewStyle().Foreground(noIcon)

	// Build buttons
	var iconYes, iconNo string
	if selection == 0 {
		iconYes = ">"
	} else {
		iconYes = " "
	}
	if selection == 1 {
		iconNo = ">"
	} else {
		iconNo = " "
	}

	line1 := yesStyle.Render("  ┌────────┐") + " " + noStyle.Render("┌───────┐")
	line2 := fmt.Sprintf("%s%s%s %s%s%s",
		yesStyle.Render("  │"),
		yesTextStyle.Render(fmt.Sprintf(" %s  YES ", yesIconStyle.Render(iconYes))),
		yesStyle.Render("│"),
		noStyle.Render("│"),
		noTextStyle.Render(fmt.Sprintf(" %s  NO ", noIconStyle.Render(iconNo))),
		noStyle.Render("│"),
	)
	line3 := yesStyle.Render("  └────────┘") + " " + noStyle.Render("└───────┘")

	return line1 + "\n" + line2 + "\n" + line3
}

// Spinner frames using braille characters (matching Rust app)
var SpinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// Spinner returns the spinner character at the given frame index
func Spinner(frame int) string {
	return string(SpinnerFrames[frame%len(SpinnerFrames)])
}

// Checkbox renders a checkbox in the given state
func Checkbox(checked bool) string {
	if checked {
		return "[✓]"
	}
	return "[ ]"
}

// Arrow returns an arrow indicator for selection
func Arrow(selected bool) string {
	if selected {
		return "▶ "
	}
	return "  "
}

// KeyBinding renders a key binding hint
func KeyBinding(key, description string, color lipgloss.Color) string {
	keyStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorWhite)

	return fmt.Sprintf("%s %s",
		keyStyle.Render(key),
		descStyle.Render(description),
	)
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MenuInfoPanel returns the ASCII art and description for a menu item
func MenuInfoPanel(index int) (title string, lines []string) {
	switch index {
	case 0: // Single Repo
		title = "Single Repo Mode"
		prBox := lipgloss.NewStyle().Foreground(ColorCyan)
		prText := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
		lines = []string{
			"",
			prBox.Render("        ┌──────────┐"),
			prBox.Render("        │") + prText.Render("    PR  ") + prBox.Render("  │"),
			prBox.Render("        └──────────┘"),
			"",
			"  • Detects dev/staging/main branches",
			"  • Shows commits to be merged",
			"  • Extracts Linear tickets (ATT-XXX)",
			"  • Creates or updates existing PR",
		}
	case 1: // Batch Mode
		title = "Batch Mode"
		prBox := lipgloss.NewStyle().Foreground(ColorMagenta)
		prText := lipgloss.NewStyle().Foreground(ColorMagenta).Bold(true)
		lines = []string{
			"",
			prBox.Render("     ┌────┐") + prBox.Render(" ┌────┐") + prBox.Render(" ┌────┐"),
			prBox.Render("     │") + prText.Render(" PR ") + prBox.Render("│") + prBox.Render(" │") + prText.Render(" PR ") + prBox.Render("│") + prBox.Render(" │") + prText.Render(" PR ") + prBox.Render("│"),
			prBox.Render("     └────┘") + prBox.Render(" └────┘") + prBox.Render(" └────┘"),
			"",
			"  • Scans ~/Programming/attuned",
			"  • Select repos with checkboxes",
			"  • Extracts Linear tickets (ATT-XXX)",
			"  • Shows summary of results",
		}
	case 2: // View Open PRs
		title = "View Open PRs"
		mainStyle := lipgloss.NewStyle().Foreground(ColorRed)
		mainText := lipgloss.NewStyle().Foreground(ColorRed).Bold(true)
		stagingStyle := lipgloss.NewStyle().Foreground(ColorYellow)
		stagingText := lipgloss.NewStyle().Foreground(ColorYellow).Bold(true)
		devStyle := lipgloss.NewStyle().Foreground(ColorGreen)
		devText := lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
		lines = []string{
			"",
			mainStyle.Render("            ○───○───○") + mainText.Render(" main"),
			stagingStyle.Render("           ╱"),
			stagingStyle.Render("      ○───○") + stagingText.Render(" staging"),
			devStyle.Render("     ╱"),
			devStyle.Render("    ○") + devText.Render(" dev"),
			"",
			"  • Select and batch merge",
			"  • Smart ordering (dev first)",
			"  • Open or copy URLs",
		}
	default: // Quit
		title = "Quit"
		lines = []string{
			"",
			"  Exit the application",
		}
	}
	return title, lines
}

// TwoColumns renders two columns side by side
func TwoColumns(left, right string, gap int) string {
	gapStr := strings.Repeat(" ", gap)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, gapStr, right)
}

// UnifiedPanel creates two columns with a vertical separator (no border - outer border is in View)
func UnifiedPanel(leftContent, rightContent string, leftWidth, rightWidth int, borderColor lipgloss.Color) string {
	leftStyle := lipgloss.NewStyle().Width(leftWidth).Padding(0, 1)
	rightStyle := lipgloss.NewStyle().Width(rightWidth).Padding(0, 1)

	leftCol := leftStyle.Render(leftContent)
	rightCol := rightStyle.Render(rightContent)

	// Build vertical separator to match column height
	separatorStyle := lipgloss.NewStyle().Foreground(ColorDarkGray)
	separator := separatorStyle.Render("│")

	leftLines := strings.Split(leftCol, "\n")
	rightLines := strings.Split(rightCol, "\n")
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}
	var sepLines []string
	for i := 0; i < maxLines; i++ {
		sepLines = append(sepLines, separator)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, strings.Join(sepLines, "\n"), rightCol)
}

// ColumnBox creates a bordered column with title for two-column layouts
// If height > 0, content is padded/truncated to exactly that many lines
func ColumnBox(content string, title string, color lipgloss.Color, isActive bool, width int, height int) string {
	borderColor := color
	if !isActive {
		borderColor = ColorDarkGray
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width)

	var fullContent string
	if title != "" {
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(color)
		fullContent = titleStyle.Render(" "+title+" ") + "\n" + content
	} else {
		fullContent = content
	}

	// Manually pad/truncate to fixed height
	if height > 0 {
		lines := strings.Split(fullContent, "\n")
		if len(lines) < height {
			// Pad with empty lines
			for len(lines) < height {
				lines = append(lines, "")
			}
		} else if len(lines) > height {
			// Truncate
			lines = lines[:height]
		}
		fullContent = strings.Join(lines, "\n")
	}

	return style.Render(fullContent)
}

// FilterInput renders a search/filter input box
// If width > 0, the box will have a fixed width
func FilterInput(filter string, title string, color lipgloss.Color, width int) string {
	var filterDisplay string
	if filter == "" {
		filterDisplay = lipgloss.NewStyle().Foreground(ColorDarkGray).Render("Type to filter...")
	} else {
		filterDisplay = lipgloss.NewStyle().Foreground(ColorYellow).Render(filter)
	}

	cursor := lipgloss.NewStyle().Foreground(ColorYellow).Render("█")
	searchIcon := lipgloss.NewStyle().Foreground(ColorCyan).Render(" 🔍 ")

	content := searchIcon + filterDisplay + cursor

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 1)

	if width > 0 {
		style = style.Width(width)
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(color)
	return style.Render(titleStyle.Render(title) + "\n" + content)
}

// RepoListItemWithCommits renders a repo item with checkbox and commit indicator
// commitCount: -1 = loading, 0 = no commits, >0 = has commits
func RepoListItemWithCommits(name string, selected bool, highlighted bool, color lipgloss.Color, indent string, commitCount int, spinnerFrame int) string {
	checkbox := Checkbox(selected)
	arrow := Arrow(highlighted)

	var style lipgloss.Style
	if highlighted {
		style = lipgloss.NewStyle().Foreground(color).Bold(true)
	} else if selected {
		style = lipgloss.NewStyle().Foreground(color)
	} else {
		style = lipgloss.NewStyle().Foreground(ColorWhite)
	}

	indentStyle := lipgloss.NewStyle().Foreground(ColorDarkGray)
	checkStyle := lipgloss.NewStyle().Foreground(color)

	// Show indicator based on state
	var indicator string
	if commitCount < 0 {
		// Loading - show spinner
		spinner := string(SpinnerFrames[spinnerFrame%len(SpinnerFrames)])
		indicator = lipgloss.NewStyle().Foreground(ColorYellow).Render(" " + spinner)
	} else if commitCount > 0 {
		// Has commits - green dot
		indicator = lipgloss.NewStyle().Foreground(ColorGreen).Render(" ●")
	} else {
		// No commits - dim dot
		indicator = lipgloss.NewStyle().Foreground(ColorDarkGray).Render(" ○")
	}

	return fmt.Sprintf("%s%s%s %s%s",
		style.Render(arrow),
		indentStyle.Render(indent),
		checkStyle.Render(checkbox),
		name,
		indicator,
	)
}

// PRListItem renders a compact single-line PR item for the merge view
func PRListItem(repoName string, prNumber uint64, headBranch string, baseBranch string, prURL string, selected bool, highlighted bool, color lipgloss.Color) string {
	checkbox := Checkbox(selected)
	arrow := Arrow(highlighted)

	var style lipgloss.Style
	if highlighted {
		style = lipgloss.NewStyle().Foreground(color).Bold(true)
	} else if selected {
		style = lipgloss.NewStyle().Foreground(color)
	} else {
		style = lipgloss.NewStyle().Foreground(ColorWhite)
	}

	checkStyle := lipgloss.NewStyle().Foreground(color)
	prNumStyle := lipgloss.NewStyle().Foreground(ColorDarkGray)

	return fmt.Sprintf("%s%s %s %s",
		style.Render(arrow),
		checkStyle.Render(checkbox),
		repoName,
		prNumStyle.Render(fmt.Sprintf("#%d", prNumber)),
	)
}

// ParentHeader renders a parent repo header for nested repos
func ParentHeader(name string) string {
	style := lipgloss.NewStyle().Foreground(ColorYellow).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDarkGray)
	return fmt.Sprintf("  %s%s",
		style.Render(fmt.Sprintf("┌─ %s ", name)),
		dimStyle.Render(strings.Repeat("─", 15)),
	)
}

// MenuRow renders a menu row with optional highlight background
// width should be the inner width of the panel (excluding border)
func MenuRow(icon, title, desc string, color lipgloss.Color, selected bool, width int) []string {
	arrow := "  "
	if selected {
		arrow = "▶ "
	}

	if selected {
		// For selected items, render the whole line with background
		rowStyle := lipgloss.NewStyle().Background(ColorDarkGray).Width(width)
		arrowStyle := lipgloss.NewStyle().Foreground(color).Background(ColorDarkGray)
		iconStyle := lipgloss.NewStyle().Background(ColorDarkGray)
		titleStyle := lipgloss.NewStyle().Foreground(color).Bold(true).Background(ColorDarkGray)
		descStyle := lipgloss.NewStyle().Foreground(ColorWhite).Background(ColorDarkGray)

		line1 := rowStyle.Render(arrowStyle.Render(arrow) + iconStyle.Render(icon+"  ") + titleStyle.Render(title))
		line2 := rowStyle.Render("       " + descStyle.Render(desc))

		return []string{line1, line2}
	}

	// Non-selected items - no background
	arrowStyle := lipgloss.NewStyle().Foreground(color)
	titleStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorWhite)

	line1 := arrowStyle.Render(arrow) + icon + "  " + titleStyle.Render(title)
	line2 := "       " + descStyle.Render(desc)

	return []string{line1, line2}
}
