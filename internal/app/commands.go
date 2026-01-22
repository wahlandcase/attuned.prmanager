package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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

		var status models.BatchStatus
		if updated {
			status = models.Updated
		} else {
			status = models.Created
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

func (m Model) handlePrCreatedResult(msg prCreatedResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errorMessage = msg.err.Error()
		m.screen = ScreenError
		return m, nil
	}

	m.prURL = msg.url
	m.screen = ScreenComplete
	m.spawnConfetti()
	return m, nil
}

func (m Model) handleBatchRepoResult(msg batchRepoResult) (tea.Model, tea.Cmd) {
	// Only add non-skipped "not selected" results to keep summary clean
	if !models.IsStatusSkipped(msg.result.Status) || models.GetStatusReason(msg.result.Status) != "Not selected" {
		m.batchResults = append(m.batchResults, msg.result)
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
