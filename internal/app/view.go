package app

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/wahlandcase/attuned.prmanager/internal/models"
	"github.com/wahlandcase/attuned.prmanager/internal/ui"
	"github.com/wahlandcase/attuned.prmanager/internal/update"

	"github.com/charmbracelet/lipgloss"
)

// Max content width for stable layout (prevents UI shifting)
const maxContentWidth = 120

// contentWidth returns the usable content width, adapting to terminal size
func (m Model) contentWidth() int {
	if m.width < maxContentWidth+4 {
		return m.width - 4 // leave some margin
	}
	return maxContentWidth
}

// View renders the application
func (m Model) View() string {
	if m.shouldQuit {
		return ""
	}

	// Calculate fixed element heights
	bannerLines := len(ui.Banner) // 5 lines
	if m.dryRun {
		bannerLines += 2 // dry run warning
	}
	statusHeight := 3 // status bar with border

	// Available height for content = total - banner - gaps - status
	availableHeight := m.height - bannerLines - 3 - statusHeight
	if availableHeight < 10 {
		availableHeight = 10
	}

	var sections []string

	// Banner
	sections = append(sections, ui.RenderBanner(m.dryRun))
	sections = append(sections, "")

	// Use fixed content width for stable layout
	contentWidth := m.contentWidth()

	// Screens that manage their own full layout (no outer box)
	fullLayoutScreens := m.screen == ScreenLoading ||
		m.screen == ScreenBatchRepoSelect ||
		m.screen == ScreenViewOpenPrs ||
		m.screen == ScreenBatchSummary ||
		m.screen == ScreenMergeSummary ||
		m.screen == ScreenCommitReview

	if fullLayoutScreens {
		sections = append(sections, m.renderContentWithHeight(availableHeight))
	} else {
		// Standard outer box for simpler screens - always use fixed width
		outerBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorPurple).
			Width(contentWidth).
			Padding(1, 2)

		sections = append(sections, outerBox.Render(m.renderContentWithHeight(availableHeight)))
	}

	// Status bar
	sections = append(sections, "")
	sections = append(sections, m.renderStatusBar())

	content := strings.Join(sections, "\n")

	// Center horizontally in the terminal
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, content)
}

func (m Model) renderContentWithHeight(availableHeight int) string {
	switch m.screen {
	case ScreenMainMenu:
		return m.renderMainMenu()
	case ScreenPrTypeSelect:
		return m.renderPrTypeSelect()
	case ScreenLoading:
		return m.renderLoading()
	case ScreenCommitReview:
		return m.renderCommitReviewWithHeight(availableHeight)
	case ScreenTitleInput:
		return m.renderTitleInput()
	case ScreenConfirmation:
		return m.renderConfirmation()
	case ScreenCreating:
		return m.renderCreating()
	case ScreenComplete:
		return m.renderComplete()
	case ScreenError:
		return m.renderError()
	case ScreenBatchRepoSelect:
		return m.renderBatchRepoSelectWithHeight(availableHeight)
	case ScreenBatchConfirmation:
		return m.renderBatchConfirmationWithHeight(availableHeight)
	case ScreenBatchProcessing:
		return m.renderBatchProcessing()
	case ScreenBatchSummary:
		return m.renderBatchSummaryWithHeight(availableHeight)
	case ScreenViewOpenPrs:
		return m.renderViewOpenPrsWithHeight(availableHeight)
	case ScreenMergeConfirmation:
		return m.renderMergeConfirmation()
	case ScreenMerging:
		return m.renderMerging()
	case ScreenMergeSummary:
		return m.renderMergeSummaryWithHeight(availableHeight)
	case ScreenUpdatePrompt:
		return m.renderUpdatePrompt()
	case ScreenUpdating:
		return m.renderUpdating()
	default:
		return ""
	}
}

func (m Model) renderMainMenu() string {
	menuItems := []struct {
		icon  string
		title string
		desc  string
		color lipgloss.Color
	}{
		{"1.", "SINGLE REPO", "Create PR for current repo", ui.ColorCyan},
		{"2.", "BATCH MODE", "Create PRs for multiple repos", ui.ColorMagenta},
		{"3.", "VIEW OPEN PRS", "See all open release PRs", ui.ColorYellow},
		{"4.", "QUIT", "Exit application", ui.ColorRed},
	}

	// Build left column (menu) content
	var menuLines []string
	menuLines = append(menuLines, "")
	for i, item := range menuItems {
		rows := ui.MenuRow(item.icon, item.title, item.desc, item.color, i == m.menuIndex, 46)
		menuLines = append(menuLines, rows...)
		menuLines = append(menuLines, "")
	}

	menuTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorOrange)
	menuContent := menuTitleStyle.Render(" Select Mode ") + "\n" + strings.Join(menuLines, "\n")

	// Build right column (info panel)
	infoTitle, infoLines := ui.MenuInfoPanel(m.menuIndex)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite)
	infoContent := titleStyle.Render(" "+infoTitle+" ") + "\n" + strings.Join(infoLines, "\n")

	return ui.UnifiedPanel(menuContent, infoContent, 48, 48, ui.ColorCyan)
}

func (m Model) renderPrTypeSelect() string {
	mainBranch := "main"
	if m.repoInfo != nil {
		mainBranch = m.repoInfo.MainBranch
	}

	// Build left column (menu) content
	var menuLines []string
	menuLines = append(menuLines, "")

	types := []struct {
		num       string
		head      string
		base      string
		desc      string
		headColor lipgloss.Color
		baseColor lipgloss.Color
	}{
		{"1.", "dev", "staging", "Merge to staging for QA", ui.ColorGreen, ui.ColorYellow},
		{"2.", "staging", mainBranch, "Release to production", ui.ColorYellow, ui.ColorRed},
	}

	for i, t := range types {
		isSelected := i == m.menuIndex
		arrow := "  "
		if isSelected {
			arrow = "▶ "
		}

		var line1, line2 string
		if isSelected {
			// Render with full-width background
			rowStyle := lipgloss.NewStyle().Background(ui.ColorDarkGray).Width(46)
			arrowStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Background(ui.ColorDarkGray)
			numStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true).Background(ui.ColorDarkGray)
			headStyle := lipgloss.NewStyle().Foreground(t.headColor).Bold(true).Background(ui.ColorDarkGray)
			baseStyle := lipgloss.NewStyle().Foreground(t.baseColor).Bold(true).Background(ui.ColorDarkGray)
			descStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite).Background(ui.ColorDarkGray)
			bgStyle := lipgloss.NewStyle().Background(ui.ColorDarkGray)

			line1 = rowStyle.Render(arrowStyle.Render(arrow) + numStyle.Render(t.num) + bgStyle.Render(" ") + headStyle.Render(t.head) + bgStyle.Render(" → ") + baseStyle.Render(t.base))
			line2 = rowStyle.Render("      " + descStyle.Render(t.desc))
		} else {
			arrowStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
			numStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
			headStyle := lipgloss.NewStyle().Foreground(t.headColor).Bold(true)
			baseStyle := lipgloss.NewStyle().Foreground(t.baseColor).Bold(true)
			descStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)

			line1 = arrowStyle.Render(arrow) + numStyle.Render(t.num) + " " + headStyle.Render(t.head) + " → " + baseStyle.Render(t.base)
			line2 = "      " + descStyle.Render(t.desc)
		}

		menuLines = append(menuLines, line1)
		menuLines = append(menuLines, line2)
		menuLines = append(menuLines, "")
	}

	// Get panel title
	panelTitle := " Select PR Type "
	if m.repoInfo != nil {
		panelTitle = fmt.Sprintf(" %s ", m.repoInfo.DisplayName)
	} else if m.mode != nil && *m.mode == ModeBatch {
		panelTitle = " Batch Mode "
	}

	menuTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorCyan)
	menuContent := menuTitleStyle.Render(panelTitle) + "\n" + strings.Join(menuLines, "\n")

	// Build right column (info panel)
	var infoLines []string
	infoLines = append(infoLines, "")

	if m.menuIndex == 0 {
		greenStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true)
		yellowStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		arrowStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite).Bold(true)
		infoLines = append(infoLines, "  "+greenStyle.Render("dev")+arrowStyle.Render(" → ")+yellowStyle.Render("staging"))
		infoLines = append(infoLines, "")
		infoLines = append(infoLines, "  Merge feature branches into")
		infoLines = append(infoLines, "  staging for QA testing.")
		infoLines = append(infoLines, "")
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		infoLines = append(infoLines, labelStyle.Render("  Base: ")+yellowStyle.Render("staging"))
		infoLines = append(infoLines, labelStyle.Render("  Head: ")+greenStyle.Render("dev"))
	} else {
		yellowStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		redStyle := lipgloss.NewStyle().Foreground(ui.ColorRed).Bold(true)
		arrowStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite).Bold(true)
		infoLines = append(infoLines, "  "+yellowStyle.Render("staging")+arrowStyle.Render(" → ")+redStyle.Render(mainBranch))
		infoLines = append(infoLines, "")
		infoLines = append(infoLines, "  Release staging changes to")
		infoLines = append(infoLines, "  production environment.")
		infoLines = append(infoLines, "")
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		infoLines = append(infoLines, labelStyle.Render("  Base: ")+redStyle.Render(mainBranch))
		infoLines = append(infoLines, labelStyle.Render("  Head: ")+yellowStyle.Render("staging"))
	}

	infoTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite)
	infoContent := infoTitleStyle.Render(" PR Details ") + "\n" + strings.Join(infoLines, "\n")

	return ui.UnifiedPanel(menuContent, infoContent, 48, 48, ui.ColorCyan)
}

func (m Model) renderLoading() string {
	return m.renderLoadingWithMessage(m.loadingMessage)
}

func (m Model) renderLoadingWithMessage(message string) string {
	spinner := ui.Spinner(m.spinnerFrame)
	spinnerStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
	textStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)

	loadingText := fmt.Sprintf("%s %s", spinnerStyle.Render(spinner), textStyle.Render(message))

	// Center the text within the box
	innerWidth := m.contentWidth() - 6
	centeredStyle := lipgloss.NewStyle().Width(innerWidth).Align(lipgloss.Center)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, centeredStyle.Render(loadingText))
	lines = append(lines, "")
	lines = append(lines, "")

	content := strings.Join(lines, "\n")

	// Purple border box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPurple).
		Width(m.contentWidth()).
		Padding(1, 2)

	return boxStyle.Render(content)
}

func (m Model) renderCommitReviewWithHeight(availableHeight int) string {
	// Fixed column sizing for stable layout
	columnWidth := (m.contentWidth() - 6) / 2
	panelHeight := availableHeight - 2
	if panelHeight < 10 {
		panelHeight = 10
	}

	mainBranch := "main"
	if m.repoInfo != nil {
		mainBranch = m.repoInfo.MainBranch
	}

	// Build LEFT column (PR info + title input + tickets)
	var leftLines []string

	// PR Info section
	if m.repoInfo != nil {
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		valueStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Bold(true)
		leftLines = append(leftLines, labelStyle.Render("  Repo: ")+valueStyle.Render(m.repoInfo.DisplayName))
	}

	if m.prType != nil {
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		arrowStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		headBranch := m.prType.HeadBranch()
		baseBranch := m.prType.BaseBranch(mainBranch)
		headStyle := lipgloss.NewStyle().Foreground(ui.BranchColor(headBranch)).Bold(true)
		baseStyle := lipgloss.NewStyle().Foreground(ui.BranchColor(baseBranch)).Bold(true)
		leftLines = append(leftLines, labelStyle.Render("  Type: ")+headStyle.Render(headBranch)+arrowStyle.Render(" → ")+baseStyle.Render(baseBranch))
	}

	leftLines = append(leftLines, "")

	// Title input section
	if len(m.commits) > 0 {
		titleSectionStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorYellow)
		leftLines = append(leftLines, titleSectionStyle.Render(" PR Title "))
		leftLines = append(leftLines, "")

		defaultTitle := ""
		if m.prType != nil {
			defaultTitle = m.prType.DefaultTitle(mainBranch)
		}

		borderStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)
		cursorStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)

		var displayText string
		var textColor lipgloss.Color
		if m.prTitle == "" {
			displayText = defaultTitle
			textColor = ui.ColorDarkGray
		} else {
			displayText = m.prTitle
			textColor = ui.ColorWhite
		}
		// Truncate display if too long (use rune count for proper Unicode width)
		innerWidth := 40
		maxLen := innerWidth - 1 // leave room for cursor
		displayRunes := utf8.RuneCountInString(displayText)
		if displayRunes > maxLen {
			// Truncate by runes, not bytes
			runes := []rune(displayText)
			displayText = string(runes[:maxLen])
			displayRunes = maxLen
		}
		textStyle := lipgloss.NewStyle().Foreground(textColor)
		padding := innerWidth - displayRunes - 1 // -1 for cursor

		leftLines = append(leftLines, borderStyle.Render("  ┌"+strings.Repeat("─", innerWidth)+"┐"))
		leftLines = append(leftLines, borderStyle.Render("  │")+textStyle.Render(displayText)+cursorStyle.Render("█")+strings.Repeat(" ", padding)+borderStyle.Render("│"))
		leftLines = append(leftLines, borderStyle.Render("  └"+strings.Repeat("─", innerWidth)+"┘"))
		leftLines = append(leftLines, "")
	}

	// Tickets section
	ticketTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite)
	leftLines = append(leftLines, ticketTitleStyle.Render(fmt.Sprintf(" Tickets (%d) ", len(m.tickets))))
	leftLines = append(leftLines, "")

	if len(m.tickets) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		leftLines = append(leftLines, dimStyle.Render("  No tickets found"))
	} else {
		ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		for _, ticket := range m.tickets {
			leftLines = append(leftLines, fmt.Sprintf("  🎫 %s", ticketStyle.Render(ticket)))
		}
	}

	leftLines = append(leftLines, "")
	if len(m.commits) > 0 {
		hintStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		enterStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true)
		leftLines = append(leftLines, hintStyle.Render("  Type to edit title"))
		leftLines = append(leftLines, hintStyle.Render("  Press ")+enterStyle.Render("Enter")+hintStyle.Render(" to create PR"))
	} else {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		leftLines = append(leftLines, dimStyle.Render("  Nothing to merge"))
	}

	leftTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorCyan)
	leftContent := leftTitleStyle.Render(" 🚀 Create PR ") + "\n" + strings.Join(leftLines, "\n")

	// Build RIGHT column (commits list)
	var commitLines []string
	commitLines = append(commitLines, "")

	// Max message length per line (account for indent)
	maxMsgLen := columnWidth - 14

	if len(m.commits) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		commitLines = append(commitLines, dimStyle.Render("  No commits to merge"))
	} else {
		ticketRegex := m.config.TicketRegex()

		for _, commit := range m.commits {
			hashStyle := lipgloss.NewStyle().Foreground(ui.ColorMagenta)
			ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)

			// Format: hash on line 1
			commitLines = append(commitLines,
				fmt.Sprintf("  %s", hashStyle.Render(commit.Hash)),
			)

			// Highlight tickets in yellow within the message
			msg := commit.Message
			styledMsg := msg
			if ticketRegex != nil {
				styledMsg = ticketRegex.ReplaceAllStringFunc(msg, func(match string) string {
					return ticketStyle.Render(match)
				})
			}

			// Wrap message to fit column, with indent
			indent := "    "
			words := strings.Fields(styledMsg)
			var line string
			for _, word := range words {
				testLine := line + " " + word
				if len(strings.TrimSpace(testLine)) > maxMsgLen && line != "" {
					commitLines = append(commitLines, indent+strings.TrimSpace(line))
					line = word
				} else {
					line = testLine
				}
			}
			if strings.TrimSpace(line) != "" {
				commitLines = append(commitLines, indent+strings.TrimSpace(line))
			}

			commitLines = append(commitLines, "") // spacing between commits
		}
	}

	commitTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorCyan)
	commitContent := commitTitleStyle.Render(fmt.Sprintf(" %d commits ", len(m.commits))) + "\n" + strings.Join(commitLines, "\n")

	// Use ColumnBox for consistent sizing - purple outer borders for consistency
	leftColumn := ui.ColumnBox(leftContent, "", ui.ColorPurple, true, columnWidth, panelHeight)
	rightColumn := ui.ColumnBox(commitContent, "", ui.ColorPurple, false, columnWidth-10, panelHeight)

	return ui.TwoColumns(leftColumn, rightColumn, 2)
}

func (m Model) renderTitleInput() string {
	mainBranch := "main"
	if m.repoInfo != nil {
		mainBranch = m.repoInfo.MainBranch
	}

	defaultTitle := ""
	if m.prType != nil {
		defaultTitle = m.prType.DefaultTitle(mainBranch)
	}

	// Build left column (title input)
	var leftLines []string
	leftLines = append(leftLines, "")

	// Show branch flow
	if m.prType != nil {
		leftLines = append(leftLines, ui.BranchFlowDiagram(m.prType.HeadBranch(), m.prType.BaseBranch(mainBranch)))
		leftLines = append(leftLines, "")
	}

	leftLines = append(leftLines, ui.SectionHeader("ENTER TITLE", ui.ColorCyan))
	leftLines = append(leftLines, "")

	// Input box with yellow border
	borderStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)
	cursorStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)

	var displayText string
	var textColor lipgloss.Color
	if m.prTitle == "" {
		displayText = fmt.Sprintf("%s (default)", defaultTitle)
		textColor = ui.ColorDarkGray
	} else {
		displayText = m.prTitle
		textColor = ui.ColorWhite
	}
	textStyle := lipgloss.NewStyle().Foreground(textColor)

	leftLines = append(leftLines, borderStyle.Render("  ┌")+borderStyle.Render(strings.Repeat("─", 38))+borderStyle.Render("┐"))
	leftLines = append(leftLines, borderStyle.Render("  │ ")+textStyle.Render(displayText)+cursorStyle.Render("█"))
	leftLines = append(leftLines, borderStyle.Render("  └")+borderStyle.Render(strings.Repeat("─", 38))+borderStyle.Render("┘"))
	leftLines = append(leftLines, "")

	hintStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
	enterStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true)
	escStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
	leftLines = append(leftLines, hintStyle.Render("  Press ")+enterStyle.Render("Enter")+hintStyle.Render(" to continue"))
	leftLines = append(leftLines, hintStyle.Render("  ")+escStyle.Render("Esc")+hintStyle.Render(" to go back"))

	leftTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorYellow)
	leftContent := leftTitleStyle.Render(" PR Title ") + "\n" + strings.Join(leftLines, "\n")

	// Build right column (context)
	var rightLines []string
	rightLines = append(rightLines, "")
	rightLines = append(rightLines, ui.SectionHeader("CONTEXT", ui.ColorMagenta))
	rightLines = append(rightLines, "")

	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)

	if m.mode != nil && *m.mode == ModeBatch {
		// Batch mode - show selected repos and tickets
		selectedCount := 0
		var selectedNames []string
		for i, selected := range m.batchSelected {
			if selected && i < len(m.batchRepos) {
				selectedCount++
				selectedNames = append(selectedNames, m.batchRepos[i].DisplayName)
			}
		}
		repoStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Bold(true)
		ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		rightLines = append(rightLines, labelStyle.Render("  Repos: ")+repoStyle.Render(fmt.Sprintf("%d selected", selectedCount)))
		rightLines = append(rightLines, labelStyle.Render("  Tickets: ")+ticketStyle.Render(fmt.Sprintf("%d", len(m.tickets))))
		rightLines = append(rightLines, "")

		// Show tickets if any
		if len(m.tickets) > 0 {
			rightLines = append(rightLines, ui.SectionHeader("TICKETS", ui.ColorYellow))
			rightLines = append(rightLines, "")
			for i, ticket := range m.tickets {
				if i >= 6 {
					remaining := len(m.tickets) - 6
					rightLines = append(rightLines, fmt.Sprintf("  ... and %d more", remaining))
					break
				}
				rightLines = append(rightLines, fmt.Sprintf("  %s", ticketStyle.Render(ticket)))
			}
			rightLines = append(rightLines, "")
		}

		// Show selected repo names
		if len(selectedNames) > 0 {
			rightLines = append(rightLines, ui.SectionHeader("REPOS", ui.ColorCyan))
			rightLines = append(rightLines, "")
			for i, name := range selectedNames {
				if i >= 5 {
					remaining := len(selectedNames) - 5
					rightLines = append(rightLines, fmt.Sprintf("  ... and %d more", remaining))
					break
				}
				rightLines = append(rightLines, fmt.Sprintf("  • %s", repoStyle.Render(name)))
			}
		}
	} else {
		// Single mode - show repo and commits
		if m.repoInfo != nil {
			valueStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
			rightLines = append(rightLines, labelStyle.Render("  Repo: ")+valueStyle.Render(m.repoInfo.DisplayName))
		}

		commitStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Bold(true)
		ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		rightLines = append(rightLines, labelStyle.Render("  Commits: ")+commitStyle.Render(fmt.Sprintf("%d", len(m.commits))))
		rightLines = append(rightLines, labelStyle.Render("  Tickets: ")+ticketStyle.Render(fmt.Sprintf("%d", len(m.tickets))))
		rightLines = append(rightLines, "")

		// Tickets preview
		if len(m.tickets) > 0 {
			rightLines = append(rightLines, ui.SectionHeader("TICKETS", ui.ColorYellow))
			rightLines = append(rightLines, "")
			for i, ticket := range m.tickets {
				if i >= 5 {
					remaining := len(m.tickets) - 5
					rightLines = append(rightLines, fmt.Sprintf("  ... and %d more", remaining))
					break
				}
				rightLines = append(rightLines, fmt.Sprintf("  %s", ticketStyle.Render(ticket)))
			}
		}
	}

	rightTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorMagenta)
	rightContent := rightTitleStyle.Render(" Context ") + "\n" + strings.Join(rightLines, "\n")

	return ui.UnifiedPanel(leftContent, rightContent, 60, 35, ui.ColorYellow)
}

func (m Model) renderConfirmation() string {
	mainBranch := "main"
	if m.repoInfo != nil {
		mainBranch = m.repoInfo.MainBranch
	}

	// Build left column (PR details)
	var leftLines []string
	leftLines = append(leftLines, "")

	// Show branch flow diagram
	if m.prType != nil {
		leftLines = append(leftLines, ui.BranchFlowDiagram(m.prType.HeadBranch(), m.prType.BaseBranch(mainBranch)))
		leftLines = append(leftLines, "")
	}

	leftLines = append(leftLines, ui.SectionHeader("PR DETAILS", ui.ColorCyan))
	leftLines = append(leftLines, "")

	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
	titleStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite).Bold(true)
	leftLines = append(leftLines, fmt.Sprintf("  📝 %s %s", labelStyle.Render("Title:"), titleStyle.Render(m.prTitle)))

	if m.repoInfo != nil {
		repoStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
		leftLines = append(leftLines, fmt.Sprintf("  📦 %s %s", labelStyle.Render("Repo: "), repoStyle.Render(m.repoInfo.DisplayName)))
	}

	leftLines = append(leftLines, "")

	// PR body preview section
	leftLines = append(leftLines, ui.SectionHeader("PR BODY PREVIEW", ui.ColorYellow))
	leftLines = append(leftLines, "")

	if len(m.tickets) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		leftLines = append(leftLines, dimStyle.Render("  (empty)"))
	} else {
		leftLines = append(leftLines, "  # Tickets")
		leftLines = append(leftLines, "")
		for _, ticket := range m.tickets {
			ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)
			urlStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
			linearURL := fmt.Sprintf("https://linear.app/%s/issue/%s", m.config.Tickets.LinearOrg, strings.ToLower(ticket))
			leftLines = append(leftLines, fmt.Sprintf("  ### - Closes %s%s", ticketStyle.Render(fmt.Sprintf("[%s]", ticket)), urlStyle.Render(fmt.Sprintf("(%s)", linearURL))))
		}
	}

	leftLines = append(leftLines, "")

	// Confirm section
	leftLines = append(leftLines, ui.SectionHeader("CONFIRM", ui.ColorGreen))
	leftLines = append(leftLines, "")

	// Show different message for create vs update
	isUpdate := m.existingPR != nil
	if isUpdate {
		warningStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		leftLines = append(leftLines, warningStyle.Render("  ⚠ PR already exists - will update"))
		leftLines = append(leftLines, "")
		leftLines = append(leftLines, "  Update this PR?")
	} else {
		leftLines = append(leftLines, "  Create this PR?")
	}
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, ui.YesNoButtons(m.confirmSelection))

	leftTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorCyan)
	panelTitle := " 🚀 Create PR "
	if isUpdate {
		panelTitle = " 🔄 Update PR "
	}
	leftContent := leftTitleStyle.Render(panelTitle) + "\n" + strings.Join(leftLines, "\n")

	// Build right column (summary)
	var rightLines []string
	rightLines = append(rightLines, "")

	// Branch flow
	if m.prType != nil {
		headBranch := m.prType.HeadBranch()
		baseBranch := m.prType.BaseBranch(mainBranch)
		headStyle := lipgloss.NewStyle().Foreground(ui.BranchColor(headBranch)).Bold(true)
		baseStyle := lipgloss.NewStyle().Foreground(ui.BranchColor(baseBranch)).Bold(true)
		arrowStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		rightLines = append(rightLines, fmt.Sprintf("  %s %s %s", headStyle.Render(headBranch), arrowStyle.Render("→"), baseStyle.Render(baseBranch)))
		rightLines = append(rightLines, "")
	}

	// Repo
	if m.repoInfo != nil {
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		valueStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Bold(true)
		rightLines = append(rightLines, fmt.Sprintf("  %s %s", labelStyle.Render("Repo:"), valueStyle.Render(m.repoInfo.DisplayName)))
	}

	// Title preview
	if m.prTitle != "" {
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		titleStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		title := m.prTitle
		if len(title) > 25 {
			title = title[:22] + "..."
		}
		rightLines = append(rightLines, fmt.Sprintf("  %s %s", labelStyle.Render("Title:"), titleStyle.Render(title)))
	}

	rightLines = append(rightLines, "")
	rightLines = append(rightLines, ui.SectionHeader("STATS", ui.ColorMagenta))
	rightLines = append(rightLines, "")

	commitStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Bold(true)
	ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
	rightLines = append(rightLines, fmt.Sprintf("  📊 %s commits", commitStyle.Render(fmt.Sprintf("%d", len(m.commits)))))
	rightLines = append(rightLines, fmt.Sprintf("  🎫 %s tickets", ticketStyle.Render(fmt.Sprintf("%d", len(m.tickets)))))

	// List tickets
	if len(m.tickets) > 0 {
		rightLines = append(rightLines, "")
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		for _, ticket := range m.tickets {
			rightLines = append(rightLines, fmt.Sprintf("     %s %s", dimStyle.Render("•"), ticketStyle.Render(ticket)))
		}
	}

	if m.dryRun {
		rightLines = append(rightLines, "")
		warningStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		rightLines = append(rightLines, warningStyle.Render("  ⚠ DRY RUN MODE"))
	}

	rightTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorMagenta)
	rightContent := rightTitleStyle.Render(" 📊 Summary ") + "\n" + strings.Join(rightLines, "\n")

	return ui.UnifiedPanel(leftContent, rightContent, 60, 35, ui.ColorCyan)
}

func (m Model) renderCreating() string {
	mainBranch := "main"
	if m.repoInfo != nil {
		mainBranch = m.repoInfo.MainBranch
	}

	var lines []string
	lines = append(lines, "")

	// Main status
	spinner := ui.Spinner(m.spinnerFrame)
	spinnerStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
	statusStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Bold(true)
	lines = append(lines, fmt.Sprintf("  %s %s", spinnerStyle.Render(spinner), statusStyle.Render("Creating Pull Request...")))
	lines = append(lines, "")

	// Details section
	if m.prType != nil && m.repoInfo != nil {
		lines = append(lines, ui.SectionHeader("DETAILS", ui.ColorMagenta))
		lines = append(lines, "")

		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		repoStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
		headStyle := lipgloss.NewStyle().Foreground(ui.BranchColor(m.prType.HeadBranch())).Bold(true)
		baseStyle := lipgloss.NewStyle().Foreground(ui.BranchColor(m.prType.BaseBranch(mainBranch))).Bold(true)
		titleStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)

		lines = append(lines, labelStyle.Render("  Repo:   ")+repoStyle.Render(m.repoInfo.DisplayName))
		lines = append(lines, labelStyle.Render("  Branch: ")+headStyle.Render(m.prType.HeadBranch())+labelStyle.Render(" -> ")+baseStyle.Render(m.prType.BaseBranch(mainBranch)))
		lines = append(lines, labelStyle.Render("  Title:  ")+titleStyle.Render(m.prTitle))
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorCyan)
	return titleStyle.Render(" Creating PR ") + "\n" + strings.Join(lines, "\n")
}

func (m Model) renderComplete() string {
	var lines []string

	// Use pulsing green effect based on sine wave
	var successColor lipgloss.Color
	pulseIntensity := (math.Sin(m.pulsePhase) + 1.0) / 2.0
	if pulseIntensity > 0.5 {
		successColor = ui.ColorGreen
	} else {
		successColor = ui.ColorLightGreen
	}

	// Typewriter effect for message
	message := "PR Created Successfully!"
	revealedChars := m.typewriterPos
	if revealedChars > len(message) {
		revealedChars = len(message)
	}
	revealedText := message[:revealedChars]

	successStyle := lipgloss.NewStyle().Foreground(successColor).Bold(true)
	iconStyle := lipgloss.NewStyle().Foreground(successColor).Bold(true)
	urlStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %s %s", iconStyle.Render("✓"), successStyle.Render(revealedText)))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  🔗 %s", urlStyle.Render(m.prURL)))
	lines = append(lines, "")

	// Render confetti
	confettiLines := m.renderConfetti()
	if confettiLines != "" {
		lines = append(lines, confettiLines)
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorGreen)
	return titleStyle.Render(" 🎉 Success ") + "\n" + strings.Join(lines, "\n")
}

func (m Model) renderConfetti() string {
	if len(m.confetti) == 0 {
		return ""
	}

	// Create a grid for confetti
	width := 80
	height := 5
	grid := make([][]rune, height)
	colors := make([][]lipgloss.Color, height)
	for i := range grid {
		grid[i] = make([]rune, width)
		colors[i] = make([]lipgloss.Color, width)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	// Place particles in grid
	for _, p := range m.confetti {
		x := int(p.X)
		y := int(p.Y) - 5 // offset for display area
		if x >= 0 && x < width && y >= 0 && y < height {
			grid[y][x] = p.Char
			colors[y][x] = p.Color
		}
	}

	// Render grid
	var lines []string
	for y := 0; y < height; y++ {
		var line strings.Builder
		line.WriteString("   ")
		for x := 0; x < width; x++ {
			if grid[y][x] != ' ' {
				style := lipgloss.NewStyle().Foreground(colors[y][x])
				line.WriteString(style.Render(string(grid[y][x])))
			} else {
				line.WriteRune(' ')
			}
		}
		lines = append(lines, line.String())
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderError() string {
	var lines []string

	errorStyle := lipgloss.NewStyle().Foreground(ui.ColorRed).Bold(true)

	lines = append(lines, "")
	lines = append(lines, errorStyle.Render("   ✗ Error"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("   %s", m.errorMessage))
	lines = append(lines, "")
	lines = append(lines, "   Press Enter to go back")

	return strings.Join(lines, "\n")
}

func (m Model) renderBatchRepoSelectWithHeight(availableHeight int) string {
	selectedCount := 0
	for _, s := range m.batchSelected {
		if s {
			selectedCount++
		}
	}

	// Fixed column width for stable layout
	columnWidth := (m.contentWidth() - 6) / 2

	// Reserve space for commits panel (5 lines) + filter box (4 lines) + gaps (4)
	commitsHeight := 5
	columnHeight := availableHeight - commitsHeight - 8
	if columnHeight < 5 {
		columnHeight = 5
	}

	// Filter width matches the two columns + gap
	filterWidth := columnWidth*2 + 2

	// Filter input at top
	title := fmt.Sprintf("Select Repositories (%d/%d)", selectedCount, len(m.batchRepos))
	filterBox := ui.FilterInput(m.batchFilter, title, ui.ColorWhite, filterWidth)

	// Get filtered repos for each column
	feFiltered := m.getFilteredBatchRepos(0)
	beFiltered := m.getFilteredBatchRepos(1)

	// Track highlighted repo index for commits panel
	var highlightedRepoIdx int = -1

	// Build Frontend column - track line index for highlighted item
	var feLines []string
	feHighlightedLine := -1
	feLines = append(feLines, ui.SectionHeader(fmt.Sprintf("🖥️  FRONTEND (%d)", len(feFiltered)), ui.ColorCyan))
	feLines = append(feLines, "")

	if len(feFiltered) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		feLines = append(feLines, dimStyle.Render("  No repos found"))
	} else {
		var feCurrentParent *string
		for i, repoIdx := range feFiltered {
			repo := m.batchRepos[repoIdx]

			// Show parent header when parent changes (only when not filtering)
			if m.batchFilter == "" && !ptrEqual(repo.ParentRepo, feCurrentParent) {
				if repo.ParentRepo != nil {
					feLines = append(feLines, ui.ParentHeader(*repo.ParentRepo))
				}
				feCurrentParent = repo.ParentRepo
			}

			name := repo.DisplayName
			if idx := strings.LastIndex(name, "/"); idx != -1 {
				name = name[idx+1:]
			}
			selected := false
			if repoIdx < len(m.batchSelected) {
				selected = m.batchSelected[repoIdx]
			}
			highlighted := m.batchColumn == 0 && m.batchFEIndex == i
			if highlighted {
				feHighlightedLine = len(feLines)
				highlightedRepoIdx = repoIdx
			}

			// Get commit count: -1 = loading, 0 = no commits, >0 = has commits
			commitCount := -1 // Default to loading
			if repoIdx < len(m.batchRepoCommits) && m.batchRepoCommits[repoIdx] != nil {
				commitCount = len(*m.batchRepoCommits[repoIdx])
			}

			// Indent nested repos
			indent := ""
			if repo.ParentRepo != nil {
				indent = "│ "
			}
			feLines = append(feLines, ui.RepoListItemWithCommits(name, selected, highlighted, ui.ColorCyan, indent, commitCount, m.spinnerFrame))
		}
	}

	// Build Backend column - track line index for highlighted item
	var beLines []string
	beHighlightedLine := -1
	beLines = append(beLines, ui.SectionHeader(fmt.Sprintf("⚙️  BACKEND (%d)", len(beFiltered)), ui.ColorMagenta))
	beLines = append(beLines, "")

	if len(beFiltered) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		beLines = append(beLines, dimStyle.Render("  No repos found"))
	} else {
		var beCurrentParent *string
		for i, repoIdx := range beFiltered {
			repo := m.batchRepos[repoIdx]

			// Show parent header when parent changes (only when not filtering)
			if m.batchFilter == "" && !ptrEqual(repo.ParentRepo, beCurrentParent) {
				if repo.ParentRepo != nil {
					beLines = append(beLines, ui.ParentHeader(*repo.ParentRepo))
				}
				beCurrentParent = repo.ParentRepo
			}

			name := repo.DisplayName
			if idx := strings.LastIndex(name, "/"); idx != -1 {
				name = name[idx+1:]
			}
			selected := false
			if repoIdx < len(m.batchSelected) {
				selected = m.batchSelected[repoIdx]
			}
			highlighted := m.batchColumn == 1 && m.batchBEIndex == i
			if highlighted {
				beHighlightedLine = len(beLines)
				highlightedRepoIdx = repoIdx
			}

			// Get commit count: -1 = loading, 0 = no commits, >0 = has commits
			commitCount := -1 // Default to loading
			if repoIdx < len(m.batchRepoCommits) && m.batchRepoCommits[repoIdx] != nil {
				commitCount = len(*m.batchRepoCommits[repoIdx])
			}

			// Indent nested repos
			indent := ""
			if repo.ParentRepo != nil {
				indent = "│ "
			}
			beLines = append(beLines, ui.RepoListItemWithCommits(name, selected, highlighted, ui.ColorMagenta, indent, commitCount, m.spinnerFrame))
		}
	}

	// Apply viewport scrolling to keep highlighted item visible
	// Keep 2-line header, scroll the rest
	headerLines := 2
	visibleContentLines := columnHeight - headerLines
	if visibleContentLines < 1 {
		visibleContentLines = 1
	}

	feContent := applyViewportScroll(feLines, headerLines, feHighlightedLine, visibleContentLines)
	beContent := applyViewportScroll(beLines, headerLines, beHighlightedLine, visibleContentLines)

	feColumn := ui.ColumnBox(feContent, "", ui.ColorCyan, m.batchColumn == 0, columnWidth, columnHeight)
	beColumn := ui.ColumnBox(beContent, "", ui.ColorMagenta, m.batchColumn == 1, columnWidth, columnHeight)

	columns := ui.TwoColumns(feColumn, beColumn, 2)

	// Build commits preview panel for highlighted repo
	commitsPanel := m.renderCommitsPreview(highlightedRepoIdx, filterWidth)

	return filterBox + "\n\n" + columns + "\n" + commitsPanel
}

// renderCommitsPreview renders a preview of commits for the given repo index
func (m Model) renderCommitsPreview(repoIdx int, width int) string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorDarkGray).
		Width(width).
		Padding(0, 1)

	if repoIdx < 0 || repoIdx >= len(m.batchRepos) {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		return borderStyle.Render(dimStyle.Render("No repo selected"))
	}

	repo := m.batchRepos[repoIdx]

	// Check if still loading
	isLoading := repoIdx >= len(m.batchRepoCommits) || m.batchRepoCommits[repoIdx] == nil

	// Build content
	var lines []string

	// Header with repo name
	repoName := repo.DisplayName
	if idx := strings.LastIndex(repoName, "/"); idx != -1 {
		repoName = repoName[idx+1:]
	}
	headerStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)

	if isLoading {
		// Show loading state
		spinner := ui.Spinner(m.spinnerFrame)
		spinnerStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)
		lines = append(lines, headerStyle.Render(repoName)+" "+spinnerStyle.Render(spinner+" fetching..."))
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		lines = append(lines, dimStyle.Render("  Checking for commits..."))
	} else {
		commits := *m.batchRepoCommits[repoIdx]
		countStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
		lines = append(lines, headerStyle.Render(repoName)+" "+countStyle.Render(fmt.Sprintf("(%d commits)", len(commits))))

		if len(commits) == 0 {
			dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
			lines = append(lines, dimStyle.Render("  No commits to merge - branches are up to date"))
		} else {
			// Show first 3 commits
			hashStyle := lipgloss.NewStyle().Foreground(ui.ColorMagenta)
			msgStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
			ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)
			ticketRegex := m.config.TicketRegex()

			maxCommits := 3
			for i, commit := range commits {
				if i >= maxCommits {
					remaining := len(commits) - maxCommits
					dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
					lines = append(lines, dimStyle.Render(fmt.Sprintf("  ... and %d more", remaining)))
					break
				}
				// Truncate message to fit
				msg := commit.Message
				maxMsgLen := width - 15 // room for hash and padding
				if len(msg) > maxMsgLen {
					msg = msg[:maxMsgLen-3] + "..."
				}
				// Highlight tickets in message
				styledMsg := msg
				if ticketRegex != nil {
					styledMsg = ticketRegex.ReplaceAllStringFunc(msg, func(match string) string {
						return ticketStyle.Render(match)
					})
				}
				lines = append(lines, fmt.Sprintf("  %s %s", hashStyle.Render(commit.Hash), msgStyle.Render(styledMsg)))
			}
		}
	}

	return borderStyle.Render(strings.Join(lines, "\n"))
}

// applyViewportScroll scrolls content to keep the highlighted line visible
func applyViewportScroll(lines []string, headerLines int, highlightedLine int, visibleLines int) string {
	if len(lines) <= headerLines+visibleLines {
		// No scrolling needed
		return strings.Join(lines, "\n")
	}

	// Keep header lines fixed
	header := lines[:headerLines]
	content := lines[headerLines:]

	scrollOffset := 0

	if highlightedLine >= headerLines {
		// Calculate scroll offset to keep highlighted line visible
		highlightInContent := highlightedLine - headerLines

		// Keep some padding around the highlighted item
		padding := 2
		if highlightInContent >= visibleLines-padding {
			scrollOffset = highlightInContent - visibleLines + padding + 1
		}
		if scrollOffset > len(content)-visibleLines {
			scrollOffset = len(content) - visibleLines
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}
	}

	endOffset := scrollOffset + visibleLines
	if endOffset > len(content) {
		endOffset = len(content)
	}

	// Build visible content with scroll indicators (copy to avoid mutating original)
	visibleContent := make([]string, endOffset-scrollOffset)
	copy(visibleContent, content[scrollOffset:endOffset])

	// Add scroll indicators
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
	hasAbove := scrollOffset > 0
	hasBelow := endOffset < len(content)

	if hasAbove {
		visibleContent[0] = dimStyle.Render("  ▲ more above")
	}
	if hasBelow {
		visibleContent[len(visibleContent)-1] = dimStyle.Render("  ▼ more below")
	}

	return strings.Join(append(header, visibleContent...), "\n")
}

func (m Model) renderBatchConfirmationWithHeight(availableHeight int) string {
	selectedCount := 0
	for _, s := range m.batchSelected {
		if s {
			selectedCount++
		}
	}

	// Calculate dynamic limit for left column repos based on available height
	maxReposLeft := (availableHeight - 12) / 1
	if maxReposLeft < 3 {
		maxReposLeft = 3
	} else if maxReposLeft > 10 {
		maxReposLeft = 10
	}

	// Get selected repo names
	var selectedRepos []string
	for i, repo := range m.batchRepos {
		if i < len(m.batchSelected) && m.batchSelected[i] {
			// Get just the repo name (last part)
			name := repo.DisplayName
			if idx := strings.LastIndex(name, "/"); idx != -1 {
				name = name[idx+1:]
			}
			selectedRepos = append(selectedRepos, name)
		}
	}

	// Build left column (PR details & repos)
	var leftLines []string
	leftLines = append(leftLines, "")

	// Branch flow diagram
	if m.prType != nil {
		leftLines = append(leftLines, ui.BranchFlowDiagram(m.prType.HeadBranch(), m.prType.BaseBranch("main")))
		leftLines = append(leftLines, "")
	}

	leftLines = append(leftLines, ui.SectionHeader("PR DETAILS", ui.ColorCyan))
	leftLines = append(leftLines, "")

	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
	titleStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite).Bold(true)
	leftLines = append(leftLines, fmt.Sprintf("  📝 %s %s", labelStyle.Render("Title:"), titleStyle.Render(m.prTitle)))
	leftLines = append(leftLines, "")

	// Repos section
	leftLines = append(leftLines, ui.SectionHeader(fmt.Sprintf("REPOSITORIES (%d)", selectedCount), ui.ColorMagenta))
	leftLines = append(leftLines, "")

	// List selected repos (dynamic limit based on height)
	for i, name := range selectedRepos {
		if i >= maxReposLeft {
			remaining := len(selectedRepos) - maxReposLeft
			leftLines = append(leftLines, fmt.Sprintf("    ... and %d more", remaining))
			break
		}
		leftLines = append(leftLines, fmt.Sprintf("  %s", name))
	}
	leftLines = append(leftLines, "")

	// Confirm section
	leftLines = append(leftLines, ui.SectionHeader("CONFIRM", ui.ColorGreen))
	leftLines = append(leftLines, "")

	// Calculate repos to skip (no commits)
	reposToSkip := selectedCount - m.batchReposWithCommits

	// Show warning if ALL repos will be skipped
	if m.batchReposWithCommits == 0 {
		warningStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		leftLines = append(leftLines, warningStyle.Render("  ⊘ All repos already up to date"))
		leftLines = append(leftLines, "")
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		leftLines = append(leftLines, dimStyle.Render("  Nothing to merge"))
	} else {
		// Show warning if some repos will be skipped
		if reposToSkip > 0 {
			warningStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
			leftLines = append(leftLines, warningStyle.Render(fmt.Sprintf("  ⊘ %d repo(s) will be skipped - already up to date", reposToSkip)))
			leftLines = append(leftLines, "")
		}

		// Show warning if some PRs already exist
		if m.batchExistingPRs > 0 {
			warningStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
			leftLines = append(leftLines, warningStyle.Render(fmt.Sprintf("  ⚠ %d PR(s) already exist - will update", m.batchExistingPRs)))
			leftLines = append(leftLines, "")
		}

		newPRs := m.batchReposWithCommits - m.batchExistingPRs
		if newPRs > 0 && m.batchExistingPRs > 0 {
			leftLines = append(leftLines, fmt.Sprintf("  Create %d, update %d PRs?", newPRs, m.batchExistingPRs))
		} else if m.batchExistingPRs > 0 {
			leftLines = append(leftLines, fmt.Sprintf("  Update %d PRs?", m.batchExistingPRs))
		} else {
			leftLines = append(leftLines, fmt.Sprintf("  Create %d PRs?", m.batchReposWithCommits))
		}
		leftLines = append(leftLines, "")
		leftLines = append(leftLines, ui.YesNoButtons(m.confirmSelection))
	}

	leftTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorCyan)
	panelTitle := " 🚀 Batch PRs "
	if m.batchExistingPRs == selectedCount {
		panelTitle = " 🔄 Update PRs "
	}

	leftContent := leftTitleStyle.Render(panelTitle) + "\n" + strings.Join(leftLines, "\n")

	// Calculate max height for right column to match left column height
	leftHeight := len(leftLines) + 1 // +1 for title

	// Build right column (commits & tickets per repo) - build ALL content first
	var rightLines []string

	repoNameStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Bold(true)
	ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)
	hashStyle := lipgloss.NewStyle().Foreground(ui.ColorMagenta)
	commitStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)

	// Show commits and tickets per selected repo (no limit - we'll scroll)
	for i, repo := range m.batchRepos {
		if i >= len(m.batchSelected) || !m.batchSelected[i] {
			continue
		}

		// Get commits for this repo
		var commits []models.CommitInfo
		if i < len(m.batchRepoCommits) && m.batchRepoCommits[i] != nil {
			commits = *m.batchRepoCommits[i]
		}

		// Skip repos with no commits
		if len(commits) == 0 {
			continue
		}

		// Repo name header
		name := repo.DisplayName
		if idx := strings.LastIndex(name, "/"); idx != -1 {
			name = name[idx+1:]
		}
		rightLines = append(rightLines, fmt.Sprintf("  %s", repoNameStyle.Render(name)))

		// Show commits with tickets (limit to 3 per repo for readability)
		maxCommits := 3
		for j, commit := range commits {
			if j >= maxCommits {
				if len(commits) > maxCommits {
					rightLines = append(rightLines, dimStyle.Render(fmt.Sprintf("      +%d more commits", len(commits)-maxCommits)))
				}
				break
			}

			// Format: hash message (with ticket highlighted if present)
			msg := commit.Message
			maxMsgLen := 55
			if len(msg) > maxMsgLen {
				msg = msg[:maxMsgLen-3] + "..."
			}

			// Highlight ticket in message if present
			if len(commit.Tickets) > 0 {
				for _, ticket := range commit.Tickets {
					msg = strings.Replace(msg, ticket, ticketStyle.Render(ticket), 1)
				}
			}

			rightLines = append(rightLines, fmt.Sprintf("    %s %s", hashStyle.Render(commit.Hash), commitStyle.Render(msg)))
		}
		rightLines = append(rightLines, "")
	}

	// Tickets summary at bottom
	if len(m.tickets) > 0 {
		rightLines = append(rightLines, ui.SectionHeader("TICKETS", ui.ColorYellow))
		// List all tickets (scrollable now)
		for _, ticket := range m.tickets {
			rightLines = append(rightLines, fmt.Sprintf("  🎫 %s", ticketStyle.Render(ticket)))
		}
	}

	if m.dryRun {
		rightLines = append(rightLines, "")
		warningStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		rightLines = append(rightLines, warningStyle.Render("  ⚠ DRY RUN MODE"))
	}

	// Apply scrolling to right column - constrain to left column height
	visibleHeight := leftHeight - 2 // -2 for title overhead
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	totalLines := len(rightLines)
	maxScroll := totalLines - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Clamp scroll offset
	scrollOffset := m.batchConfirmScroll
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Get visible window of lines with consistent height
	var visibleLines []string
	visibleLines = append(visibleLines, "") // Top padding

	// Always reserve space for scroll up indicator
	if scrollOffset > 0 {
		visibleLines = append(visibleLines, dimStyle.Render("  ↑ more above"))
	} else {
		visibleLines = append(visibleLines, "") // Empty line to maintain height
	}

	// Calculate visible portion (account for indicator lines)
	contentHeight := visibleHeight - 2 // Reserve 2 lines for indicators
	if contentHeight < 3 {
		contentHeight = 3
	}

	endIdx := scrollOffset + contentHeight
	if endIdx > totalLines {
		endIdx = totalLines
	}
	if scrollOffset < totalLines {
		visibleLines = append(visibleLines, rightLines[scrollOffset:endIdx]...)
	}

	// Pad to consistent height
	for len(visibleLines) < contentHeight+2 {
		visibleLines = append(visibleLines, "")
	}

	// Always reserve space for scroll down indicator
	if endIdx < totalLines {
		visibleLines = append(visibleLines, dimStyle.Render("  ↓ more below"))
	} else {
		visibleLines = append(visibleLines, "") // Empty line to maintain height
	}

	rightTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorMagenta)
	rightContent := rightTitleStyle.Render(" 📋 Changes ") + "\n" + strings.Join(visibleLines, "\n")

	return ui.UnifiedPanel(leftContent, rightContent, 50, 45, ui.ColorCyan)
}

func (m Model) renderBatchProcessing() string {
	var lines []string

	// Header with count - use selected count, not total repos
	// len(batchResults) = completed, +1 if currently processing one
	processedCount := len(m.batchResults)
	if m.batchCurrentRepo != "" {
		processedCount++
	}
	countStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
	header := fmt.Sprintf("Processing Repositories %s", countStyle.Render(fmt.Sprintf("(%d/%d)", processedCount, m.batchTotal)))
	lines = append(lines, ui.SectionHeader(header, ui.ColorMagenta))
	lines = append(lines, "")

	// Current repo being processed
	spinner := ui.Spinner(m.spinnerFrame)
	spinnerStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
	repoStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
	stepStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)

	if m.batchCurrentRepo != "" {
		lines = append(lines, fmt.Sprintf("   %s Processing %s...",
			spinnerStyle.Render(spinner),
			repoStyle.Render(m.batchCurrentRepo),
		))
		// Show current step if available
		if m.batchCurrentStep != "" {
			lines = append(lines, fmt.Sprintf("      → %s", stepStyle.Render(m.batchCurrentStep)))
		}
	}
	lines = append(lines, "")

	// Completed results log
	if len(m.batchResults) > 0 {
		lines = append(lines, ui.SectionHeader("Completed", ui.ColorWhite))
		lines = append(lines, "")

		for _, result := range m.batchResults {
			var icon string
			var statusText string
			var color lipgloss.Color

			if models.IsStatusCreated(result.Status) {
				icon = "✓"
				statusText = "PR created"
				color = ui.ColorGreen
			} else if models.IsStatusUpdated(result.Status) {
				icon = "✓"
				statusText = "PR updated"
				color = ui.ColorGreen
			} else if models.IsStatusSkipped(result.Status) {
				icon = "⊘"
				statusText = models.GetStatusReason(result.Status)
				color = ui.ColorYellow
			} else if models.IsStatusFailed(result.Status) {
				icon = "✗"
				statusText = models.GetStatusReason(result.Status)
				color = ui.ColorRed
			}

			iconStyle := lipgloss.NewStyle().Foreground(color)
			repoNameStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
			statusStyle := lipgloss.NewStyle().Foreground(color)

			lines = append(lines, fmt.Sprintf("   %s %s: %s",
				iconStyle.Render(icon),
				repoNameStyle.Render(result.Repo.DisplayName),
				statusStyle.Render(statusText),
			))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderBatchSummaryWithHeight(availableHeight int) string {
	var lines []string

	// Count results by status
	successCount := 0
	skipCount := 0
	failCount := 0
	for _, result := range m.batchResults {
		if models.IsStatusSuccess(result.Status) {
			successCount++
		} else if models.IsStatusSkipped(result.Status) {
			skipCount++
		} else if models.IsStatusFailed(result.Status) {
			failCount++
		}
	}

	// Determine header message and colors based on results
	var headerMsg string
	var headerColor lipgloss.Color
	var icon string

	if successCount > 0 {
		headerMsg = fmt.Sprintf("%d PRs processed successfully!", successCount)
		headerColor = ui.ColorGreen
		icon = "✓"
	} else if skipCount > 0 && failCount == 0 {
		headerMsg = fmt.Sprintf("All %d repos skipped - branches already up to date", skipCount)
		headerColor = ui.ColorYellow
		icon = "⊘"
	} else if failCount > 0 {
		headerMsg = fmt.Sprintf("%d repos failed to process", failCount)
		headerColor = ui.ColorRed
		icon = "✗"
	} else {
		headerMsg = "No repositories processed"
		headerColor = ui.ColorYellow
		icon = "⊘"
	}

	// Typewriter effect for header message
	revealedChars := m.typewriterPos
	if revealedChars > len(headerMsg) {
		revealedChars = len(headerMsg)
	}
	revealedText := headerMsg[:revealedChars]

	// Pulsing icon (only pulse green for success)
	iconColor := headerColor
	if successCount > 0 {
		pulseIntensity := (math.Sin(m.pulsePhase) + 1.0) / 2.0
		if pulseIntensity > 0.5 {
			iconColor = ui.ColorGreen
		} else {
			iconColor = ui.ColorLightGreen
		}
	}

	iconStyle := lipgloss.NewStyle().Foreground(iconColor).Bold(true)
	headerStyle := lipgloss.NewStyle().Foreground(headerColor).Bold(true)

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("   %s %s", iconStyle.Render(icon), headerStyle.Render(revealedText)))
	lines = append(lines, "")

	// Results list
	for i, result := range m.batchResults {
		var statusStr string
		var statusColor lipgloss.Color

		if models.IsStatusCreated(result.Status) {
			statusStr = "✓ Created"
			statusColor = ui.ColorGreen
		} else if models.IsStatusUpdated(result.Status) {
			statusStr = "↻ Updated"
			statusColor = ui.ColorCyan
		} else if models.IsStatusSkipped(result.Status) {
			statusStr = "⊘ Skipped"
			statusColor = ui.ColorYellow
		} else if models.IsStatusFailed(result.Status) {
			statusStr = "✗ Failed"
			statusColor = ui.ColorRed
		}

		statusStyle := lipgloss.NewStyle().Foreground(statusColor)
		repoStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)

		// Highlight selected row
		prefix := "  "
		if i == m.menuIndex {
			prefix = "▶ "
			repoStyle = repoStyle.Bold(true)
		}

		lines = append(lines, fmt.Sprintf("   %s%s %s",
			prefix,
			statusStyle.Render(fmt.Sprintf("%-12s", statusStr)),
			repoStyle.Render(result.Repo.DisplayName),
		))

		// Show URL if available
		if result.PrURL != nil {
			urlStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
			lines = append(lines, fmt.Sprintf("              🔗 %s", urlStyle.Render(*result.PrURL)))
		}

		// Show skip/fail reason
		reason := models.GetStatusReason(result.Status)
		if reason != "" {
			reasonStyle := lipgloss.NewStyle().Foreground(statusColor)
			lines = append(lines, fmt.Sprintf("              %s", reasonStyle.Render(reason)))
		}

		// Show tickets if any
		if len(result.Tickets) > 0 {
			ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)
			lines = append(lines, fmt.Sprintf("              🎫 %s", ticketStyle.Render(strings.Join(result.Tickets, ", "))))
		}
	}

	lines = append(lines, "")

	// Summary footer
	dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
	lines = append(lines, dimStyle.Render(fmt.Sprintf("   Total: %d success, %d skipped, %d failed",
		successCount, skipCount, failCount)))

	// Render confetti if there were successes
	if successCount > 0 {
		lines = append(lines, "")
		lines = append(lines, m.renderConfetti())
	}

	content := strings.Join(lines, "\n")

	// Fixed box width for stable layout
	boxWidth := m.contentWidth() - 10

	return ui.ColumnBox(content, " Batch Summary ", ui.ColorGreen, true, boxWidth, availableHeight)
}

func (m Model) renderViewOpenPrsWithHeight(availableHeight int) string {
	if len(m.mergePRs) == 0 {
		successStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen)
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)

		successText := fmt.Sprintf("%s No open release PRs", successStyle.Render("✓"))
		subText := dimStyle.Render("All repositories are up to date!")

		// Center the text
		centeredStyle := lipgloss.NewStyle().Width(m.contentWidth()).Align(lipgloss.Center)

		var lines []string
		lines = append(lines, "")
		lines = append(lines, "")
		lines = append(lines, "")
		lines = append(lines, centeredStyle.Render(successText))
		lines = append(lines, centeredStyle.Render(subText))

		return strings.Join(lines, "\n")
	}

	// Fixed column dimensions for stable layout (same as batch select)
	columnWidth := (m.contentWidth() - 6) / 2

	// Column height calculation
	columnHeight := availableHeight - 8
	if columnHeight < 5 {
		columnHeight = 5
	}

	// Title bar width matches the two columns + gap
	titleWidth := columnWidth*2 + 2

	// Count selected
	selectedCount := 0
	for _, s := range m.mergeSelected {
		if s {
			selectedCount++
		}
	}

	// Title bar (similar to batch select filter box)
	title := fmt.Sprintf("Open Release PRs (%d selected)", selectedCount)
	titleBox := ui.FilterInput("", title, ui.ColorYellow, titleWidth)

	// Build Dev → Staging column
	var devLines []string
	devHighlightedLine := -1
	devLines = append(devLines, ui.SectionHeader("🟢 DEV → STAGING", ui.ColorGreen))
	devLines = append(devLines, "")

	devCount := 0
	for i, pr := range m.mergePRs {
		if pr.PrType == models.DevToStaging {
			name := pr.Repo.DisplayName
			if idx := strings.LastIndex(name, "/"); idx != -1 {
				name = name[idx+1:]
			}
			selected := false
			if i < len(m.mergeSelected) {
				selected = m.mergeSelected[i]
			}
			highlighted := m.mergeColumn == 0 && m.mergeDevIndex == devCount
			if highlighted {
				devHighlightedLine = len(devLines)
			}
			devLines = append(devLines, ui.PRListItem(name, pr.PrNumber, pr.PrType.HeadBranch(), pr.PrType.BaseBranch(pr.Repo.MainBranch), pr.URL, selected, highlighted, ui.ColorGreen))
			devCount++
		}
	}
	if devCount == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		devLines = append(devLines, dimStyle.Render("  No open PRs"))
	}

	// Build Staging → Main column
	var mainLines []string
	mainHighlightedLine := -1
	mainLines = append(mainLines, ui.SectionHeader("🔴 STAGING → MAIN", ui.ColorRed))
	mainLines = append(mainLines, "")

	mainCount := 0
	for i, pr := range m.mergePRs {
		if pr.PrType == models.StagingToMain {
			name := pr.Repo.DisplayName
			if idx := strings.LastIndex(name, "/"); idx != -1 {
				name = name[idx+1:]
			}
			selected := false
			if i < len(m.mergeSelected) {
				selected = m.mergeSelected[i]
			}
			highlighted := m.mergeColumn == 1 && m.mergeMainIndex == mainCount
			if highlighted {
				mainHighlightedLine = len(mainLines)
			}
			mainLines = append(mainLines, ui.PRListItem(name, pr.PrNumber, pr.PrType.HeadBranch(), pr.PrType.BaseBranch(pr.Repo.MainBranch), pr.URL, selected, highlighted, ui.ColorRed))
			mainCount++
		}
	}
	if mainCount == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		mainLines = append(mainLines, dimStyle.Render("  No open PRs"))
	}

	// Apply viewport scrolling to keep highlighted item visible
	headerLines := 2
	visibleContentLines := columnHeight - headerLines
	if visibleContentLines < 1 {
		visibleContentLines = 1
	}

	devContent := applyViewportScroll(devLines, headerLines, devHighlightedLine, visibleContentLines)
	mainContent := applyViewportScroll(mainLines, headerLines, mainHighlightedLine, visibleContentLines)

	// Use same height for both columns
	devColumn := ui.ColumnBox(devContent, "", ui.ColorGreen, m.mergeColumn == 0, columnWidth, columnHeight)
	mainColumn := ui.ColumnBox(mainContent, "", ui.ColorRed, m.mergeColumn == 1, columnWidth, columnHeight)

	return titleBox + "\n" + ui.TwoColumns(devColumn, mainColumn, 2)
}

func (m Model) renderMergeConfirmation() string {
	var lines []string

	lines = append(lines, ui.SectionHeader("Confirm Merge", ui.ColorMagenta))
	lines = append(lines, "")

	selected := 0
	for _, s := range m.mergeSelected {
		if s {
			selected++
		}
	}

	lines = append(lines, fmt.Sprintf("   PRs to merge: %d", selected))
	lines = append(lines, "")

	if m.dryRun {
		warningStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		lines = append(lines, warningStyle.Render("   ⚠ DRY RUN: No actual changes will be made"))
		lines = append(lines, "")
	}

	lines = append(lines, ui.YesNoButtons(m.confirmSelection))

	return strings.Join(lines, "\n")
}

func (m Model) renderMerging() string {
	var lines []string

	lines = append(lines, ui.SectionHeader("Merging PRs", ui.ColorMagenta))
	lines = append(lines, "")

	spinner := ui.Spinner(m.spinnerFrame)
	spinnerStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)
	statusStyle := lipgloss.NewStyle().Foreground(ui.ColorMagenta)

	lines = append(lines, fmt.Sprintf("   %s %s",
		spinnerStyle.Render(spinner),
		statusStyle.Render("Merging PRs..."),
	))

	return strings.Join(lines, "\n")
}

func (m Model) renderMergeSummaryWithHeight(availableHeight int) string {
	var lines []string

	// Count successes and failures
	successCount := 0
	failCount := 0
	for _, result := range m.mergeResults {
		if result.Success {
			successCount++
		} else {
			failCount++
		}
	}

	// Header color based on overall result
	headerColor := ui.ColorGreen
	if failCount > 0 {
		headerColor = ui.ColorYellow
	}

	lines = append(lines, ui.SectionHeader("Merge Results", headerColor))
	lines = append(lines, "")

	// Summary counts
	successStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen)
	failStyle := lipgloss.NewStyle().Foreground(ui.ColorRed)
	lines = append(lines, fmt.Sprintf("   %s %d succeeded  %s %d failed",
		successStyle.Render("✓"),
		successCount,
		failStyle.Render("✗"),
		failCount,
	))
	lines = append(lines, "")

	// Individual results
	for _, result := range m.mergeResults {
		var icon string
		var iconStyle lipgloss.Style
		if result.Success {
			icon = "✓"
			iconStyle = lipgloss.NewStyle().Foreground(ui.ColorGreen)
		} else {
			icon = "✗"
			iconStyle = lipgloss.NewStyle().Foreground(ui.ColorRed)
		}

		repoStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite).Bold(true)
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)

		lines = append(lines, fmt.Sprintf("   %s %s %s",
			iconStyle.Render(icon),
			repoStyle.Render(result.RepoName),
			dimStyle.Render(fmt.Sprintf("#%d", result.PrNumber)),
		))
	}

	content := strings.Join(lines, "\n")

	// Fixed box width for stable layout
	boxWidth := m.contentWidth() - 10

	return ui.ColumnBox(content, " Merge Summary ", headerColor, true, boxWidth, availableHeight)
}

func (m Model) renderUpdatePrompt() string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, ui.SectionHeader("Update Available!", ui.ColorCyan))
	lines = append(lines, "")

	if m.updateAvailable != nil {
		versionStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true)
		currentStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)

		lines = append(lines, fmt.Sprintf("   Current version: %s", currentStyle.Render(m.version)))
		lines = append(lines, fmt.Sprintf("   New version:     %s", versionStyle.Render(update.VersionDisplay(m.updateAvailable.TagName))))
		lines = append(lines, "")
	}

	lines = append(lines, "   What would you like to do?")
	lines = append(lines, "")

	// Option buttons - fixed width for alignment
	options := []struct {
		key   string
		label string
		color lipgloss.Color
	}{
		{"y", "Update now", ui.ColorGreen},
		{"n", "Skip for now", ui.ColorYellow},
		{"s", "Skip this version", ui.ColorRed},
	}

	var buttons []string
	for i, opt := range options {
		text := fmt.Sprintf("[%s] %s", opt.key, opt.label)
		var style lipgloss.Style
		if i == m.updateSelection {
			style = lipgloss.NewStyle().
				Background(opt.color).
				Foreground(lipgloss.Color("#000000")).
				Padding(0, 1).
				Bold(true)
		} else {
			style = lipgloss.NewStyle().
				Foreground(opt.color).
				Padding(0, 1)
		}
		buttons = append(buttons, style.Render(text))
	}

	lines = append(lines, "   "+strings.Join(buttons, "   "))
	lines = append(lines, "")

	return strings.Join(lines, "\n")
}

func (m Model) renderUpdating() string {
	var lines []string

	lines = append(lines, "")
	lines = append(lines, ui.SectionHeader("Updating...", ui.ColorCyan))
	lines = append(lines, "")

	spinner := ui.Spinner(m.spinnerFrame)
	spinnerStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
	statusStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)

	lines = append(lines, fmt.Sprintf("   %s %s",
		spinnerStyle.Render(spinner),
		statusStyle.Render("Downloading and installing update..."),
	))
	lines = append(lines, "")

	if m.updateAvailable != nil {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		lines = append(lines, dimStyle.Render(fmt.Sprintf("   Installing version %s", update.VersionDisplay(m.updateAvailable.TagName))))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderStatusBar() string {
	var hints []string

	switch m.screen {
	case ScreenMainMenu:
		hints = []string{
			ui.KeyBinding("1-4", "Select", ui.ColorYellow),
			ui.KeyBinding("↑↓", "Navigate", ui.ColorWhite),
			ui.KeyBinding("Enter", "Select", ui.ColorGreen),
			ui.KeyBinding("c", "Config", ui.ColorMagenta),
			ui.KeyBinding("u", "Update", ui.ColorCyan),
			ui.KeyBinding("q", "Quit", ui.ColorRed),
		}
	case ScreenPrTypeSelect:
		hints = []string{
			ui.KeyBinding("1-2", "Select", ui.ColorYellow),
			ui.KeyBinding("↑↓", "Navigate", ui.ColorWhite),
			ui.KeyBinding("Enter", "Select", ui.ColorGreen),
			ui.KeyBinding("Esc", "Back", ui.ColorYellow),
		}
	case ScreenCommitReview:
		if len(m.commits) > 0 {
			hints = []string{
				ui.KeyBinding("Type", "Edit title", ui.ColorYellow),
				ui.KeyBinding("Enter", "Create PR", ui.ColorGreen),
				ui.KeyBinding("Esc", "Back", ui.ColorYellow),
			}
		} else {
			hints = []string{
				ui.KeyBinding("Esc", "Back", ui.ColorYellow),
			}
		}
	case ScreenTitleInput:
		hints = []string{
			ui.KeyBinding("Enter", "Submit", ui.ColorGreen),
			ui.KeyBinding("Esc", "Back", ui.ColorYellow),
		}
	case ScreenConfirmation, ScreenBatchConfirmation, ScreenMergeConfirmation:
		hints = []string{
			ui.KeyBinding("←→", "Select", ui.ColorWhite),
			ui.KeyBinding("y/n", "Quick", ui.ColorGreen),
			ui.KeyBinding("Enter", "Confirm", ui.ColorGreen),
			ui.KeyBinding("Esc", "Back", ui.ColorYellow),
		}
	case ScreenComplete:
		hints = []string{
			ui.KeyBinding("o", "Open URL", ui.ColorBlue),
			ui.KeyBinding("c", "Copy URL", ui.ColorBlue),
			ui.KeyBinding("m", "Merge PRs", ui.ColorGreen),
			ui.KeyBinding("Enter", "Done", ui.ColorGreen),
		}
	case ScreenBatchRepoSelect:
		hints = []string{
			ui.KeyBinding("↑↓", "Navigate", ui.ColorWhite),
			ui.KeyBinding("←→", "Column", ui.ColorWhite),
			ui.KeyBinding("Space", "Toggle", ui.ColorGreen),
			ui.KeyBinding("Tab", "Continue", ui.ColorGreen),
			ui.KeyBinding("Type", "Filter", ui.ColorYellow),
		}
	case ScreenViewOpenPrs:
		if len(m.mergePRs) == 0 {
			hints = []string{
				ui.KeyBinding("r", "Refresh", ui.ColorBlue),
				ui.KeyBinding("Esc", "Back", ui.ColorYellow),
			}
		} else {
			hints = []string{
				ui.KeyBinding("↑↓", "Navigate", ui.ColorWhite),
				ui.KeyBinding("←→", "Column", ui.ColorWhite),
				ui.KeyBinding("Space", "Toggle", ui.ColorGreen),
				ui.KeyBinding("Tab", "Continue", ui.ColorGreen),
				ui.KeyBinding("r", "Refresh", ui.ColorBlue),
				ui.KeyBinding("Esc", "Back", ui.ColorYellow),
			}
		}
	case ScreenError:
		hints = []string{
			ui.KeyBinding("Enter", "Back", ui.ColorGreen),
			ui.KeyBinding("q", "Quit", ui.ColorRed),
		}
	case ScreenBatchSummary:
		hints = []string{
			ui.KeyBinding("o", "Open URLs", ui.ColorBlue),
			ui.KeyBinding("c", "Copy URLs", ui.ColorBlue),
			ui.KeyBinding("m", "Merge PRs", ui.ColorGreen),
			ui.KeyBinding("Enter", "Done", ui.ColorGreen),
			ui.KeyBinding("q", "Quit", ui.ColorRed),
		}
	case ScreenMergeSummary:
		hints = []string{
			ui.KeyBinding("o", "Open URLs", ui.ColorBlue),
			ui.KeyBinding("c", "Copy URLs", ui.ColorBlue),
			ui.KeyBinding("Enter", "Done", ui.ColorGreen),
			ui.KeyBinding("q", "Quit", ui.ColorRed),
		}
	case ScreenUpdatePrompt:
		hints = []string{
			ui.KeyBinding("←→", "Select", ui.ColorWhite),
			ui.KeyBinding("y", "Update", ui.ColorGreen),
			ui.KeyBinding("n", "Skip", ui.ColorYellow),
			ui.KeyBinding("s", "Skip version", ui.ColorRed),
			ui.KeyBinding("Enter", "Confirm", ui.ColorGreen),
		}
	case ScreenUpdating:
		hints = []string{}
	default:
		hints = []string{}
	}

	installedVersion := ""
	if m.version != "" {
		installedVersion = update.VersionDisplay(m.version)
	}

	// Don't render an empty box if there are no hints or version
	if len(hints) == 0 && m.copyFeedback == "" && installedVersion == "" {
		return ""
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorDarkGray).
		Padding(0, 1)

	var contentLines []string

	hotkeysLine := strings.Join(hints, "  ")

	// Add copy feedback if present
	if m.copyFeedback != "" {
		feedbackStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true)
		if strings.HasPrefix(m.copyFeedback, "✗") {
			feedbackStyle = lipgloss.NewStyle().Foreground(ui.ColorRed).Bold(true)
		}
		if hotkeysLine != "" {
			hotkeysLine += "  │  "
		}
		hotkeysLine += feedbackStyle.Render(m.copyFeedback)
	}

	if hotkeysLine != "" {
		contentLines = append(contentLines, hotkeysLine)
	}

	if installedVersion != "" {
		versionStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		versionLine := fmt.Sprintf("Version: %s", installedVersion)
		if m.updateCheckInProgress {
			spinnerStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
			versionLine = fmt.Sprintf("%s  •  Checking updates %s", versionLine, spinnerStyle.Render(ui.Spinner(m.spinnerFrame)))
		}

		targetWidth := lipgloss.Width(hotkeysLine)
		if w := lipgloss.Width(versionLine); w > targetWidth {
			targetWidth = w
		}
		if targetWidth > 0 {
			versionLine = lipgloss.PlaceHorizontal(targetWidth, lipgloss.Center, versionLine)
		}
		contentLines = append(contentLines, versionStyle.Render(versionLine))
	}

	return borderStyle.Render(strings.Join(contentLines, "\n"))
}

// ptrEqual compares two string pointers for equality
func ptrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
