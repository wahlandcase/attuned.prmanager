package app

import (
	"math"
	"math/rand"
	"time"

	"attuned-release/internal/config"
	"attuned-release/internal/models"
	"attuned-release/internal/ui"

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

// Model is the main application state
type Model struct {
	// Configuration
	config *config.Config
	dryRun bool

	// Navigation
	screen       Screen
	menuIndex    int
	shouldQuit   bool

	// Mode
	mode *AppMode

	// Single mode state
	repoInfo *models.RepoInfo
	prType   *models.PrType
	commits  []models.CommitInfo
	tickets  []string
	prTitle  string
	prURL    string

	// Batch mode state
	batchRepos    []models.RepoInfo
	batchSelected []bool
	batchResults  []models.BatchResult
	batchCurrent  int
	batchTotal    int
	batchFilter   string
	batchColumn   int // 0=Frontend, 1=Backend
	batchFEIndex  int
	batchBEIndex  int

	// Open PRs / Merge state
	openPRs        []OpenPREntry
	openPRsLoading bool
	mergePRs       []models.MergePrEntry
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

	// Animation state
	confetti      []ConfettiParticle
	pulsePhase    float64 // 0.0 - 2*PI for sine wave
	typewriterPos int     // Characters revealed so far

	// Window size
	width  int
	height int
}

// OpenPREntry holds repo info with its PR status
type OpenPREntry struct {
	Repo   models.RepoInfo
	Status models.RepoPrStatus
}

// New creates a new application model
func New(cfg *config.Config, dryRun bool) Model {
	return Model{
		config:     cfg,
		dryRun:     dryRun,
		screen:     ScreenMainMenu,
		menuIndex:  0,
		width:      80,
		height:     24,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		tickCmd(),
	)
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
