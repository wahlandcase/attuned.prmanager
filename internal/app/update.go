package app

import (
	"fmt"
	"strings"

	"github.com/wahlandcase/attuned.prmanager/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

// cancelBatchFetch cancels background fetches (channel is closed by sender goroutine)
func (m *Model) cancelBatchFetch() {
	if m.batchFetchCancel != nil {
		m.batchFetchCancel()
		m.batchFetchCancel = nil
	}
	m.batchResultsChan = nil
}

// Update handles all messages and updates state
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % 10
		m.updateAnimations()
		return m, tickCmd()

	// Task result messages
	case fetchCommitsResult:
		return m.handleFetchCommitsResult(msg)

	case batchCommitsResult:
		return m.handleBatchCommitsResult(msg)

	case prCreatedResult:
		return m.handlePrCreatedResult(msg)

	case batchRepoResult:
		return m.handleBatchRepoResult(msg)

	case batchProgressMsg:
		m.batchCurrentStep = msg.step
		// Continue listening for more progress updates
		return m, listenForProgress(m.batchProgressChan)

	case openPRsFetchedResult:
		return m.handleOpenPRsFetchedResult(msg)

	case mergeCompleteResult:
		return m.handleMergeCompleteResult(msg)

	case batchReposLoadedResult:
		return m.handleBatchReposLoaded(msg)

	case batchRepoCommitResult:
		return m.handleBatchRepoCommitResult(msg)

	case currentRepoLoadedResult:
		return m.handleCurrentRepoLoaded(msg)
	}

	return m, nil
}

// handleKey processes keyboard input
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear copy feedback on any keypress
	m.copyFeedback = ""

	// Global quit
	if msg.Type == tea.KeyCtrlC {
		m.shouldQuit = true
		return m, tea.Quit
	}

	switch m.screen {
	case ScreenMainMenu:
		return m.handleMainMenuKey(msg)
	case ScreenPrTypeSelect:
		return m.handlePrTypeSelectKey(msg)
	case ScreenCommitReview:
		return m.handleCommitReviewKey(msg)
	case ScreenTitleInput:
		return m.handleTitleInputKey(msg)
	case ScreenConfirmation, ScreenBatchConfirmation, ScreenMergeConfirmation:
		return m.handleConfirmationKey(msg)
	case ScreenComplete:
		return m.handleCompleteKey(msg)
	case ScreenError:
		return m.handleErrorKey(msg)
	case ScreenBatchRepoSelect:
		return m.handleBatchRepoSelectKey(msg)
	case ScreenBatchSummary:
		return m.handleBatchSummaryKey(msg)
	case ScreenViewOpenPrs:
		return m.handleViewOpenPrsKey(msg)
	case ScreenMergeSummary:
		return m.handleMergeSummaryKey(msg)
	}

	return m, nil
}

func (m Model) handleMainMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.shouldQuit = true
		return m, tea.Quit
	case "up", "k":
		if m.menuIndex > 0 {
			m.menuIndex--
		} else {
			m.menuIndex = 3 // Wrap to bottom
		}
	case "down", "j":
		if m.menuIndex < 3 {
			m.menuIndex++
		} else {
			m.menuIndex = 0 // Wrap to top
		}
	case "enter":
		return m.selectMainMenuItem()
	case "1":
		m.menuIndex = 0
		return m.selectMainMenuItem()
	case "2":
		m.menuIndex = 1
		return m.selectMainMenuItem()
	case "3":
		m.menuIndex = 2
		return m.selectMainMenuItem()
	case "4":
		m.menuIndex = 3
		return m.selectMainMenuItem()
	}
	return m, nil
}

func (m Model) selectMainMenuItem() (tea.Model, tea.Cmd) {
	switch m.menuIndex {
	case 0: // Single Repo
		mode := ModeSingle
		m.mode = &mode
		m.screen = ScreenLoading
		m.loadingMessage = "Detecting repository..."
		return m, loadCurrentRepoCmd()
	case 1: // Batch Mode
		mode := ModeBatch
		m.mode = &mode
		m.screen = ScreenPrTypeSelect
		m.menuIndex = 0
	case 2: // View Open PRs
		mode := ModeBatch
		m.mode = &mode
		m.screen = ScreenLoading
		m.loadingMessage = "Fetching open PRs..."
		return m, fetchOpenPRsCmd(m.config, m.dryRun)
	case 3: // Quit
		m.shouldQuit = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handlePrTypeSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.shouldQuit = true
		return m, tea.Quit
	case "up", "k":
		if m.menuIndex > 0 {
			m.menuIndex--
		} else {
			m.menuIndex = 1
		}
	case "down", "j":
		if m.menuIndex < 1 {
			m.menuIndex++
		} else {
			m.menuIndex = 0
		}
	case "enter":
		return m.selectPrType()
	case "1":
		m.menuIndex = 0
		return m.selectPrType()
	case "2":
		m.menuIndex = 1
		return m.selectPrType()
	case "esc":
		m.screen = ScreenMainMenu
		m.mode = nil
		m.menuIndex = 0
	}
	return m, nil
}

func (m Model) selectPrType() (tea.Model, tea.Cmd) {
	prTypes := []models.PrType{models.DevToStaging, models.StagingToMain}
	prType := prTypes[m.menuIndex]
	m.prType = &prType

	if m.mode != nil && *m.mode == ModeBatch {
		// Batch mode - load repos, then fetch commits in background
		m.screen = ScreenLoading
		m.loadingMessage = "Scanning repositories..."
		// Create channel for background fetch results
		m.batchResultsChan = make(chan batchRepoCommitResult, 50)
		return m, loadBatchReposCmd(m.config, m.prType, m.dryRun, m.batchResultsChan)
	} else {
		// Single mode - start fetching commits
		m.screen = ScreenLoading
		m.loadingMessage = "Fetching branches and commits..."
		return m, fetchCommitsCmd(m.repoInfo, m.prType, m.config.TicketRegex(), m.dryRun)
	}
}

func (m Model) handleCommitReviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Don't allow continuing if there are no commits
		if len(m.commits) == 0 {
			return m, nil
		}
		// Use default title if none entered
		if m.prTitle == "" && m.prType != nil {
			mainBranch := "main"
			if m.repoInfo != nil {
				mainBranch = m.repoInfo.MainBranch
			}
			m.prTitle = m.prType.DefaultTitle(mainBranch)
		}
		// Go directly to confirmation (skip title input screen)
		if m.mode != nil && *m.mode == ModeBatch {
			m.screen = ScreenBatchConfirmation
			m.batchConfirmScroll = 0
		} else {
			m.screen = ScreenConfirmation
		}
		m.confirmSelection = 0
	case tea.KeyEsc:
		m.screen = ScreenPrTypeSelect
		m.prType = nil
		m.prTitle = ""
		m.commits = nil
		m.tickets = nil
		m.menuIndex = 0
	case tea.KeyBackspace:
		if len(m.prTitle) > 0 {
			m.prTitle = m.prTitle[:len(m.prTitle)-1]
		}
	case tea.KeySpace:
		m.prTitle += " "
	case tea.KeyRunes:
		key := string(msg.Runes)
		if key == "q" && m.prTitle == "" {
			// Only quit if no title entered (so 'q' can be typed in title)
			m.shouldQuit = true
			return m, tea.Quit
		}
		m.prTitle += key
	}
	return m, nil
}

func (m Model) handleTitleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if m.prTitle == "" && m.prType != nil {
			mainBranch := "main"
			if m.repoInfo != nil {
				mainBranch = m.repoInfo.MainBranch
			}
			m.prTitle = m.prType.DefaultTitle(mainBranch)
		}
		if m.mode != nil && *m.mode == ModeBatch {
			m.screen = ScreenBatchConfirmation
			m.batchConfirmScroll = 0
		} else {
			m.screen = ScreenConfirmation
		}
		m.confirmSelection = 0
	case tea.KeyEsc:
		if m.mode != nil && *m.mode == ModeBatch {
			m.screen = ScreenBatchRepoSelect
		} else {
			m.screen = ScreenCommitReview
		}
		m.menuIndex = 0
	case tea.KeyBackspace:
		if len(m.prTitle) > 0 {
			m.prTitle = m.prTitle[:len(m.prTitle)-1]
		}
	case tea.KeySpace:
		m.prTitle += " "
	case tea.KeyRunes:
		m.prTitle += string(msg.Runes)
	}
	return m, nil
}

func (m Model) handleConfirmationKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.shouldQuit = true
		return m, tea.Quit
	case "left", "right", "tab":
		m.confirmSelection = 1 - m.confirmSelection
	case "up":
		// Scroll up in batch confirmation right column
		if m.screen == ScreenBatchConfirmation {
			// First clamp to max in case we're somehow beyond it
			totalLines := m.batchConfirmContentLines()
			visibleHeight := m.height - 10
			if visibleHeight < 10 {
				visibleHeight = 10
			}
			maxScroll := totalLines - visibleHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.batchConfirmScroll > maxScroll {
				m.batchConfirmScroll = maxScroll
			}
			// Then scroll up if possible
			if m.batchConfirmScroll > 0 {
				m.batchConfirmScroll--
			}
		}
	case "down":
		// Scroll down in batch confirmation right column
		if m.screen == ScreenBatchConfirmation {
			// Calculate max scroll based on content and visible height
			totalLines := m.batchConfirmContentLines()
			// Estimate visible height (will be clamped in view anyway)
			visibleHeight := m.height - 10
			if visibleHeight < 10 {
				visibleHeight = 10
			}
			maxScroll := totalLines - visibleHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.batchConfirmScroll < maxScroll {
				m.batchConfirmScroll++
			}
		}
	case "y":
		m.confirmSelection = 0
		return m.confirmAction()
	case "n":
		return m.goBack()
	case "enter":
		if m.confirmSelection == 0 {
			return m.confirmAction()
		}
		return m.goBack()
	case "esc":
		return m.goBack()
	}
	return m, nil
}

func (m Model) confirmAction() (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenConfirmation:
		m.screen = ScreenCreating
		return m, createPRCmd(m.repoInfo, m.prType, m.prTitle, m.tickets, m.config.Tickets.LinearOrg, m.dryRun)
	case ScreenBatchConfirmation:
		// Block if no repos have commits
		if m.batchReposWithCommits == 0 {
			return m, nil
		}
		// Count selected repos
		m.batchTotal = 0
		for _, selected := range m.batchSelected {
			if selected {
				m.batchTotal++
			}
		}
		m.batchCurrent = 0
		m.batchResults = nil
		m.batchCurrentRepo = m.batchRepos[0].DisplayName
		m.batchCurrentStep = ""
		// Create progress channel for real-time updates
		m.batchProgressChan = make(chan string, 1)
		m.screen = ScreenBatchProcessing
		// Start both the processing command and the progress listener
		return m, tea.Batch(
			startBatchProcessingCmd(&m, 0),
			listenForProgress(m.batchProgressChan),
		)
	case ScreenMergeConfirmation:
		// Count selected PRs
		m.mergeTotal = 0
		for _, selected := range m.mergeSelected {
			if selected {
				m.mergeTotal++
			}
		}
		m.mergeCurrent = 0
		m.mergeResults = nil
		m.screen = ScreenMerging
		// Find first selected PR
		for i, selected := range m.mergeSelected {
			if selected {
				return m, startMergingCmd(&m, i)
			}
		}
	}
	return m, nil
}

func (m Model) goBack() (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenConfirmation:
		m.screen = ScreenCommitReview
	case ScreenBatchConfirmation:
		m.screen = ScreenTitleInput // batch mode still uses separate title input
	case ScreenMergeConfirmation:
		m.screen = ScreenViewOpenPrs
	}
	m.confirmSelection = 0
	return m, nil
}

func (m Model) handleCompleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.shouldQuit = true
		return m, tea.Quit
	case "o":
		if m.prURL != "" {
			_ = openURL(m.prURL)
		}
	case "c":
		if m.prURL != "" {
			// Format as markdown list item
			repoName := "PR"
			if m.repoInfo != nil {
				repoName = m.repoInfo.DisplayName
			}
			formatted := fmt.Sprintf("- %s: %s", repoName, m.prURL)
			if err := copyToClipboard(formatted); err == nil {
				m.copyFeedback = "✓ Copied URL!"
			} else {
				m.copyFeedback = "✗ Copy failed"
			}
		}
		return m, nil
	case "enter", "esc":
		return m.reset()
	}
	return m, nil
}

func (m Model) handleErrorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", "q":
		m.errorMessage = ""
		if m.prType != nil {
			m.screen = ScreenPrTypeSelect
		} else {
			m.screen = ScreenMainMenu
			m.mode = nil
		}
		m.menuIndex = 0
	}
	return m, nil
}

func (m Model) handleBatchRepoSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		m.navigateBatchColumn(true)
	case tea.KeyDown:
		m.navigateBatchColumn(false)
	case tea.KeyLeft:
		if m.batchColumn != 0 {
			filtered := m.getFilteredBatchRepos(0)
			if len(filtered) > 0 {
				m.batchColumn = 0
				// Clamp index to valid range
				if m.batchFEIndex >= len(filtered) {
					m.batchFEIndex = len(filtered) - 1
				}
			}
		}
	case tea.KeyRight:
		if m.batchColumn != 1 {
			filtered := m.getFilteredBatchRepos(1)
			if len(filtered) > 0 {
				m.batchColumn = 1
				// Clamp index to valid range
				if m.batchBEIndex >= len(filtered) {
					m.batchBEIndex = len(filtered) - 1
				}
			}
		}
	case tea.KeySpace:
		m.toggleBatchSelection()
	case tea.KeyTab, tea.KeyEnter:
		// Count selected - do nothing if none selected
		count := 0
		for _, selected := range m.batchSelected {
			if selected {
				count++
			}
		}
		if count == 0 {
			return m, nil
		}
		if m.prType != nil {
			m.prTitle = m.prType.DefaultTitle("main")
		}
		// Check if any selected repos are still loading
		loadingCount := 0
		for i, selected := range m.batchSelected {
			if selected && i < len(m.batchRepoCommits) && m.batchRepoCommits[i] == nil {
				loadingCount++
			}
		}
		if loadingCount > 0 {
			// Wait for selected repos to finish - show loading but keep listening
			m.screen = ScreenLoading
			m.loadingMessage = fmt.Sprintf("Waiting for %d repo(s) to finish...", loadingCount)
			return m, listenForBatchCommits(m.batchResultsChan)
		}
		// All selected repos done - cancel remaining fetches and proceed
		m.cancelBatchFetch()
		// Go to loading screen to check for existing PRs (commits already cached)
		m.screen = ScreenLoading
		m.loadingMessage = "Checking for existing PRs..."
		return m, fetchBatchCommitsCmd(m.batchRepos, m.batchSelected, m.batchRepoCommits, m.prType, m.dryRun)
	case tea.KeyEsc:
		// Cancel background fetches and close channel
		m.cancelBatchFetch()
		m.screen = ScreenPrTypeSelect
		m.prType = nil
		m.menuIndex = 0
	case tea.KeyBackspace:
		if len(m.batchFilter) > 0 {
			m.batchFilter = m.batchFilter[:len(m.batchFilter)-1]
			m.batchFEIndex = 0
			m.batchBEIndex = 0
		}
	case tea.KeyCtrlC:
		m.shouldQuit = true
		return m, tea.Quit
	case tea.KeyRunes:
		// Type to filter - all printable characters go to filter
		m.batchFilter += string(msg.Runes)
		m.batchFEIndex = 0
		m.batchBEIndex = 0
	}
	return m, nil
}

// getFilteredBatchRepos returns indices of repos matching the current filter for the given column (0=frontend, 1=backend)
func (m *Model) getFilteredBatchRepos(column int) []int {
	var indices []int
	filter := strings.ToLower(m.batchFilter)

	for i, repo := range m.batchRepos {
		name := repo.DisplayName

		// Determine if this repo belongs to the column
		isFrontend := strings.Contains(name, "frontend/") || strings.HasPrefix(name, "frontend")
		isBackend := strings.Contains(name, "backend/") || strings.HasPrefix(name, "backend") ||
			strings.Contains(name, "services/") || strings.HasPrefix(name, "services")

		// Column 0 = frontend, column 1 = backend
		inColumn := (column == 0 && isFrontend) || (column == 1 && (isBackend || (!isFrontend && !isBackend)))

		if !inColumn {
			continue
		}

		// Apply filter
		if filter != "" && !strings.Contains(strings.ToLower(name), filter) {
			continue
		}

		indices = append(indices, i)
	}

	return indices
}

func (m *Model) navigateBatchColumn(up bool) {
	filtered := m.getFilteredBatchRepos(m.batchColumn)
	if len(filtered) == 0 {
		return
	}

	// Get current index for this column
	var currentIdx *int
	if m.batchColumn == 0 {
		currentIdx = &m.batchFEIndex
	} else {
		currentIdx = &m.batchBEIndex
	}

	if up {
		if *currentIdx > 0 {
			*currentIdx--
		} else {
			*currentIdx = len(filtered) - 1 // Wrap to bottom
		}
	} else {
		if *currentIdx < len(filtered)-1 {
			*currentIdx++
		} else {
			*currentIdx = 0 // Wrap to top
		}
	}
}

func (m *Model) toggleBatchSelection() {
	filtered := m.getFilteredBatchRepos(m.batchColumn)
	if len(filtered) == 0 {
		return
	}

	// Get current index for this column
	var currentIdx int
	if m.batchColumn == 0 {
		currentIdx = m.batchFEIndex
	} else {
		currentIdx = m.batchBEIndex
	}

	if currentIdx >= len(filtered) {
		return
	}

	// Get the actual repo index
	repoIdx := filtered[currentIdx]
	if repoIdx < len(m.batchSelected) {
		m.batchSelected[repoIdx] = !m.batchSelected[repoIdx]
	}
}

func (m Model) handleBatchSummaryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.shouldQuit = true
		return m, tea.Quit
	case "up":
		if m.menuIndex > 0 {
			m.menuIndex--
		}
	case "down":
		if m.menuIndex < len(m.batchResults)-1 {
			m.menuIndex++
		}
	case "o":
		// Open all PR URLs
		var urls []string
		for _, result := range m.batchResults {
			if result.PrURL != nil {
				urls = append(urls, *result.PrURL)
			}
		}
		openURLs(urls)
	case "c":
		// Copy all PR URLs as markdown list
		var lines []string
		for _, result := range m.batchResults {
			if result.PrURL != nil {
				lines = append(lines, fmt.Sprintf("- %s: %s", result.Repo.DisplayName, *result.PrURL))
			}
		}
		if len(lines) > 0 {
			if err := copyToClipboard(strings.Join(lines, "\n")); err == nil {
				m.copyFeedback = "✓ Copied URLs!"
			} else {
				m.copyFeedback = "✗ Copy failed"
			}
		}
		return m, nil
	case "enter", "esc":
		return m.reset()
	}
	return m, nil
}

func (m Model) handleViewOpenPrsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		m.navigateMergeColumn(true)
	case tea.KeyDown:
		m.navigateMergeColumn(false)
	case tea.KeyLeft:
		if m.mergeColumn != 0 {
			filtered := m.getFilteredMergePRs(0)
			if len(filtered) > 0 {
				m.mergeColumn = 0
				// Clamp index to valid range
				if m.mergeDevIndex >= len(filtered) {
					m.mergeDevIndex = len(filtered) - 1
				}
			}
		}
	case tea.KeyRight:
		if m.mergeColumn != 1 {
			filtered := m.getFilteredMergePRs(1)
			if len(filtered) > 0 {
				m.mergeColumn = 1
				// Clamp index to valid range
				if m.mergeMainIndex >= len(filtered) {
					m.mergeMainIndex = len(filtered) - 1
				}
			}
		}
	case tea.KeySpace:
		m.toggleMergeSelection()
	case tea.KeyTab, tea.KeyEnter:
		// Proceed to merge confirmation if any selected
		count := 0
		for _, selected := range m.mergeSelected {
			if selected {
				count++
			}
		}
		if count > 0 {
			m.screen = ScreenMergeConfirmation
			m.confirmSelection = 0
		}
	case tea.KeyEsc:
		m.openPRs = nil
		m.mergePRs = nil
		m.mergeSelected = nil
		m.screen = ScreenMainMenu
		m.menuIndex = 0
	case tea.KeyCtrlC:
		m.shouldQuit = true
		return m, tea.Quit
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "q":
			m.shouldQuit = true
			return m, tea.Quit
		case "a":
			m.selectAllInColumn()
		case "r":
			m.screen = ScreenLoading
			m.loadingMessage = "Fetching open PRs..."
			return m, fetchOpenPRsCmd(m.config, m.dryRun)
		case "o":
			// Open all PR URLs
			var urls []string
			for _, pr := range m.mergePRs {
				urls = append(urls, pr.URL)
			}
			openURLs(urls)
		case "c":
			// Copy all PR URLs as markdown list
			var lines []string
			for _, pr := range m.mergePRs {
				lines = append(lines, fmt.Sprintf("- %s: %s", pr.Repo.DisplayName, pr.URL))
			}
			if len(lines) > 0 {
				if err := copyToClipboard(strings.Join(lines, "\n")); err == nil {
					m.copyFeedback = "✓ Copied URLs!"
				} else {
					m.copyFeedback = "✗ Copy failed"
				}
			}
			return m, nil
		}
	}
	return m, nil
}

// getFilteredMergePRs returns indices of PRs for the given column (0=dev->staging, 1=staging->main)
func (m *Model) getFilteredMergePRs(column int) []int {
	var indices []int

	var targetType models.PrType
	if column == 0 {
		targetType = models.DevToStaging
	} else {
		targetType = models.StagingToMain
	}

	for i, pr := range m.mergePRs {
		if pr.PrType == targetType {
			indices = append(indices, i)
		}
	}

	return indices
}

func (m *Model) navigateMergeColumn(up bool) {
	filtered := m.getFilteredMergePRs(m.mergeColumn)
	if len(filtered) == 0 {
		return
	}

	// Get current index for this column
	var currentIdx *int
	if m.mergeColumn == 0 {
		currentIdx = &m.mergeDevIndex
	} else {
		currentIdx = &m.mergeMainIndex
	}

	if up {
		if *currentIdx > 0 {
			*currentIdx--
		} else {
			*currentIdx = len(filtered) - 1 // Wrap to bottom
		}
	} else {
		if *currentIdx < len(filtered)-1 {
			*currentIdx++
		} else {
			*currentIdx = 0 // Wrap to top
		}
	}
}

func (m *Model) toggleMergeSelection() {
	filtered := m.getFilteredMergePRs(m.mergeColumn)
	if len(filtered) == 0 {
		return
	}

	// Get current index for this column
	var currentIdx int
	if m.mergeColumn == 0 {
		currentIdx = m.mergeDevIndex
	} else {
		currentIdx = m.mergeMainIndex
	}

	if currentIdx >= len(filtered) {
		return
	}

	// Get the actual PR index
	prIdx := filtered[currentIdx]
	if prIdx < len(m.mergeSelected) {
		m.mergeSelected[prIdx] = !m.mergeSelected[prIdx]
	}
}

func (m *Model) selectAllInColumn() {
	filtered := m.getFilteredMergePRs(m.mergeColumn)
	if len(filtered) == 0 {
		return
	}

	// Check if all in column are selected
	allSelected := true
	for _, prIdx := range filtered {
		if prIdx < len(m.mergeSelected) && !m.mergeSelected[prIdx] {
			allSelected = false
			break
		}
	}

	// Toggle: if all selected, deselect all; otherwise select all
	newState := !allSelected
	for _, prIdx := range filtered {
		if prIdx < len(m.mergeSelected) {
			m.mergeSelected[prIdx] = newState
		}
	}
}

func (m Model) handleMergeSummaryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.shouldQuit = true
		return m, tea.Quit
	case "o":
		// Open URLs for successfully merged PRs
		var urls []string
		for _, result := range m.mergeResults {
			if result.Success {
				// Find the original PR to get its URL
				for _, pr := range m.mergePRs {
					if pr.Repo.DisplayName == result.RepoName && pr.PrNumber == result.PrNumber {
						urls = append(urls, pr.URL)
						break
					}
				}
			}
		}
		openURLs(urls)
	case "c":
		// Copy URLs for successfully merged PRs as markdown list
		var lines []string
		for _, result := range m.mergeResults {
			if result.Success {
				for _, pr := range m.mergePRs {
					if pr.Repo.DisplayName == result.RepoName && pr.PrNumber == result.PrNumber {
						lines = append(lines, fmt.Sprintf("- %s: %s", pr.Repo.DisplayName, pr.URL))
						break
					}
				}
			}
		}
		if len(lines) > 0 {
			if err := copyToClipboard(strings.Join(lines, "\n")); err == nil {
				m.copyFeedback = "✓ Copied URLs!"
			} else {
				m.copyFeedback = "✗ Copy failed"
			}
		}
		return m, nil
	case "enter", "esc":
		return m.reset()
	}
	return m, nil
}

func (m Model) reset() (tea.Model, tea.Cmd) {
	// Cancel any background fetches and close channel
	m.cancelBatchFetch()
	m.screen = ScreenMainMenu
	m.menuIndex = 0
	m.mode = nil
	m.repoInfo = nil
	m.prType = nil
	m.commits = nil
	m.tickets = nil
	m.prTitle = ""
	m.prURL = ""
	m.batchRepos = nil
	m.batchRepoCommits = nil
	m.batchResultsChan = nil
	m.batchFetchPending = 0
	m.batchSelected = nil
	m.batchResults = nil
	m.batchFilter = ""
	m.openPRs = nil
	m.mergePRs = nil
	m.mergeSelected = nil
	m.mergeResults = nil
	m.confirmSelection = 0
	// Reset animation state
	m.confetti = nil
	m.typewriterPos = 0
	return m, nil
}
