package app

import (
	"fmt"
	"math"
	"strings"

	"attuned-release/internal/models"
	"attuned-release/internal/ui"

	"github.com/charmbracelet/lipgloss"
)

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

	// Screens that manage their own full layout (no outer box)
	fullLayoutScreens := m.screen == ScreenBatchRepoSelect ||
		m.screen == ScreenViewOpenPrs ||
		m.screen == ScreenBatchSummary ||
		m.screen == ScreenMergeSummary ||
		m.screen == ScreenCommitReview

	if fullLayoutScreens {
		sections = append(sections, m.renderContentWithHeight(availableHeight))
	} else {
		// Standard outer box for simpler screens
		contentWidth := m.width - 4
		if contentWidth < 80 {
			contentWidth = 80
		}
		if contentWidth > 120 {
			contentWidth = 120
		}

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
		return m.renderBatchConfirmation()
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
		{"📦", "SINGLE REPO", "Create PR for current repo", ui.ColorCyan},
		{"🚀", "BATCH MODE", "Create PRs for multiple repos", ui.ColorMagenta},
		{"👀", "VIEW OPEN PRS", "See all open release PRs", ui.ColorYellow},
		{"❌", "QUIT", "Exit application", ui.ColorRed},
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
		icon      string
		head      string
		base      string
		desc      string
		headColor lipgloss.Color
		baseColor lipgloss.Color
	}{
		{"🔄", "dev", "staging", "Merge to staging for QA", ui.ColorGreen, ui.ColorYellow},
		{"🚀", "staging", mainBranch, "Release to production", ui.ColorYellow, ui.ColorRed},
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
			iconStyle := lipgloss.NewStyle().Background(ui.ColorDarkGray)
			headStyle := lipgloss.NewStyle().Foreground(t.headColor).Bold(true).Background(ui.ColorDarkGray)
			baseStyle := lipgloss.NewStyle().Foreground(t.baseColor).Bold(true).Background(ui.ColorDarkGray)
			descStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite).Background(ui.ColorDarkGray)

			line1 = rowStyle.Render(arrowStyle.Render(arrow) + iconStyle.Render(t.icon+"  ") + headStyle.Render(t.head) + iconStyle.Render(" → ") + baseStyle.Render(t.base))
			line2 = rowStyle.Render("       " + descStyle.Render(t.desc))
		} else {
			arrowStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
			headStyle := lipgloss.NewStyle().Foreground(t.headColor).Bold(true)
			baseStyle := lipgloss.NewStyle().Foreground(t.baseColor).Bold(true)
			descStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)

			line1 = arrowStyle.Render(arrow) + t.icon + "  " + headStyle.Render(t.head) + " → " + baseStyle.Render(t.base)
			line2 = "       " + descStyle.Render(t.desc)
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
		titleStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true)
		infoLines = append(infoLines, titleStyle.Render("  Development → Staging"))
		infoLines = append(infoLines, "")
		infoLines = append(infoLines, "  Merge feature branches into")
		infoLines = append(infoLines, "  staging for QA testing.")
		infoLines = append(infoLines, "")
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		baseStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		headStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true)
		infoLines = append(infoLines, labelStyle.Render("  Base: ")+baseStyle.Render("staging"))
		infoLines = append(infoLines, labelStyle.Render("  Head: ")+headStyle.Render("dev"))
	} else {
		titleStyle := lipgloss.NewStyle().Foreground(ui.ColorRed).Bold(true)
		infoLines = append(infoLines, titleStyle.Render("  Staging → Production"))
		infoLines = append(infoLines, "")
		infoLines = append(infoLines, "  Release staging changes to")
		infoLines = append(infoLines, "  production environment.")
		infoLines = append(infoLines, "")
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		baseStyle := lipgloss.NewStyle().Foreground(ui.ColorRed).Bold(true)
		headStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		infoLines = append(infoLines, labelStyle.Render("  Base: ")+baseStyle.Render(mainBranch))
		infoLines = append(infoLines, labelStyle.Render("  Head: ")+headStyle.Render("staging"))
	}

	infoTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite)
	infoContent := infoTitleStyle.Render(" PR Details ") + "\n" + strings.Join(infoLines, "\n")

	return ui.UnifiedPanel(menuContent, infoContent, 48, 48, ui.ColorCyan)
}

func (m Model) renderLoading() string {
	spinner := ui.Spinner(m.spinnerFrame)
	spinnerStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
	textStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)

	return fmt.Sprintf("\n   %s %s\n",
		spinnerStyle.Render(spinner),
		textStyle.Render(m.loadingMessage),
	)
}

func (m Model) renderCommitReviewWithHeight(availableHeight int) string {
	// Dynamic column sizing
	columnWidth := (m.width - 6) / 2
	if columnWidth < 40 {
		columnWidth = 40
	}
	if columnWidth > 60 {
		columnWidth = 60
	}
	panelHeight := availableHeight - 2
	if panelHeight < 10 {
		panelHeight = 10
	}

	// Build left column (commits list)
	var commitLines []string
	commitLines = append(commitLines, "")

	if len(m.commits) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		commitLines = append(commitLines, dimStyle.Render("  No commits to merge"))
	} else {
		for i, commit := range m.commits {
			arrowStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
			arrow := "  "
			if i == m.menuIndex {
				arrow = "▶ "
			}

			hashStyle := lipgloss.NewStyle().Foreground(ui.ColorMagenta)
			msgStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)

			ticketStr := ""
			if len(commit.Tickets) > 0 {
				ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
				ticketStr = " " + ticketStyle.Render("["+strings.Join(commit.Tickets, ", ")+"]")
			}

			commitLines = append(commitLines, fmt.Sprintf("  %s%s %s%s",
				arrowStyle.Render(arrow),
				hashStyle.Render(commit.Hash),
				msgStyle.Render(commit.Message),
				ticketStr,
			))
		}
	}

	commitTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorCyan)
	commitContent := commitTitleStyle.Render(fmt.Sprintf(" %d commits ", len(m.commits))) + "\n" + strings.Join(commitLines, "\n")

	// Build right column (PR info + tickets)
	var rightLines []string

	infoTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorWhite)
	rightLines = append(rightLines, infoTitleStyle.Render(" PR Info "))
	rightLines = append(rightLines, "")

	if m.repoInfo != nil {
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		valueStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Bold(true)
		rightLines = append(rightLines, labelStyle.Render("  Repo: ")+valueStyle.Render(m.repoInfo.DisplayName))
	}

	if m.prType != nil {
		mainBranch := "main"
		if m.repoInfo != nil {
			mainBranch = m.repoInfo.MainBranch
		}
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		typeStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		rightLines = append(rightLines, labelStyle.Render("  Type: ")+typeStyle.Render(m.prType.Display(mainBranch)))
	}

	rightLines = append(rightLines, "")

	// Tickets section
	ticketTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorYellow)
	rightLines = append(rightLines, ticketTitleStyle.Render(fmt.Sprintf(" Tickets (%d) ", len(m.tickets))))
	rightLines = append(rightLines, "")

	if len(m.tickets) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		rightLines = append(rightLines, dimStyle.Render("  No tickets found"))
	} else {
		for _, ticket := range m.tickets {
			ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
			rightLines = append(rightLines, fmt.Sprintf("  🎫 %s", ticketStyle.Render(ticket)))
		}
	}

	rightLines = append(rightLines, "")
	continueStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
	enterStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true)
	rightLines = append(rightLines, continueStyle.Render("  Press ")+enterStyle.Render("Enter")+" to continue")

	rightContent := strings.Join(rightLines, "\n")

	// Use ColumnBox for consistent sizing
	leftColumn := ui.ColumnBox(commitContent, "", ui.ColorCyan, true, columnWidth, panelHeight)
	rightColumn := ui.ColumnBox(rightContent, "", ui.ColorWhite, false, columnWidth-10, panelHeight)

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

	if m.repoInfo != nil {
		labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
		valueStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
		rightLines = append(rightLines, labelStyle.Render("  Repo: ")+valueStyle.Render(m.repoInfo.DisplayName))
	}

	labelStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)
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
		leftLines = append(leftLines, "  ## Tickets")
		for _, ticket := range m.tickets {
			ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow)
			urlStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
			leftLines = append(leftLines, fmt.Sprintf("  - %s%s", ticketStyle.Render(fmt.Sprintf("[%s]", ticket)), urlStyle.Render("(linear.app/...)")))
		}
	}

	leftLines = append(leftLines, "")

	// Confirm section
	leftLines = append(leftLines, ui.SectionHeader("CONFIRM", ui.ColorGreen))
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, "  Create this PR?")
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, ui.YesNoButtons(m.confirmSelection))

	leftTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorCyan)
	leftContent := leftTitleStyle.Render(" 🚀 Create PR ") + "\n" + strings.Join(leftLines, "\n")

	// Build right column (stats)
	var rightLines []string
	rightLines = append(rightLines, "")
	rightLines = append(rightLines, ui.SectionHeader("STATS", ui.ColorMagenta))
	rightLines = append(rightLines, "")

	commitStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Bold(true)
	ticketStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
	rightLines = append(rightLines, fmt.Sprintf("  📊 %s commits", commitStyle.Render(fmt.Sprintf("%d", len(m.commits)))))
	rightLines = append(rightLines, fmt.Sprintf("  🎫 %s tickets", ticketStyle.Render(fmt.Sprintf("%d", len(m.tickets)))))

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

	// Column width: half of available width, with bounds
	columnWidth := (m.width - 6) / 2
	if columnWidth < 35 {
		columnWidth = 35
	}
	if columnWidth > 50 {
		columnWidth = 50
	}

	// Column height: available height minus filter box (4 lines) and gap (2)
	columnHeight := availableHeight - 6
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

	// Build Frontend column
	var feLines []string
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

			// Indent nested repos
			indent := ""
			if repo.ParentRepo != nil {
				indent = "│ "
			}
			feLines = append(feLines, ui.RepoListItem(name, selected, highlighted, ui.ColorCyan, indent))
		}
	}

	// Build Backend column
	var beLines []string
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

			// Indent nested repos
			indent := ""
			if repo.ParentRepo != nil {
				indent = "│ "
			}
			beLines = append(beLines, ui.RepoListItem(name, selected, highlighted, ui.ColorMagenta, indent))
		}
	}

	// Create columns with fixed dimensions
	feContent := strings.Join(feLines, "\n")
	beContent := strings.Join(beLines, "\n")

	feColumn := ui.ColumnBox(feContent, "", ui.ColorCyan, m.batchColumn == 0, columnWidth, columnHeight)
	beColumn := ui.ColumnBox(beContent, "", ui.ColorMagenta, m.batchColumn == 1, columnWidth, columnHeight)

	columns := ui.TwoColumns(feColumn, beColumn, 2)

	return filterBox + "\n\n" + columns
}

func (m Model) renderBatchConfirmation() string {
	selectedCount := 0
	for _, s := range m.batchSelected {
		if s {
			selectedCount++
		}
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

	// List selected repos (max 8)
	for i, name := range selectedRepos {
		if i >= 8 {
			remaining := len(selectedRepos) - 8
			leftLines = append(leftLines, fmt.Sprintf("    ... and %d more", remaining))
			break
		}
		leftLines = append(leftLines, fmt.Sprintf("  %s", name))
	}
	leftLines = append(leftLines, "")

	// Confirm section
	leftLines = append(leftLines, ui.SectionHeader("CONFIRM", ui.ColorGreen))
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, fmt.Sprintf("  Create %d PRs?", selectedCount))
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, ui.YesNoButtons(m.confirmSelection))

	leftTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorCyan)
	leftContent := leftTitleStyle.Render(" 🚀 Batch PRs ") + "\n" + strings.Join(leftLines, "\n")

	// Build right column (stats)
	var rightLines []string
	rightLines = append(rightLines, "")
	rightLines = append(rightLines, ui.SectionHeader("STATS", ui.ColorMagenta))
	rightLines = append(rightLines, "")

	repoStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Bold(true)
	prStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
	rightLines = append(rightLines, fmt.Sprintf("  📊 %s repositories", repoStyle.Render(fmt.Sprintf("%d", selectedCount))))
	rightLines = append(rightLines, fmt.Sprintf("  🔄 %s PRs to create", prStyle.Render(fmt.Sprintf("%d", selectedCount))))

	if m.dryRun {
		rightLines = append(rightLines, "")
		warningStyle := lipgloss.NewStyle().Foreground(ui.ColorYellow).Bold(true)
		rightLines = append(rightLines, warningStyle.Render("  ⚠ DRY RUN MODE"))
	}

	rightTitleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorMagenta)
	rightContent := rightTitleStyle.Render(" 📊 Summary ") + "\n" + strings.Join(rightLines, "\n")

	return ui.UnifiedPanel(leftContent, rightContent, 60, 35, ui.ColorCyan)
}

func (m Model) renderBatchProcessing() string {
	var lines []string

	lines = append(lines, ui.SectionHeader("Processing Repositories", ui.ColorMagenta))
	lines = append(lines, "")

	spinner := ui.Spinner(m.spinnerFrame)
	spinnerStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
	statusStyle := lipgloss.NewStyle().Foreground(ui.ColorWhite)

	lines = append(lines, fmt.Sprintf("   %s %s",
		spinnerStyle.Render(spinner),
		statusStyle.Render("Processing repos..."),
	))
	lines = append(lines, "")

	progress := ui.ProgressBar(m.batchCurrent, len(m.batchRepos), 30)
	lines = append(lines, fmt.Sprintf("   %s", progress))
	lines = append(lines, "")

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

	// Wrap in a box with dynamic sizing
	boxWidth := m.width - 10
	if boxWidth < 60 {
		boxWidth = 60
	}
	if boxWidth > 100 {
		boxWidth = 100
	}

	return ui.ColumnBox(content, " Batch Summary ", ui.ColorGreen, true, boxWidth, availableHeight)
}

func (m Model) renderViewOpenPrsWithHeight(availableHeight int) string {
	if m.openPRsLoading {
		spinner := ui.Spinner(m.spinnerFrame)
		spinnerStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
		textStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		return fmt.Sprintf("\n   %s %s\n\n   %s",
			spinnerStyle.Render(spinner),
			textStyle.Render("Fetching open PRs..."),
			dimStyle.Render("Checking all repositories in parallel"))
	}

	if len(m.mergePRs) == 0 {
		successStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen)
		dimStyle := lipgloss.NewStyle().Foreground(ui.ColorDarkGray)
		return fmt.Sprintf("\n   %s No open release PRs\n\n   %s",
			successStyle.Render("✓"),
			dimStyle.Render("All repositories are up to date!"))
	}

	// Column dimensions - use most of terminal width
	columnWidth := (m.width - 8) / 2 // Split width evenly with gap
	if columnWidth < 40 {
		columnWidth = 40
	}
	// No max cap - let columns expand to fill space

	// Column height for equal sizing
	columnHeight := availableHeight - 2
	if columnHeight < 8 {
		columnHeight = 8
	}

	// Build Dev → Staging column
	var devLines []string
	devCount := 0
	for i, pr := range m.mergePRs {
		if pr.PrType == models.DevToStaging {
			devCount++
			name := pr.Repo.DisplayName
			if idx := strings.LastIndex(name, "/"); idx != -1 {
				name = name[idx+1:]
			}
			selected := false
			if i < len(m.mergeSelected) {
				selected = m.mergeSelected[i]
			}
			highlighted := m.mergeColumn == 0 && m.mergeDevIndex == devCount-1
			devLines = append(devLines, ui.PRListItem(name, pr.PrNumber, pr.PrType.HeadBranch(), pr.PrType.BaseBranch(pr.Repo.MainBranch), pr.URL, selected, highlighted, ui.ColorGreen))
		}
	}
	if devCount == 0 {
		devLines = append(devLines, "    No open PRs")
	}

	// Build Staging → Main column
	var mainLines []string
	mainCount := 0
	for i, pr := range m.mergePRs {
		if pr.PrType == models.StagingToMain {
			mainCount++
			name := pr.Repo.DisplayName
			if idx := strings.LastIndex(name, "/"); idx != -1 {
				name = name[idx+1:]
			}
			selected := false
			if i < len(m.mergeSelected) {
				selected = m.mergeSelected[i]
			}
			highlighted := m.mergeColumn == 1 && m.mergeMainIndex == mainCount-1
			mainLines = append(mainLines, ui.PRListItem(name, pr.PrNumber, pr.PrType.HeadBranch(), pr.PrType.BaseBranch(pr.Repo.MainBranch), pr.URL, selected, highlighted, ui.ColorRed))
		}
	}
	if mainCount == 0 {
		mainLines = append(mainLines, "    No open PRs")
	}

	// Create header and columns
	devHeader := ui.SectionHeader(fmt.Sprintf("DEV → STAGING (%d)", devCount), ui.ColorGreen)
	mainHeader := ui.SectionHeader(fmt.Sprintf("STAGING → MAIN (%d)", mainCount), ui.ColorRed)

	devContent := devHeader + "\n\n" + strings.Join(devLines, "\n")
	mainContent := mainHeader + "\n\n" + strings.Join(mainLines, "\n")

	// Use same height for both columns so they align at bottom
	devColumn := ui.ColumnBox(devContent, "", ui.ColorGreen, m.mergeColumn == 0, columnWidth, columnHeight)
	mainColumn := ui.ColumnBox(mainContent, "", ui.ColorRed, m.mergeColumn == 1, columnWidth, columnHeight)

	return "\n" + ui.TwoColumns(devColumn, mainColumn, 2)
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

	// Wrap in a box with dynamic sizing
	boxWidth := m.width - 10
	if boxWidth < 50 {
		boxWidth = 50
	}
	if boxWidth > 80 {
		boxWidth = 80
	}

	return ui.ColumnBox(content, " Merge Summary ", headerColor, true, boxWidth, availableHeight)
}

func (m Model) renderStatusBar() string {
	var hints []string

	switch m.screen {
	case ScreenMainMenu:
		hints = []string{
			ui.KeyBinding("↑↓", "Navigate", ui.ColorWhite),
			ui.KeyBinding("Enter", "Select", ui.ColorGreen),
			ui.KeyBinding("q", "Quit", ui.ColorRed),
		}
	case ScreenPrTypeSelect:
		hints = []string{
			ui.KeyBinding("↑↓", "Navigate", ui.ColorWhite),
			ui.KeyBinding("Enter", "Select", ui.ColorGreen),
			ui.KeyBinding("Esc", "Back", ui.ColorYellow),
		}
	case ScreenCommitReview:
		hints = []string{
			ui.KeyBinding("↑↓", "Scroll", ui.ColorWhite),
			ui.KeyBinding("Enter", "Continue", ui.ColorGreen),
			ui.KeyBinding("Esc", "Back", ui.ColorYellow),
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
		hints = []string{
			ui.KeyBinding("↑↓", "Navigate", ui.ColorWhite),
			ui.KeyBinding("←→", "Column", ui.ColorWhite),
			ui.KeyBinding("Space", "Toggle", ui.ColorGreen),
			ui.KeyBinding("m", "Merge", ui.ColorMagenta),
			ui.KeyBinding("r", "Refresh", ui.ColorBlue),
			ui.KeyBinding("Esc", "Back", ui.ColorYellow),
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
	default:
		hints = []string{}
	}

	// Don't render an empty box if there are no hints
	if len(hints) == 0 {
		return ""
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorDarkGray).
		Padding(0, 1)

	return borderStyle.Render(strings.Join(hints, "  "))
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
