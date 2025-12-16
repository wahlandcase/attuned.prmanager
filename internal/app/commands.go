package app

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"attuned-release/internal/config"
	"attuned-release/internal/git"
	"attuned-release/internal/github"
	"attuned-release/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

// Message types for async operations

type fetchCommitsResult struct {
	commits []models.CommitInfo
	tickets []string
	err     error
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

// Commands

func fetchCommitsCmd(repo *models.RepoInfo, prType *models.PrType, dryRun bool) tea.Cmd {
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
		commits, err := git.GetCommitsBetween(repo.Path, baseBranch, headBranch)
		if err != nil {
			return fetchCommitsResult{err: err}
		}

		// Extract all unique tickets
		tickets := git.GetAllTickets(commits)

		return fetchCommitsResult{commits: commits, tickets: tickets}
	}
}

func createPRCmd(repo *models.RepoInfo, prType *models.PrType, title string, tickets []string, dryRun bool) tea.Cmd {
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
		pr, _, err := github.CreateOrUpdatePR(repo.Path, headBranch, baseBranch, title, tickets)
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
		repos, err := git.FindAttunedRepos(cfg.AttunedPath())
		if err != nil {
			return openPRsFetchedResult{err: err}
		}

		// Fetch open PRs in parallel
		type result struct {
			entry OpenPREntry
			hasAny bool
		}

		var wg sync.WaitGroup
		results := make(chan result, len(repos))

		for _, repo := range repos {
			wg.Add(1)
			go func(r models.RepoInfo) {
				defer wg.Done()

				status := github.GetOpenReleasePRs(r.Path, r.MainBranch)
				hasAny := status.DevToStaging != nil || status.StagingToMain != nil

				results <- result{
					entry: OpenPREntry{Repo: r, Status: *status},
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
		var entries []OpenPREntry
		for res := range results {
			if res.hasAny {
				entries = append(entries, res.entry)
			}
		}

		return openPRsFetchedResult{entries: entries}
	}
}

func startBatchProcessingCmd(m *Model, repoIndex int) tea.Cmd {
	return func() tea.Msg {
		if repoIndex >= len(m.batchRepos) {
			return nil
		}

		repo := m.batchRepos[repoIndex]
		if repoIndex >= len(m.batchSelected) || !m.batchSelected[repoIndex] {
			// Skip unselected repos
			return batchRepoResult{result: models.BatchResult{
				Repo:   repo,
				Status: models.Skipped("Not selected"),
			}}
		}

		if m.dryRun {
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

		// Fetch and get commits
		if err := git.FetchBranches(repo.Path, []string{headBranch, baseBranch}); err != nil {
			return batchRepoResult{result: models.BatchResult{
				Repo:   repo,
				Status: models.Failed(err.Error()),
			}}
		}

		commits, err := git.GetCommitsBetween(repo.Path, baseBranch, headBranch)
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
		pr, updated, err := github.CreateOrUpdatePR(repo.Path, headBranch, baseBranch, m.prTitle, tickets)
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
	repos []models.RepoInfo
	err   error
}

type currentRepoLoadedResult struct {
	repo *models.RepoInfo
	err  error
}

// loadBatchReposCmd loads all repos for batch mode
func loadBatchReposCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		repos, err := git.FindAttunedRepos(cfg.AttunedPath())
		if err != nil {
			return batchReposLoadedResult{err: err}
		}
		return batchReposLoadedResult{repos: repos}
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
	m.batchSelected = make([]bool, len(msg.repos))
	m.screen = ScreenBatchRepoSelect
	m.batchColumn = 0
	m.batchFEIndex = 0
	m.batchBEIndex = 0
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
	m.screen = ScreenCommitReview
	m.menuIndex = 0
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

	// Start next batch repo processing
	return m, startBatchProcessingCmd(&m, m.batchCurrent)
}

func (m Model) handleOpenPRsFetchedResult(msg openPRsFetchedResult) (tea.Model, tea.Cmd) {
	m.openPRsLoading = false

	if msg.err != nil {
		m.errorMessage = msg.err.Error()
		m.screen = ScreenError
		return m, nil
	}

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
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
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

// copyURLs copies multiple URLs to the clipboard (one per line)
func copyURLs(urls []string) error {
	return copyToClipboard(strings.Join(urls, "\n"))
}
