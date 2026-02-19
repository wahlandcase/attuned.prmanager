package app

import (
	"math"
	"math/rand"
	"time"

	"github.com/wahlandcase/attuned.prmanager/internal/config"
	"github.com/wahlandcase/attuned.prmanager/internal/models"
	"github.com/wahlandcase/attuned.prmanager/internal/ui"
	"github.com/wahlandcase/attuned.prmanager/internal/update"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfettiParticle represents a single confetti particle
type ConfettiParticle struct {
	X, Y   float64
	VX, VY float64
	Char   rune
	Color  lipgloss.Color
}

// sessionPR holds info about a PR created during this session
type sessionPR struct {
	repoName  string
	url       string
	prType    string // "dev→staging" or "staging→main"
	createdAt time.Time
}

// Model is the main application state
type Model struct {
	// Configuration
	config     *config.Config
	dryRun     bool
	testUpdate bool

	// Navigation
	screen       Screen
	menuIndex    int
	shouldQuit   bool

	// Mode
	mode *AppMode

	// Single mode state
	repoInfo   *models.RepoInfo
	prType     *models.PrType
	commits    []models.CommitInfo
	tickets    []string
	prTitle    string
	prURL      string
	existingPR *models.GhPr // Non-nil if PR already exists (will update)

	// Batch mode state
	batchRepos            []models.RepoInfo
	batchRepoCommits      []*[]models.CommitInfo     // Commits per repo: nil=loading, empty=no commits, non-empty=has commits
	batchFetchCancel      func()                     // Cancel function for background fetch
	batchResultsChan      chan batchRepoCommitResult // Channel for background fetch results
	batchFetchPending     int                        // Number of repos still fetching
	batchSelected         []bool
	batchResults          []models.BatchResult
	batchCurrent          int
	batchCurrentRepo      string // Name of repo currently being processed
	batchTotal            int
	batchFilter           string
	batchColumn           int // 0=Frontend, 1=Backend
	batchFEIndex          int
	batchBEIndex          int
	batchExistingPRs      int                // Count of repos with existing PRs (will update)
	batchReposWithCommits int                // Count of repos that have commits to merge
	batchConfirmScroll    int                // Scroll offset for batch confirmation right column
	batchProgressChan     chan string        // Channel for real-time progress updates
	batchCurrentStep      string             // Current step being executed (e.g., "Fetching branches...")

	// Open PRs / Merge state
	openPRs       []OpenPREntry
	mergePRs      []models.MergePrEntry
	mergeSelected  []bool
	mergeColumn    int // 0=dev->staging, 1=staging->main
	mergeDevIndex  int
	mergeMainIndex int
	mergeResults   []models.MergeResult
	mergeCurrent   int
	mergeTotal     int

	// UI state
	confirmSelection int // 0=Yes, 1=No
	errorMessage     string
	loadingMessage   string
	spinnerFrame     int
	copyFeedback     string // Brief "Copied!" message, clears on next action
	authError        error  // Non-nil if gh auth check failed

	// Update state
	version               string          // Current app version
	updateAvailable       *update.Release // Non-nil if update available
	updateSelection       int             // 0=Update now, 1=Skip, 2=Skip this version
	updateCheckInProgress bool            // True while checking for updates (manual)

	// Animation state
	confetti      []ConfettiParticle
	pulsePhase    float64 // 0.0 - 2*PI for sine wave
	typewriterPos int     // Characters revealed so far

	// Session history (survives reset)
	sessionPRs   []sessionPR
	historyIndex int

	// Pull all state
	pullBranch     string // "dev", "staging", or "main"
	pullRepos      []models.RepoInfo
	pullResults    []models.PullResult
	pullCurrentIdx int

	// GitHub Actions state
	actionsEntries     []actionsEntry
	actionsIndex       int // flat index into filtered entries
	actionsLoading     bool
	actionsLastRefresh time.Time
	actionsFilter       string
	actionsFilterActive bool

	// Actions pinned runs (shown in right panel)
	actionsPinned       []actionsPanel
	actionsColumn       int // 0=left (runs), 1=right (pinned)
	actionsRunScroll    int // scroll offset in lines for left run list
	actionsPinnedIndex  int // focused pinned panel index
	actionsPinnedScroll int // scroll offset in lines for right panel

	// Window size
	width  int
	height int
}

// OpenPREntry holds repo info with its PR status
type OpenPREntry struct {
	Repo   models.RepoInfo
	Status models.RepoPrStatus
}

// actionsEntry holds a single workflow run with its repo
type actionsEntry struct {
	Repo models.RepoInfo
	Run  models.WorkflowRun
}

// actionsPanel holds a pinned workflow run with its fetched job details
type actionsPanel struct {
	Run  models.WorkflowRun
	Repo models.RepoInfo
	Jobs []models.WorkflowJob // nil = loading
}

func (m *Model) isPinned(runID uint64) bool {
	for _, p := range m.actionsPinned {
		if p.Run.DatabaseID == runID {
			return true
		}
	}
	return false
}

func (m *Model) unpinRun(runID uint64) bool {
	for i, p := range m.actionsPinned {
		if p.Run.DatabaseID == runID {
			m.actionsPinned = append(m.actionsPinned[:i], m.actionsPinned[i+1:]...)
			if m.actionsPinnedIndex >= len(m.actionsPinned) && m.actionsPinnedIndex > 0 {
				m.actionsPinnedIndex--
			}
			return true
		}
	}
	return false
}

func (m *Model) adjustActionsPinnedScroll() {
	if len(m.actionsPinned) == 0 {
		m.actionsPinnedScroll = 0
		return
	}
	// Compute line offsets for each panel: border(2) + title(1) + info(1) + jobs + expanded steps
	panelStart := 0
	focusStart := 0
	focusEnd := 0
	for i, p := range m.actionsPinned {
		lines := 2 + 1 + 1 // border top/bottom + title + info line
		if p.Jobs == nil {
			lines++ // "Loading jobs..." line
		} else {
			lines += len(p.Jobs)
			for _, j := range p.Jobs {
				if j.Conclusion == "failure" || j.Status == "in_progress" {
					for _, s := range j.Steps {
						if s.Conclusion == "failure" || s.Status == "in_progress" {
							lines++
						}
					}
				}
			}
		}
		if i == m.actionsPinnedIndex {
			focusStart = panelStart
			focusEnd = panelStart + lines
		}
		panelStart += lines
	}

	visibleLines := m.actionsVisibleLines()
	// Edge-only: scroll down if focused panel's bottom is below viewport
	if focusEnd > m.actionsPinnedScroll+visibleLines {
		m.actionsPinnedScroll = focusEnd - visibleLines
	}
	// Scroll up if focused panel's top is above viewport
	if focusStart < m.actionsPinnedScroll {
		m.actionsPinnedScroll = focusStart
	}
	if m.actionsPinnedScroll < 0 {
		m.actionsPinnedScroll = 0
	}
}

func (m *Model) adjustActionsRunScroll(filtered []int) {
	// Keep highlight visible: 1 line reserved for ColumnBox title
	visibleLines := m.actionsVisibleLines()
	highlightLine := m.actionsRunListHighlightLine(filtered)
	if highlightLine < m.actionsRunScroll {
		m.actionsRunScroll = highlightLine
	} else if highlightLine >= m.actionsRunScroll+visibleLines {
		m.actionsRunScroll = highlightLine - visibleLines + 1
	}
	if m.actionsRunScroll < 0 {
		m.actionsRunScroll = 0
	}
}

func (m *Model) actionsVisibleLines() int {
	bannerLines := 5
	if m.dryRun {
		bannerLines += 2
	}
	panelHeight := m.height - bannerLines - 3 - 3 - 6 // banner, gaps, status, title+filter
	if panelHeight < 5 {
		panelHeight = 5
	}
	return panelHeight - 1 // -1 for ColumnBox title
}

// copyWithFeedback copies text to the clipboard and sets copyFeedback
func (m *Model) copyWithFeedback(text, successMsg string) {
	if err := copyToClipboard(text); err == nil {
		m.copyFeedback = "✓ " + successMsg
	} else {
		m.copyFeedback = "✗ Copy failed"
	}
}

// mainBranch returns the main branch name for the current repo, defaulting to "main"
func (m Model) mainBranch() string {
	if m.repoInfo != nil {
		return m.repoInfo.MainBranch
	}
	return "main"
}

// New creates a new application model
func New(cfg *config.Config, dryRun, testUpdate bool, version string) Model {
	return Model{
		config:     cfg,
		dryRun:     dryRun,
		testUpdate: testUpdate,
		version:    version,
		screen:     ScreenMainMenu,
		menuIndex:  0,
		width:      80,
		height:     24,
		sessionPRs: loadHistory(),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.EnterAltScreen,
		tickCmd(),
	}
	if !m.dryRun {
		cmds = append(cmds, authCheckCmd())
		// Check for updates if enabled and 24h since last check
		if m.config.ShouldCheckForUpdate() {
			cmds = append(cmds, checkUpdateCmd(m.version, m.config.Update.Repo))
		}
	}
	// Test update flag shows fake update prompt
	if m.testUpdate {
		cmds = append(cmds, func() tea.Msg {
			return updateCheckResult{release: &update.Release{TagName: "v99.0.0"}}
		})
	}
	return tea.Batch(cmds...)
}

// tickMsg is sent on each tick for animations
type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

// spawnConfetti creates confetti particles for celebration
func (m *Model) spawnConfetti() {
	colors := []lipgloss.Color{
		ui.ColorCyan,
		ui.ColorMagenta,
		ui.ColorYellow,
		ui.ColorGreen,
		ui.ColorRed,
		ui.ColorWhite,
	}
	chars := []rune{'*', '•', '✦', '✧', '◆', '◇', '▪', '♦', '★', '☆'}

	m.confetti = nil
	for i := 0; i < 40; i++ {
		angle := (float64(i) / 40.0) * math.Pi * 2.0
		speed := 2.0 + float64(i%5)*0.5
		m.confetti = append(m.confetti, ConfettiParticle{
			X:     40.0, // center-ish
			Y:     5.0,
			VX:    math.Cos(angle) * speed,
			VY:    math.Sin(angle)*speed - 2.0, // bias upward initially
			Char:  chars[rand.Intn(len(chars))],
			Color: colors[rand.Intn(len(colors))],
		})
	}
	m.typewriterPos = 0
}

// batchConfirmContentLines calculates total content lines for the right column
func (m *Model) batchConfirmContentLines() int {
	totalLines := 0
	for i := range m.batchRepos {
		if i < len(m.batchSelected) && m.batchSelected[i] {
			if i < len(m.batchRepoCommits) && m.batchRepoCommits[i] != nil {
				commits := *m.batchRepoCommits[i]
				if len(commits) > 0 {
					totalLines++ // repo name
					if len(commits) > 3 {
						totalLines += 4 // 3 commits + "more" line
					} else {
						totalLines += len(commits)
					}
					totalLines++ // blank line after repo
				}
			}
		}
	}
	// Tickets section
	if len(m.tickets) > 0 {
		totalLines++ // header
		totalLines += len(m.tickets)
	}
	return totalLines
}

// updateAnimations updates all animation state
func (m *Model) updateAnimations() {
	// Update pulse phase (smooth sine wave)
	m.pulsePhase = math.Mod(m.pulsePhase+0.08, 2.0*math.Pi)

	// Update confetti physics
	for i := range m.confetti {
		m.confetti[i].X += m.confetti[i].VX
		m.confetti[i].Y += m.confetti[i].VY
		m.confetti[i].VY += 0.15 // gravity
		m.confetti[i].VX *= 0.98 // air resistance
	}

	// Remove particles that fell off screen
	filtered := m.confetti[:0]
	for _, p := range m.confetti {
		if p.Y < 50.0 {
			filtered = append(filtered, p)
		}
	}
	m.confetti = filtered

	// Typewriter effect - reveal more characters on success screens
	if (m.screen == ScreenComplete || m.screen == ScreenBatchSummary) && m.typewriterPos < 100 {
		m.typewriterPos++
	}
}
