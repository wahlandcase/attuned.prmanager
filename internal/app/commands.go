package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wahlandcase/attuned.prmanager/internal/config"
	"github.com/wahlandcase/attuned.prmanager/internal/git"
	"github.com/wahlandcase/attuned.prmanager/internal/github"
	"github.com/wahlandcase/attuned.prmanager/internal/models"
	"github.com/wahlandcase/attuned.prmanager/internal/update"

	tea "github.com/charmbracelet/bubbletea"
)

// Message types for async operations

type fetchCommitsResult struct {
	commits    []models.CommitInfo
	tickets    []string
	existingPR *models.GhPr
	err        error
}

type prCreatedResult struct {
	url string
	err error
}

type batchRepoResult struct {
	result models.BatchResult
}

type openPRsFetchedResult struct {
	entries []OpenPREntry
	err     error
}

type mergeCompleteResult struct {
	result models.MergeResult
}

type authCheckResult struct {
	err error
}

// authCheckCmd runs gh auth check in the background
func authCheckCmd() tea.Cmd {
	return func() tea.Msg {
		err := github.CheckAuth()
		return authCheckResult{err: err}
	}
}

// Update check messages
type updateCheckResult struct {
	release *update.Release
	err     error
}

type updateDownloadResult struct {
	success bool
	version string
	err     error
}

// checkUpdateCmd checks for available updates
func checkUpdateCmd(currentVersion, repo string) tea.Cmd {
	return func() tea.Msg {
		release, err := update.CheckForUpdate(currentVersion, repo)
		return updateCheckResult{release: release, err: err}
	}
}

// downloadUpdateCmd downloads and installs an update
func downloadUpdateCmd(release *update.Release, repo string) tea.Cmd {
	return func() tea.Msg {
		err := update.DownloadAndInstall(release, repo)
		if err != nil {
			return updateDownloadResult{success: false, err: err}
		}
		return updateDownloadResult{success: true, version: update.VersionDisplay(release.TagName)}
	}
}

// openConfigCmd opens the config folder in the system file manager
func openConfigCmd() tea.Cmd {
	return func() tea.Msg {
		configPath, err := config.Path()
		if err != nil {
			return nil
		}
		configDir := filepath.Dir(configPath)

		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			// macOS: open folder in Finder, select the file
			cmd = exec.Command("open", "-R", configPath)
		case "linux":
			// Check if WSL
			if isWSL() {
				// Convert Linux path to Windows path and open in Explorer
				winPath, err := exec.Command("wslpath", "-w", configDir).Output()
				if err == nil {
					cmd = exec.Command("explorer.exe", strings.TrimSpace(string(winPath)))
				}
			} else {
				cmd = exec.Command("xdg-open", configDir)
			}
		}

		if cmd != nil {
			cmd.Start()
		}
		return nil
	}
}

type batchCommitsResult struct {
	tickets          []string
	existingPRs      int // Count of repos with existing PRs
	reposWithCommits int // Count of repos that have commits to merge
	err              error
}

// batchProgressMsg is sent for real-time progress updates during batch processing
type batchProgressMsg struct {
	step string
}

// listenForProgress creates a subscription that listens to the progress channel
func listenForProgress(ch chan string) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return nil
		}
		step, ok := <-ch
		if !ok {
			return nil
		}
		return batchProgressMsg{step: step}
	}
}

// Commands

func fetchCommitsCmd(repo *models.RepoInfo, prType *models.PrType, ticketRegex *regexp.Regexp, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		// Dry run mode: return fake commits
		if dryRun {
			time.Sleep(800 * time.Millisecond)
			commits := []models.CommitInfo{
				{Hash: "abc1234", Message: "feat: Add new dashboard component", Tickets: []string{"ATT-1234"}},
				{Hash: "def5678", Message: "fix: Resolve authentication bug", Tickets: []string{"ATT-1235"}},
				{Hash: "ghi9012", Message: "chore: Update dependencies", Tickets: []string{}},
				{Hash: "jkl3456", Message: "feat: Implement user settings page", Tickets: []string{"ATT-1236", "ATT-1237"}},
				{Hash: "mno7890", Message: "docs: Update README with new instructions", Tickets: []string{}},
			}
			tickets := []string{"ATT-1234", "ATT-1235", "ATT-1236", "ATT-1237"}
			return fetchCommitsResult{commits: commits, tickets: tickets}
		}

		if repo == nil || prType == nil {
			return fetchCommitsResult{err: nil}
		}

		headBranch := prType.HeadBranch()
		baseBranch := prType.BaseBranch(repo.MainBranch)

		// Fetch branches from remote
		if err := git.FetchBranches(repo.Path, []string{headBranch, baseBranch}); err != nil {
			return fetchCommitsResult{err: err}
		}

		// Get commits between branches
		commits, err := git.GetCommitsBetween(repo.Path, baseBranch, headBranch, ticketRegex)
		if err != nil {
			return fetchCommitsResult{err: err}
		}

		// Extract all unique tickets
		tickets := git.GetAllTickets(commits)

		// Check for existing PR
		existingPR, _ := github.GetExistingPR(repo.Path, headBranch, baseBranch)

		return fetchCommitsResult{commits: commits, tickets: tickets, existingPR: existingPR}
	}
}

func fetchBatchCommitsCmd(repos []models.RepoInfo, selected []bool, cachedCommits []*[]models.CommitInfo, prType *models.PrType, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		if dryRun {
			time.Sleep(300 * time.Millisecond)
			// Count selected repos for dry run
			selectedCount := 0
			for i := range repos {
				if i < len(selected) && selected[i] {
					selectedCount++
				}
			}
			return batchCommitsResult{tickets: []string{"ATT-1234", "ATT-1235", "ATT-1236"}, existingPRs: 1, reposWithCommits: selectedCount}
		}

		if prType == nil {
			return batchCommitsResult{err: nil}
		}

		// Collect selected repos with their cached commits
		type selectedRepo struct {
			repo    models.RepoInfo
			commits []models.CommitInfo
		}
		var selectedRepos []selectedRepo
		for i, repo := range repos {
			if i < len(selected) && selected[i] {
				var commits []models.CommitInfo
				if i < len(cachedCommits) && cachedCommits[i] != nil {
					commits = *cachedCommits[i]
				}
				selectedRepos = append(selectedRepos, selectedRepo{repo: repo, commits: commits})
			}
		}

		if len(selectedRepos) == 0 {
			return batchCommitsResult{tickets: nil}
		}

		// Only check for existing PRs in parallel (commits already cached)
		type repoResult struct {
			hasExisting bool
		}
		results := make(chan repoResult, len(selectedRepos))

		var wg sync.WaitGroup
		for _, sr := range selectedRepos {
			wg.Add(1)
			go func(r models.RepoInfo) {
				defer wg.Done()

				headBranch := prType.HeadBranch()
				baseBranch := prType.BaseBranch(r.MainBranch)

				// Check for existing PR (no need to re-fetch commits)
				existingPR, _ := github.GetExistingPR(r.Path, headBranch, baseBranch)

				results <- repoResult{hasExisting: existingPR != nil}
			}(sr.repo)
		}

		// Close channel when done
		go func() {
			wg.Wait()
			close(results)
		}()

		// Aggregate tickets from cached commits and count existing PRs
		ticketSet := make(map[string]bool)
		withCommitsCount := 0
		for _, sr := range selectedRepos {
			tickets := git.GetAllTickets(sr.commits)
			for _, t := range tickets {
				ticketSet[t] = true
			}
			if len(sr.commits) > 0 {
				withCommitsCount++
			}
		}

		existingCount := 0
		for res := range results {
			if res.hasExisting {
				existingCount++
			}
		}

		var allTickets []string
		for t := range ticketSet {
			allTickets = append(allTickets, t)
		}

		return batchCommitsResult{tickets: allTickets, existingPRs: existingCount, reposWithCommits: withCommitsCount}
	}
}

func createPRCmd(repo *models.RepoInfo, prType *models.PrType, title string, tickets []string, linearOrg string, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		// Dry run mode: return fake URL
		if dryRun {
			time.Sleep(1500 * time.Millisecond)
			repoName := "example-repo"
			if repo != nil {
				repoName = repo.DisplayName
			}
			return prCreatedResult{url: "https://github.com/example/" + repoName + "/pull/123 (DRY RUN)"}
		}

		if repo == nil || prType == nil {
			return prCreatedResult{err: nil}
		}

		headBranch := prType.HeadBranch()
		baseBranch := prType.BaseBranch(repo.MainBranch)

		// Create or update PR
		pr, _, err := github.CreateOrUpdatePR(repo.Path, headBranch, baseBranch, title, tickets, linearOrg)
		if err != nil {
			return prCreatedResult{err: err}
		}

		return prCreatedResult{url: pr.URL}
	}
}

func fetchOpenPRsCmd(cfg *config.Config, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		// Dry run mode: return fake PRs
		if dryRun {
			time.Sleep(1000 * time.Millisecond)
			entries := []OpenPREntry{
				{
					Repo: models.RepoInfo{
						Path:        "/home/user/repos/frontend/web",
						DisplayName: "frontend/web",
						MainBranch:  "main",
					},
					Status: models.RepoPrStatus{
						DevToStaging: &models.GhPr{
							Number: 123,
							URL:    "https://github.com/example/web/pull/123",
							Title:  "dev → staging",
							State:  "open",
						},
						StagingToMain: &models.GhPr{
							Number: 124,
							URL:    "https://github.com/example/web/pull/124",
							Title:  "staging → main",
							State:  "open",
						},
					},
				},
				{
					Repo: models.RepoInfo{
						Path:        "/home/user/repos/backend/api",
						DisplayName: "backend/api",
						MainBranch:  "main",
					},
					Status: models.RepoPrStatus{
						DevToStaging: &models.GhPr{
							Number: 456,
							URL:    "https://github.com/example/api/pull/456",
							Title:  "dev → staging",
							State:  "open",
						},
					},
				},
			}
			return openPRsFetchedResult{entries: entries}
		}

		// Find all repos
		repos, err := git.FindAttunedRepos(cfg.AttunedPath(), cfg.Paths.FrontendGlob, cfg.Paths.BackendGlob)
		if err != nil {
			return openPRsFetchedResult{err: err}
		}

		// Fetch open PRs in parallel
		type result struct {
			entry  OpenPREntry
			hasAny bool
			err    error
		}

		var wg sync.WaitGroup
		results := make(chan result, len(repos))

		for _, repo := range repos {
			wg.Add(1)
			go func(r models.RepoInfo) {
				defer wg.Done()

				status, err := github.GetOpenReleasePRs(r.Path, r.MainBranch)
				if err != nil {
					results <- result{err: fmt.Errorf("%s: %w", r.DisplayName, err)}
					return
				}
				hasAny := status.DevToStaging != nil || status.StagingToMain != nil

				results <- result{
					entry:  OpenPREntry{Repo: r, Status: *status},
					hasAny: hasAny,
				}
			}(repo)
		}

		// Close results channel when all goroutines complete
		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect results, filtering to only repos with open PRs
		// Skip repos that error (e.g., no remotes) instead of failing entirely
		var entries []OpenPREntry
		for res := range results {
			if res.err != nil {
				continue // Skip problematic repos
			}
			if res.hasAny {
				entries = append(entries, res.entry)
			}
		}

		return openPRsFetchedResult{entries: entries}
	}
}

// sendProgress safely sends a progress update to the channel
func sendProgress(ch chan string, step string) {
	if ch != nil {
		select {
		case ch <- step:
		default:
			// Channel full or closed, skip
		}
	}
}

func startBatchProcessingCmd(m *Model, repoIndex int) tea.Cmd {
	return func() tea.Msg {
		if repoIndex >= len(m.batchRepos) {
			return nil
		}

		repo := m.batchRepos[repoIndex]
		progressCh := m.batchProgressChan

		if repoIndex >= len(m.batchSelected) || !m.batchSelected[repoIndex] {
			// Skip unselected repos
			return batchRepoResult{result: models.BatchResult{
				Repo:   repo,
				Status: models.Skipped("Not selected"),
			}}
		}

		if m.dryRun {
			sendProgress(progressCh, "Simulating PR creation...")
			time.Sleep(500 * time.Millisecond)
			url := "https://github.com/example/" + repo.DisplayName + "/pull/123 (DRY RUN)"
			return batchRepoResult{result: models.BatchResult{
				Repo:   repo,
				Status: models.Created,
				PrURL:  &url,
			}}
		}

		// Use the selected PR type
		if m.prType == nil {
			return batchRepoResult{result: models.BatchResult{
				Repo:   repo,
				Status: models.Failed("No PR type selected"),
			}}
		}
		prType := *m.prType
		headBranch := prType.HeadBranch()
		baseBranch := prType.BaseBranch(repo.MainBranch)

		// Fetch branches
		sendProgress(progressCh, "Fetching branches...")
		if err := git.FetchBranches(repo.Path, []string{headBranch, baseBranch}); err != nil {
			return batchRepoResult{result: models.BatchResult{
				Repo:   repo,
				Status: models.Failed(err.Error()),
			}}
		}

		// Get commits
		sendProgress(progressCh, "Getting commits...")
		commits, err := git.GetCommitsBetween(repo.Path, baseBranch, headBranch, m.config.TicketRegex())
		if err != nil {
			return batchRepoResult{result: models.BatchResult{
				Repo:   repo,
				Status: models.Failed(err.Error()),
			}}
		}

		if len(commits) == 0 {
			return batchRepoResult{result: models.BatchResult{
				Repo:   repo,
				Status: models.Skipped("No commits to merge"),
			}}
		}

		tickets := git.GetAllTickets(commits)

		// Create or update PR
		sendProgress(progressCh, "Creating PR...")
		pr, updated, err := github.CreateOrUpdatePR(repo.Path, headBranch, baseBranch, m.prTitle, tickets, m.config.Tickets.LinearOrg)
		if err != nil {
			return batchRepoResult{result: models.BatchResult{
				Repo:   repo,
				Status: models.Failed(err.Error()),
			}}
		}

		status := models.Created
		if updated {
			status = models.Updated
		}

		return batchRepoResult{result: models.BatchResult{
			Repo:    repo,
			Status:  status,
			PrURL:   &pr.URL,
			Tickets: tickets,
		}}
	}
}

func startMergingCmd(m *Model, prIndex int) tea.Cmd {
	return func() tea.Msg {
		if prIndex >= len(m.mergePRs) {
			return nil
		}

		pr := m.mergePRs[prIndex]
		if prIndex >= len(m.mergeSelected) || !m.mergeSelected[prIndex] {
			// Skip unselected PRs
			return nil
		}

		if m.dryRun {
			time.Sleep(500 * time.Millisecond)
			return mergeCompleteResult{result: models.MergeResult{
				RepoName: pr.Repo.DisplayName,
				PrNumber: pr.PrNumber,
				Success:  true,
			}}
		}

		// Merge the PR
		err := github.MergePR(pr.Repo.Path, pr.PrNumber)
		if err != nil {
			errStr := err.Error()
			return mergeCompleteResult{result: models.MergeResult{
				RepoName: pr.Repo.DisplayName,
				PrNumber: pr.PrNumber,
				Success:  false,
				Error:    &errStr,
			}}
		}

		return mergeCompleteResult{result: models.MergeResult{
			RepoName: pr.Repo.DisplayName,
			PrNumber: pr.PrNumber,
			Success:  true,
		}}
	}
}

// Message types for repo loading
type batchReposLoadedResult struct {
	repos      []models.RepoInfo
	cancelFunc func() // Cancel function for background fetch
	err        error
}

// Single repo commit fetch result (sent incrementally from background)
type batchRepoCommitResult struct {
	index   int
	commits []models.CommitInfo
}

type currentRepoLoadedResult struct {
	repo *models.RepoInfo
	err  error
}

// loadBatchReposCmd loads repos and starts background commit fetching
func loadBatchReposCmd(cfg *config.Config, prType *models.PrType, dryRun bool, resultsChan chan batchRepoCommitResult) tea.Cmd {
	return func() tea.Msg {
		repos, err := git.FindAttunedRepos(cfg.AttunedPath(), cfg.Paths.FrontendGlob, cfg.Paths.BackendGlob)
		if err != nil {
			return batchReposLoadedResult{err: err}
		}

		if len(repos) == 0 {
			return batchReposLoadedResult{repos: repos}
		}

		// Create cancellation context
		ctx, cancel := context.WithCancel(context.Background())

		// Start background fetches for all repos
		go func() {
			defer close(resultsChan) // Close channel when all workers done or cancelled

			var wg sync.WaitGroup
			for i, repo := range repos {
				wg.Add(1)
				go func(idx int, r models.RepoInfo) {
					defer wg.Done()

					// Check for cancellation
					select {
					case <-ctx.Done():
						return
					default:
					}

					var commits []models.CommitInfo

					if dryRun {
						// Simulate network delay
						time.Sleep(time.Duration(100+idx*50) * time.Millisecond)
						if idx%3 != 0 {
							commits = []models.CommitInfo{
								{Hash: "abc1234", Message: "feat: Add new feature", Tickets: []string{"ATT-1234"}},
								{Hash: "def5678", Message: "fix: Bug fix", Tickets: []string{"ATT-1235"}},
							}
						}
					} else if prType != nil {
						headBranch := prType.HeadBranch()
						baseBranch := prType.BaseBranch(r.MainBranch)

						// Fetch from remote (network call)
						if err := git.FetchBranches(r.Path, []string{headBranch, baseBranch}); err == nil {
							commits, _ = git.GetCommitsBetween(r.Path, baseBranch, headBranch, cfg.TicketRegex())
						}
					}

					// Send result (check cancellation again)
					select {
					case <-ctx.Done():
						return
					case resultsChan <- batchRepoCommitResult{index: idx, commits: commits}:
					}
				}(i, repo)
			}
			wg.Wait()
		}()

		return batchReposLoadedResult{repos: repos, cancelFunc: cancel}
	}
}

// listenForBatchCommits creates a command that listens for commit results
func listenForBatchCommits(resultsChan chan batchRepoCommitResult) tea.Cmd {
	return func() tea.Msg {
		result, ok := <-resultsChan
		if !ok {
			return nil // Channel closed
		}
		return result
	}
}

// loadCurrentRepoCmd loads info for the current repository
func loadCurrentRepoCmd() tea.Cmd {
	return func() tea.Msg {
		repo, err := git.GetCurrentRepoInfo()
		if err != nil {
			return currentRepoLoadedResult{err: err}
		}
		return currentRepoLoadedResult{repo: repo}
	}
}

// Result handlers

func (m Model) handleBatchReposLoaded(msg batchReposLoadedResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMessage = msg.err.Error()
		m.screen = ScreenError
		return m, nil
	}

	m.batchRepos = msg.repos
	m.batchRepoCommits = make([]*[]models.CommitInfo, len(msg.repos)) // All nil = loading
	m.batchSelected = make([]bool, len(msg.repos))
	m.batchFetchCancel = msg.cancelFunc
	m.batchFetchPending = len(msg.repos)
	m.screen = ScreenBatchRepoSelect
	m.batchColumn = 0
	m.batchFEIndex = 0
	m.batchBEIndex = 0

	// Start listening for commit results
	if len(msg.repos) > 0 && m.batchResultsChan != nil {
		return m, listenForBatchCommits(m.batchResultsChan)
	}
	return m, nil
}

func (m Model) handleBatchRepoCommitResult(msg batchRepoCommitResult) (tea.Model, tea.Cmd) {
	// Update commits for this repo
	if msg.index >= 0 && msg.index < len(m.batchRepoCommits) {
		commits := msg.commits // Make a copy to get a stable pointer
		m.batchRepoCommits[msg.index] = &commits
	}

	m.batchFetchPending--

	// If we're waiting for selected repos to finish (Loading screen)
	if m.screen == ScreenLoading && m.batchResultsChan != nil {
		// Check if all selected repos are now done
		allDone := true
		for i, selected := range m.batchSelected {
			if selected && i < len(m.batchRepoCommits) && m.batchRepoCommits[i] == nil {
				allDone = false
				break
			}
		}
		if allDone {
			// All selected repos done - cancel remaining and proceed
			m.cancelBatchFetch()
			m.loadingMessage = "Checking for existing PRs..."
			return m, fetchBatchCommitsCmd(m.batchRepos, m.batchSelected, m.batchRepoCommits, m.prType, m.dryRun)
		}
		// Still waiting - keep listening
		return m, listenForBatchCommits(m.batchResultsChan)
	}

	// Keep listening if more results pending and still on batch select screen
	if m.batchFetchPending > 0 && m.screen == ScreenBatchRepoSelect && m.batchResultsChan != nil {
		return m, listenForBatchCommits(m.batchResultsChan)
	}
	return m, nil
}

func (m Model) handleCurrentRepoLoaded(msg currentRepoLoadedResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMessage = "Not in a git repository: " + msg.err.Error()
		m.screen = ScreenError
		return m, nil
	}

	m.repoInfo = msg.repo
	m.screen = ScreenPrTypeSelect
	m.menuIndex = 0
	return m, nil
}

func (m Model) handleFetchCommitsResult(msg fetchCommitsResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMessage = msg.err.Error()
		m.screen = ScreenError
		return m, nil
	}

	m.commits = msg.commits
	m.tickets = msg.tickets
	m.existingPR = msg.existingPR
	m.screen = ScreenCommitReview
	m.menuIndex = 0
	return m, nil
}

func (m Model) handleBatchCommitsResult(msg batchCommitsResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMessage = msg.err.Error()
		m.screen = ScreenError
		return m, nil
	}

	m.tickets = msg.tickets
	m.batchExistingPRs = msg.existingPRs
	m.batchReposWithCommits = msg.reposWithCommits
	m.screen = ScreenTitleInput
	return m, nil
}

// recordSessionPR adds a PR to session history if it's a staging→main PR
func (m *Model) recordSessionPR(repoName, url string) {
	if m.prType == nil || *m.prType != models.StagingToMain {
		return
	}
	m.sessionPRs = append(m.sessionPRs, sessionPR{
		repoName:  repoName,
		url:       url,
		prType:    "staging→main",
		createdAt: time.Now(),
	})
	saveHistory(m.sessionPRs)
}

func (m Model) handlePrCreatedResult(msg prCreatedResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMessage = msg.err.Error()
		m.screen = ScreenError
		return m, nil
	}

	m.prURL = msg.url
	m.screen = ScreenComplete
	m.spawnConfetti()

	if m.repoInfo != nil {
		m.recordSessionPR(m.repoInfo.DisplayName, msg.url)
	}
	return m, nil
}

func (m Model) handleBatchRepoResult(msg batchRepoResult) (tea.Model, tea.Cmd) {
	// Only add non-skipped "not selected" results to keep summary clean
	if !models.IsStatusSkipped(msg.result.Status) || models.GetStatusReason(msg.result.Status) != "Not selected" {
		m.batchResults = append(m.batchResults, msg.result)
	}

	// Add successful PRs to session history
	if models.IsStatusSuccess(msg.result.Status) && msg.result.PrURL != nil {
		m.recordSessionPR(msg.result.Repo.DisplayName, *msg.result.PrURL)
	}
	m.batchCurrent++

	// Process all repos, not just selected count
	if m.batchCurrent >= len(m.batchRepos) {
		m.batchCurrentRepo = ""
		m.batchCurrentStep = ""
		// Close progress channel
		if m.batchProgressChan != nil {
			close(m.batchProgressChan)
			m.batchProgressChan = nil
		}
		m.screen = ScreenBatchSummary
		m.menuIndex = 0
		// Spawn confetti if any successes
		for _, result := range m.batchResults {
			if models.IsStatusSuccess(result.Status) {
				m.spawnConfetti()
				break
			}
		}
		return m, nil
	}

	// Set next repo name, clear step, and start processing
	m.batchCurrentRepo = m.batchRepos[m.batchCurrent].DisplayName
	m.batchCurrentStep = ""
	return m, tea.Batch(
		startBatchProcessingCmd(&m, m.batchCurrent),
		listenForProgress(m.batchProgressChan),
	)
}

func (m Model) handleOpenPRsFetchedResult(msg openPRsFetchedResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMessage = msg.err.Error()
		m.screen = ScreenError
		return m, nil
	}

	m.screen = ScreenViewOpenPrs
	m.openPRs = msg.entries

	// Build merge PR list
	m.mergePRs = nil
	for _, entry := range m.openPRs {
		if entry.Status.DevToStaging != nil {
			m.mergePRs = append(m.mergePRs, models.MergePrEntry{
				Repo:     entry.Repo,
				PrNumber: entry.Status.DevToStaging.Number,
				PrTitle:  entry.Status.DevToStaging.Title,
				URL:      entry.Status.DevToStaging.URL,
				PrType:   models.DevToStaging,
			})
		}
		if entry.Status.StagingToMain != nil {
			m.mergePRs = append(m.mergePRs, models.MergePrEntry{
				Repo:     entry.Repo,
				PrNumber: entry.Status.StagingToMain.Number,
				PrTitle:  entry.Status.StagingToMain.Title,
				URL:      entry.Status.StagingToMain.URL,
				PrType:   models.StagingToMain,
			})
		}
	}

	m.mergeSelected = make([]bool, len(m.mergePRs))
	m.mergeColumn = 0
	m.mergeDevIndex = 0
	m.mergeMainIndex = 0

	return m, nil
}

func (m Model) handleMergeCompleteResult(msg mergeCompleteResult) (tea.Model, tea.Cmd) {
	m.mergeResults = append(m.mergeResults, msg.result)
	m.mergeCurrent++

	if m.mergeCurrent >= m.mergeTotal {
		m.screen = ScreenMergeSummary
		m.menuIndex = 0
		return m, nil
	}

	// Find next selected PR to merge
	for i := m.mergeCurrent; i < len(m.mergePRs); i++ {
		if i < len(m.mergeSelected) && m.mergeSelected[i] {
			return m, startMergingCmd(&m, i)
		}
		m.mergeCurrent++
	}

	// No more PRs to merge
	m.screen = ScreenMergeSummary
	m.menuIndex = 0
	return m, nil
}

// openURL opens a URL in the default browser
func openURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default: // Linux and others
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}

// isWSL checks if running under Windows Subsystem for Linux
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	version := strings.ToLower(string(data))
	return strings.Contains(version, "microsoft") || strings.Contains(version, "wsl")
}

// copyToClipboard copies text to the system clipboard
func copyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default: // Linux
		if isWSL() {
			// WSL: use clip.exe to reach Windows clipboard
			cmd = exec.Command("clip.exe")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// openURLs opens multiple URLs in the default browser
func openURLs(urls []string) {
	for _, url := range urls {
		_ = openURL(url)
	}
}

// Pull all repos messages and commands

type pullRepoResult struct {
	result models.PullResult
}

type pullReposLoadedResult struct {
	repos []models.RepoInfo
	err   error
}

// makePullResult creates a pullRepoResult with the given status
func makePullResult(repo models.RepoInfo, status models.PullStatus, commits int, errMsg string) pullRepoResult {
	return pullRepoResult{result: models.PullResult{
		Repo:        repo,
		Status:      status,
		CommitCount: commits,
		Error:       errMsg,
	}}
}

// loadPullReposCmd loads all repos for pull operation
func loadPullReposCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		repos, err := git.FindAttunedRepos(cfg.AttunedPath(), cfg.Paths.FrontendGlob, cfg.Paths.BackendGlob)
		if err != nil {
			return pullReposLoadedResult{err: err}
		}
		return pullReposLoadedResult{repos: repos}
	}
}

// pullNextRepoCmd pulls the next repo in the list
func pullNextRepoCmd(repo models.RepoInfo, branch string, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		// For "main", use the repo's actual main branch (could be "master")
		targetBranch := branch
		if branch == "main" {
			targetBranch = repo.MainBranch
		}

		if dryRun {
			time.Sleep(200 * time.Millisecond)
			// Simulate various results for dry run
			hash := 0
			for _, c := range repo.DisplayName {
				hash += int(c)
			}
			statuses := []models.PullStatus{
				models.PullUpdated,
				models.PullUpToDate,
				models.PullSkippedNoBranch,
			}
			status := statuses[hash%len(statuses)]
			commits := 0
			if status == models.PullUpdated {
				commits = (hash % 5) + 1
			}
			return makePullResult(repo, status, commits, "")
		}

		// Check if branch exists
		if !git.HasBranch(repo.Path, targetBranch) {
			return makePullResult(repo, models.PullSkippedNoBranch, 0, "")
		}

		// Check for dirty working tree
		dirty, err := git.IsDirty(repo.Path)
		if err != nil {
			return makePullResult(repo, models.PullFailed, 0, err.Error())
		}
		if dirty {
			return makePullResult(repo, models.PullSkippedDirty, 0, "")
		}

		// Fetch first to get remote changes
		if err := git.FetchBranches(repo.Path, []string{targetBranch}); err != nil {
			// If fetch fails due to branch not found, skip
			if _, ok := err.(*git.BranchNotFoundError); ok {
				return makePullResult(repo, models.PullSkippedNoBranch, 0, "")
			}
			return makePullResult(repo, models.PullFailed, 0, err.Error())
		}

		// Checkout and pull
		commits, err := git.CheckoutAndPull(repo.Path, targetBranch)
		if err != nil {
			return makePullResult(repo, models.PullFailed, 0, err.Error())
		}

		if commits == 0 {
			return makePullResult(repo, models.PullUpToDate, 0, "")
		}
		return makePullResult(repo, models.PullUpdated, commits, "")
	}
}

// handlePullReposLoaded handles the result of loading repos for pull
func (m Model) handlePullReposLoaded(msg pullReposLoadedResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMessage = msg.err.Error()
		m.screen = ScreenError
		return m, nil
	}

	m.pullRepos = msg.repos
	m.pullResults = nil
	m.pullCurrentIdx = 0
	m.screen = ScreenPullProgress

	if len(m.pullRepos) == 0 {
		m.screen = ScreenPullSummary
		return m, nil
	}

	// Start pulling first repo
	return m, pullNextRepoCmd(m.pullRepos[0], m.pullBranch, m.dryRun)
}

// handlePullRepoResult handles the result of pulling a single repo
func (m Model) handlePullRepoResult(msg pullRepoResult) (tea.Model, tea.Cmd) {
	m.pullResults = append(m.pullResults, msg.result)
	m.pullCurrentIdx++

	if m.pullCurrentIdx >= len(m.pullRepos) {
		m.screen = ScreenPullSummary
		return m, nil
	}

	// Pull next repo
	return m, pullNextRepoCmd(m.pullRepos[m.pullCurrentIdx], m.pullBranch, m.dryRun)
}

// GitHub Actions messages and commands

type actionsRunsFetchedResult struct {
	entries []actionsEntry
	err     error
}

type actionsRefreshTickMsg struct{}

type actionsJobsFetchedResult struct {
	runID uint64
	jobs  []models.WorkflowJob
	err   error
}

func fetchActionsRunsCmd(cfg *config.Config, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		if dryRun {
			time.Sleep(800 * time.Millisecond)
			now := time.Now()
			fakeRepos := []models.RepoInfo{
				{Path: "/home/user/repos/frontend/web", DisplayName: "frontend/web", MainBranch: "main"},
				{Path: "/home/user/repos/frontend/mobile", DisplayName: "frontend/mobile", MainBranch: "main"},
				{Path: "/home/user/repos/backend/api", DisplayName: "backend/api", MainBranch: "main"},
				{Path: "/home/user/repos/backend/workers", DisplayName: "backend/workers", MainBranch: "main"},
			}
			fakeEntries := []actionsEntry{
				{Repo: fakeRepos[0], Run: models.WorkflowRun{DatabaseID: 1001, DisplayTitle: "feat: Add dashboard", WorkflowName: "CI", Status: "in_progress", HeadBranch: "dev", Event: "push", URL: "https://github.com/example/web/actions/runs/1001", CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now.Add(-1 * time.Minute)}},
				{Repo: fakeRepos[0], Run: models.WorkflowRun{DatabaseID: 1000, DisplayTitle: "fix: Auth bug", WorkflowName: "CI", Status: "completed", Conclusion: "success", HeadBranch: "staging", Event: "push", URL: "https://github.com/example/web/actions/runs/1000", CreatedAt: now.Add(-30 * time.Minute), UpdatedAt: now.Add(-25 * time.Minute)}},
				{Repo: fakeRepos[1], Run: models.WorkflowRun{DatabaseID: 2001, DisplayTitle: "chore: Update deps", WorkflowName: "CI", Status: "completed", Conclusion: "failure", HeadBranch: "dev", Event: "push", URL: "https://github.com/example/mobile/actions/runs/2001", CreatedAt: now.Add(-10 * time.Minute), UpdatedAt: now.Add(-8 * time.Minute)}},
				{Repo: fakeRepos[2], Run: models.WorkflowRun{DatabaseID: 3001, DisplayTitle: "feat: Add endpoints", WorkflowName: "CI", Status: "in_progress", HeadBranch: "dev", Event: "push", URL: "https://github.com/example/api/actions/runs/3001", CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-30 * time.Second)}},
				{Repo: fakeRepos[2], Run: models.WorkflowRun{DatabaseID: 3002, DisplayTitle: "Deploy staging", WorkflowName: "Deploy", Status: "queued", HeadBranch: "staging", Event: "push", URL: "https://github.com/example/api/actions/runs/3002", CreatedAt: now.Add(-1 * time.Minute), UpdatedAt: now.Add(-1 * time.Minute)}},
				{Repo: fakeRepos[2], Run: models.WorkflowRun{DatabaseID: 3000, DisplayTitle: "fix: DB migration", WorkflowName: "CI", Status: "completed", Conclusion: "success", HeadBranch: "main", Event: "push", URL: "https://github.com/example/api/actions/runs/3000", CreatedAt: now.Add(-1 * time.Hour), UpdatedAt: now.Add(-55 * time.Minute)}},
				{Repo: fakeRepos[3], Run: models.WorkflowRun{DatabaseID: 4001, DisplayTitle: "refactor: Queue handler", WorkflowName: "CI", Status: "completed", Conclusion: "cancelled", HeadBranch: "dev", Event: "push", URL: "https://github.com/example/workers/actions/runs/4001", CreatedAt: now.Add(-15 * time.Minute), UpdatedAt: now.Add(-12 * time.Minute)}},
			}
			return actionsRunsFetchedResult{entries: fakeEntries}
		}

		repos, err := git.FindAttunedRepos(cfg.AttunedPath(), cfg.Paths.FrontendGlob, cfg.Paths.BackendGlob)
		if err != nil {
			return actionsRunsFetchedResult{err: err}
		}

		type repoResult struct {
			repo models.RepoInfo
			runs []models.WorkflowRun
		}

		var wg sync.WaitGroup
		results := make(chan repoResult, len(repos))

		for _, repo := range repos {
			wg.Add(1)
			go func(r models.RepoInfo) {
				defer wg.Done()
				runs, err := github.ListWorkflowRuns(r.Path, 10)
				if err != nil {
					results <- repoResult{repo: r}
					return
				}
				results <- repoResult{repo: r, runs: runs}
			}(repo)
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		cutoff := time.Now().Add(-48 * time.Hour)
		var entries []actionsEntry
		for res := range results {
			// Keep in-progress/queued runs + latest completed per workflow (within 48h)
			latestCompleted := map[string]bool{} // workflowName -> already added
			for _, run := range res.runs {
				if run.UpdatedAt.Before(cutoff) {
					continue
				}
				if run.Status == "in_progress" || run.Status == "queued" {
					entries = append(entries, actionsEntry{Repo: res.repo, Run: run})
				} else if run.Status == "completed" && !latestCompleted[run.WorkflowName] {
					entries = append(entries, actionsEntry{Repo: res.repo, Run: run})
					latestCompleted[run.WorkflowName] = true
				}
			}
		}

		// Sort: newest first (by UpdatedAt descending)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Run.UpdatedAt.After(entries[j].Run.UpdatedAt)
		})

		return actionsRunsFetchedResult{entries: entries}
	}
}

func actionsRefreshTickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
		return actionsRefreshTickMsg{}
	})
}

func fetchActionsJobsCmd(repoPath string, runID uint64, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		if dryRun {
			time.Sleep(500 * time.Millisecond)
			now := time.Now()
			fakeJobs := []models.WorkflowJob{
				{
					Name: "build", Status: "completed", Conclusion: "success",
					StartedAt: now.Add(-5 * time.Minute), CompletedAt: now.Add(-3 * time.Minute),
					URL: "https://github.com/example/repo/actions/runs/1001/job/1",
					Steps: []models.WorkflowStep{
						{Name: "Checkout", Number: 1, Status: "completed", Conclusion: "success"},
						{Name: "Setup Node", Number: 2, Status: "completed", Conclusion: "success"},
						{Name: "Install deps", Number: 3, Status: "completed", Conclusion: "success"},
						{Name: "Build", Number: 4, Status: "completed", Conclusion: "success"},
					},
				},
				{
					Name: "test", Status: "in_progress", Conclusion: "",
					StartedAt: now.Add(-2 * time.Minute),
					URL:        "https://github.com/example/repo/actions/runs/1001/job/2",
					Steps: []models.WorkflowStep{
						{Name: "Checkout", Number: 1, Status: "completed", Conclusion: "success"},
						{Name: "Setup Node", Number: 2, Status: "completed", Conclusion: "success"},
						{Name: "Run tests", Number: 3, Status: "in_progress", Conclusion: ""},
						{Name: "Upload coverage", Number: 4, Status: "queued", Conclusion: ""},
					},
				},
				{
					Name: "deploy", Status: "queued", Conclusion: "",
					URL: "https://github.com/example/repo/actions/runs/1001/job/3",
					Steps: []models.WorkflowStep{
						{Name: "Deploy to staging", Number: 1, Status: "queued", Conclusion: ""},
					},
				},
			}
			return actionsJobsFetchedResult{runID: runID, jobs: fakeJobs}
		}

		jobs, err := github.GetWorkflowRunJobs(repoPath, runID)
		if err != nil {
			return actionsJobsFetchedResult{runID: runID, err: err}
		}
		return actionsJobsFetchedResult{runID: runID, jobs: jobs}
	}
}

func (m Model) handleActionsRunsFetched(msg actionsRunsFetchedResult) (tea.Model, tea.Cmd) {
	m.actionsLoading = false
	if msg.err != nil {
		if m.screen == ScreenActionsOverview || m.screen == ScreenLoading {
			m.errorMessage = msg.err.Error()
			m.screen = ScreenError
		}
		return m, nil
	}
	m.actionsEntries = msg.entries
	m.actionsLastRefresh = time.Now()
	if m.screen == ScreenLoading {
		m.screen = ScreenActionsOverview
	}

	// Clamp index and scroll to new filtered list size
	filtered := m.getFilteredActions()
	if m.actionsIndex >= len(filtered) {
		m.actionsIndex = max(len(filtered)-1, 0)
	}
	m.adjustActionsRunScroll(filtered)

	// Update pinned panels with fresh run data and re-fetch jobs if status changed or still active
	var refreshCmds []tea.Cmd
	for i, panel := range m.actionsPinned {
		for _, entry := range msg.entries {
			if entry.Run.DatabaseID == panel.Run.DatabaseID {
				if entry.Run.Status != panel.Run.Status || entry.Run.Status == "in_progress" || entry.Run.Status == "queued" {
					refreshCmds = append(refreshCmds, fetchActionsJobsCmd(entry.Repo.Path, entry.Run.DatabaseID, m.dryRun))
				}
				m.actionsPinned[i].Run = entry.Run
				break
			}
		}
	}

	var cmds []tea.Cmd
	if m.screen == ScreenActionsOverview {
		cmds = append(cmds, actionsRefreshTickCmd())
	}
	cmds = append(cmds, refreshCmds...)
	return m, tea.Batch(cmds...)
}

func (m Model) handleActionsRefreshTick() (tea.Model, tea.Cmd) {
	if m.screen != ScreenActionsOverview {
		return m, nil // Stop tick chain
	}
	m.actionsLoading = true
	return m, fetchActionsRunsCmd(m.config, m.dryRun)
}

func (m Model) handleActionsJobsFetched(msg actionsJobsFetchedResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.unpinRun(msg.runID)
		return m, nil
	}
	for i, p := range m.actionsPinned {
		if p.Run.DatabaseID == msg.runID {
			m.actionsPinned[i].Jobs = msg.jobs
			break
		}
	}
	return m, nil
}
