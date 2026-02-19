package app

import (
	"fmt"
	"strings"
	"time"

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

	case authCheckResult:
		m.authError = msg.err
		return m, nil

	case updateCheckResult:
		return m.handleUpdateCheckResult(msg)

	case updateDownloadResult:
		return m.handleUpdateDownloadResult(msg)

	case pullReposLoadedResult:
		return m.handlePullReposLoaded(msg)

	case pullRepoResult:
		return m.handlePullRepoResult(msg)

	case actionsRunsFetchedResult:
		return m.handleActionsRunsFetched(msg)

	case actionsRefreshTickMsg:
		return m.handleActionsRefreshTick()

	case actionsJobsFetchedResult:
		return m.handleActionsJobsFetched(msg)
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
	case ScreenUpdatePrompt:
		return m.handleUpdatePromptKey(msg)
	case ScreenSessionHistory:
		return m.handleSessionHistoryKey(msg)
	case ScreenPullBranchSelect:
		return m.handlePullBranchSelectKey(msg)
	case ScreenPullSummary:
		return m.handlePullSummaryKey(msg)
	case ScreenActionsOverview:
		return m.handleActionsOverviewKey(msg)
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
			m.menuIndex = 4 // Wrap to bottom
		}
	case "down", "j":
		if m.menuIndex < 4 {
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
	case "5":
		m.menuIndex = 4
		return m.selectMainMenuItem()
	case "a":
		m.menuIndex = 3
		return m.selectMainMenuItem()
	case "u":
		// Manual update check
		if m.updateCheckInProgress {
			return m, nil
		}
		m.updateCheckInProgress = true
		return m, checkUpdateCmd(m.version, m.config.Update.Repo)
	case "c":
		// Open config in editor
		return m, openConfigCmd()
	case "h":
		// View session history
		if len(m.sessionPRs) > 0 {
			m.screen = ScreenSessionHistory
			m.historyIndex = 0
		}
	case "p":
		// Pull all repos
		m.screen = ScreenPullBranchSelect
		m.menuIndex = 0
	}
	return m, nil
}

func (m Model) selectMainMenuItem() (tea.Model, tea.Cmd) {
	// Check for auth error before any GitHub operation (except Quit)
	if m.authError != nil && m.menuIndex != 4 {
		m.screen = ScreenError
		m.errorMessage = m.authError.Error()
		return m, nil
	}

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
		return m.navigateToMergePRs()
	case 3: // GitHub Actions
		m.actionsLoading = true
		m.screen = ScreenLoading
		m.loadingMessage = "Fetching workflow runs..."
		return m, fetchActionsRunsCmd(m.config, m.dryRun)
	case 4: // Quit
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
	}

	// Single mode - start fetching commits
	m.screen = ScreenLoading
	m.loadingMessage = "Fetching branches and commits..."
	return m, fetchCommitsCmd(m.repoInfo, m.prType, m.config.TicketRegex(), m.dryRun)
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
			m.prTitle = m.prType.DefaultTitle(m.mainBranch())
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
			m.prTitle = m.prType.DefaultTitle(m.mainBranch())
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
	case "m":
		return m.navigateToMergePRs()
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
			m.copyWithFeedback(formatted, "Copied URL!")
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
		if !repo.InColumn(column) {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(repo.DisplayName), filter) {
			continue
		}
		indices = append(indices, i)
	}

	return indices
}

// navigateColumnIndex wraps up/down navigation within a filtered list
func navigateColumnIndex(idx *int, listLen int, up bool) {
	if listLen == 0 {
		return
	}
	if up {
		if *idx > 0 {
			*idx--
		} else {
			*idx = listLen - 1
		}
	} else {
		if *idx < listLen-1 {
			*idx++
		} else {
			*idx = 0
		}
	}
}

func (m *Model) navigateBatchColumn(up bool) {
	filtered := m.getFilteredBatchRepos(m.batchColumn)
	if m.batchColumn == 0 {
		navigateColumnIndex(&m.batchFEIndex, len(filtered), up)
	} else {
		navigateColumnIndex(&m.batchBEIndex, len(filtered), up)
	}
}

// toggleSelection toggles a boolean in a selection slice at the index pointed to by the current column position
func toggleSelection(selected []bool, filtered []int, currentIdx int) {
	if currentIdx >= len(filtered) {
		return
	}
	idx := filtered[currentIdx]
	if idx < len(selected) {
		selected[idx] = !selected[idx]
	}
}

func (m *Model) toggleBatchSelection() {
	filtered := m.getFilteredBatchRepos(m.batchColumn)
	currentIdx := m.batchFEIndex
	if m.batchColumn == 1 {
		currentIdx = m.batchBEIndex
	}
	toggleSelection(m.batchSelected, filtered, currentIdx)
}

func (m Model) handleBatchSummaryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.shouldQuit = true
		return m, tea.Quit
	case "m":
		return m.navigateToMergePRs()
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
			m.copyWithFeedback(strings.Join(lines, "\n"), "Copied URLs!")
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
				m.copyWithFeedback(strings.Join(lines, "\n"), "Copied URLs!")
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
	if m.mergeColumn == 0 {
		navigateColumnIndex(&m.mergeDevIndex, len(filtered), up)
	} else {
		navigateColumnIndex(&m.mergeMainIndex, len(filtered), up)
	}
}

func (m *Model) toggleMergeSelection() {
	filtered := m.getFilteredMergePRs(m.mergeColumn)
	currentIdx := m.mergeDevIndex
	if m.mergeColumn == 1 {
		currentIdx = m.mergeMainIndex
	}
	toggleSelection(m.mergeSelected, filtered, currentIdx)
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
			m.copyWithFeedback(strings.Join(lines, "\n"), "Copied URLs!")
		}
		return m, nil
	case "enter", "esc":
		return m.reset()
	}
	return m, nil
}

func (m Model) handleUpdateCheckResult(msg updateCheckResult) (tea.Model, tea.Cmd) {
	m.updateCheckInProgress = false

	// Record that we checked (regardless of result)
	m.config.RecordUpdateCheck()
	_ = m.config.Save()

	if msg.err != nil {
		// Silently ignore update check errors
		return m, nil
	}

	if msg.release == nil {
		// No update available
		return m, nil
	}

	// Check if this version was skipped
	if m.config.Update.SkippedVersion == msg.release.TagName {
		return m, nil
	}

	// Only show update prompt if user is still on main menu
	// (don't interrupt if they've started navigating)
	if m.screen != ScreenMainMenu {
		return m, nil
	}

	// Update available - show prompt
	m.updateAvailable = msg.release
	m.screen = ScreenUpdatePrompt
	m.updateSelection = 0
	return m, nil
}

func (m Model) handleUpdateDownloadResult(msg updateDownloadResult) (tea.Model, tea.Cmd) {
	if !msg.success {
		m.errorMessage = fmt.Sprintf("Update failed: %v", msg.err)
		m.screen = ScreenError
		return m, nil
	}

	// Update successful - quit so user restarts with new version
	m.shouldQuit = true
	// Return a quit command with a message
	return m, tea.Sequence(
		tea.Printf("\nUpdated to %s! Run attpr again.\n", msg.version),
		tea.Quit,
	)
}

func (m Model) handleUpdatePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		if m.updateSelection > 0 {
			m.updateSelection--
		}
	case "right", "l":
		if m.updateSelection < 2 {
			m.updateSelection++
		}
	case "y", "1":
		m.updateSelection = 0
		return m.executeUpdateSelection()
	case "n", "2":
		m.updateSelection = 1
		return m.executeUpdateSelection()
	case "s", "3":
		m.updateSelection = 2
		return m.executeUpdateSelection()
	case "enter":
		return m.executeUpdateSelection()
	case "q", "esc":
		m.updateAvailable = nil
		m.screen = ScreenMainMenu
	}
	return m, nil
}

func (m Model) executeUpdateSelection() (tea.Model, tea.Cmd) {
	switch m.updateSelection {
	case 0: // Update now
		m.screen = ScreenUpdating
		return m, downloadUpdateCmd(m.updateAvailable, m.config.Update.Repo)
	case 1: // Skip for now
		m.updateAvailable = nil
		m.screen = ScreenMainMenu
	case 2: // Skip this version
		if m.updateAvailable != nil {
			m.config.Update.SkippedVersion = m.updateAvailable.TagName
			_ = m.config.Save()
		}
		m.updateAvailable = nil
		m.screen = ScreenMainMenu
	}
	return m, nil
}

func (m Model) handleSessionHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.shouldQuit = true
		return m, tea.Quit
	case "up", "k":
		if m.historyIndex > 0 {
			m.historyIndex--
		}
	case "down", "j":
		if m.historyIndex < len(m.sessionPRs)-1 {
			m.historyIndex++
		}
	case "o":
		// Open selected URL
		if m.historyIndex < len(m.sessionPRs) {
			_ = openURL(m.sessionPRs[m.historyIndex].url)
		}
	case "c":
		// Copy selected URL as markdown
		if m.historyIndex < len(m.sessionPRs) {
			pr := m.sessionPRs[m.historyIndex]
			formatted := fmt.Sprintf("- %s: %s", pr.repoName, pr.url)
			m.copyWithFeedback(formatted, "Copied!")
		}
		return m, nil
	case "esc", "enter":
		m.screen = ScreenMainMenu
		m.menuIndex = 0
	}
	return m, nil
}

// GitHub Actions key handlers

func (m Model) handleActionsOverviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.getFilteredActions()

	// Right column: up/down navigates pinned panels
	if m.actionsColumn == 1 {
		switch msg.Type {
		case tea.KeyUp:
			if m.actionsPinnedIndex > 0 {
				m.actionsPinnedIndex--
				m.adjustActionsPinnedScroll()
			}
		case tea.KeyDown:
			if m.actionsPinnedIndex < len(m.actionsPinned)-1 {
				m.actionsPinnedIndex++
				m.adjustActionsPinnedScroll()
			}
		case tea.KeyLeft:
			m.actionsColumn = 0
		case tea.KeyEsc:
			m.actionsColumn = 0
		case tea.KeyCtrlC:
			m.shouldQuit = true
			return m, tea.Quit
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "o":
				if m.actionsPinnedIndex < len(m.actionsPinned) {
					panel := m.actionsPinned[m.actionsPinnedIndex]
					if panel.Run.URL != "" {
						_ = openURL(panel.Run.URL)
					}
				}
			case "q":
				m.shouldQuit = true
				return m, tea.Quit
			}
		}
		return m, nil
	}

	// Left column
	switch msg.Type {
	case tea.KeyUp:
		navigateColumnIndex(&m.actionsIndex, len(filtered), true)
		m.adjustActionsRunScroll(filtered)
	case tea.KeyDown:
		navigateColumnIndex(&m.actionsIndex, len(filtered), false)
		m.adjustActionsRunScroll(filtered)
	case tea.KeyLeft:
		// no-op, already in left column
	case tea.KeyRight:
		if len(m.actionsPinned) > 0 {
			m.actionsColumn = 1
			m.actionsPinnedIndex = 0
		}
	case tea.KeySpace:
		if m.actionsIndex >= len(filtered) {
			return m, nil
		}
		entry := m.actionsEntries[filtered[m.actionsIndex]]
		if m.unpinRun(entry.Run.DatabaseID) {
			return m, nil
		}
		m.actionsPinned = append(m.actionsPinned, actionsPanel{
			Run:  entry.Run,
			Repo: entry.Repo,
		})
		return m, fetchActionsJobsCmd(entry.Repo.Path, entry.Run.DatabaseID, m.dryRun)
	case tea.KeyEsc:
		if m.actionsFilterActive {
			m.actionsFilterActive = false
			m.actionsFilter = ""
			m.actionsIndex = 0
			m.actionsRunScroll = 0
		} else {
			return m.reset()
		}
	case tea.KeyBackspace:
		if m.actionsFilterActive && len(m.actionsFilter) > 0 {
			m.actionsFilter = m.actionsFilter[:len(m.actionsFilter)-1]
			m.actionsIndex = 0
			m.actionsRunScroll = 0
		}
	case tea.KeyCtrlC:
		m.shouldQuit = true
		return m, tea.Quit
	case tea.KeyRunes:
		key := string(msg.Runes)
		if m.actionsFilterActive {
			m.actionsFilter += key
			m.actionsIndex = 0
			m.actionsRunScroll = 0
			return m, nil
		}
		switch key {
		case "q":
			m.shouldQuit = true
			return m, tea.Quit
		case "/":
			m.actionsFilterActive = true
		case "a":
			var cmds []tea.Cmd
			for _, idx := range filtered {
				entry := m.actionsEntries[idx]
				if m.isPinned(entry.Run.DatabaseID) {
					continue
				}
				m.actionsPinned = append(m.actionsPinned, actionsPanel{
					Run:  entry.Run,
					Repo: entry.Repo,
				})
				cmds = append(cmds, fetchActionsJobsCmd(entry.Repo.Path, entry.Run.DatabaseID, m.dryRun))
			}
			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
		case "n":
			m.actionsPinned = nil
			m.actionsPinnedIndex = 0
		case "o":
			if m.actionsIndex < len(filtered) {
				entry := m.actionsEntries[filtered[m.actionsIndex]]
				if entry.Run.URL != "" {
					_ = openURL(entry.Run.URL)
				}
			}
		}
	}
	return m, nil
}

// getFilteredActions returns indices of entries matching the text filter (flat, no columns)
func (m *Model) getFilteredActions() []int {
	filter := strings.ToLower(m.actionsFilter)
	var indices []int
	for i, entry := range m.actionsEntries {
		if filter == "" || matchesActionsFilter(entry, filter) {
			indices = append(indices, i)
		}
	}
	return indices
}

func matchesActionsFilter(entry actionsEntry, filter string) bool {
	return strings.Contains(strings.ToLower(entry.Repo.DisplayName), filter) ||
		strings.Contains(strings.ToLower(entry.Run.WorkflowName), filter) ||
		strings.Contains(strings.ToLower(entry.Run.HeadBranch), filter) ||
		strings.Contains(strings.ToLower(entry.Run.DisplayTitle), filter)
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
	m.batchFetchPending = 0
	m.batchSelected = nil
	m.batchResults = nil
	m.batchFilter = ""
	m.openPRs = nil
	m.mergePRs = nil
	m.mergeSelected = nil
	m.mergeResults = nil
	m.confirmSelection = 0
	// Reset update state
	m.updateAvailable = nil
	m.updateSelection = 0
	// Reset animation state
	m.confetti = nil
	m.typewriterPos = 0
	// Reset pull state
	m.pullBranch = ""
	m.pullRepos = nil
	m.pullResults = nil
	m.pullCurrentIdx = 0
	// Reset actions state
	m.actionsEntries = nil
	m.actionsIndex = 0
	m.actionsLoading = false
	m.actionsLastRefresh = time.Time{}
	m.actionsFilter = ""
	m.actionsFilterActive = false
	m.actionsPinned = nil
	m.actionsColumn = 0
	m.actionsPinnedIndex = 0
	m.actionsPinnedScroll = 0
	m.actionsRunScroll = 0
	return m, nil
}

func (m Model) navigateToMergePRs() (tea.Model, tea.Cmd) {
	mode := ModeBatch
	m.mode = &mode
	m.screen = ScreenLoading
	m.loadingMessage = "Fetching open PRs..."
	return m, fetchOpenPRsCmd(m.config, m.dryRun)
}

func (m Model) handlePullBranchSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.shouldQuit = true
		return m, tea.Quit
	case "up", "k":
		if m.menuIndex > 0 {
			m.menuIndex--
		} else {
			m.menuIndex = 2 // Wrap to bottom (3 options: dev, staging, main)
		}
	case "down", "j":
		if m.menuIndex < 2 {
			m.menuIndex++
		} else {
			m.menuIndex = 0 // Wrap to top
		}
	case "enter":
		return m.selectPullBranch()
	case "1":
		m.menuIndex = 0
		return m.selectPullBranch()
	case "2":
		m.menuIndex = 1
		return m.selectPullBranch()
	case "3":
		m.menuIndex = 2
		return m.selectPullBranch()
	case "esc":
		m.screen = ScreenMainMenu
		m.menuIndex = 0
	}
	return m, nil
}

func (m Model) selectPullBranch() (tea.Model, tea.Cmd) {
	branches := []string{"dev", "staging", "main"}
	m.pullBranch = branches[m.menuIndex]
	m.screen = ScreenLoading
	m.loadingMessage = fmt.Sprintf("Scanning repositories for %s...", m.pullBranch)
	return m, loadPullReposCmd(m.config)
}

func (m Model) handlePullSummaryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.shouldQuit = true
		return m, tea.Quit
	case "enter", "esc":
		return m.reset()
	}
	return m, nil
}
