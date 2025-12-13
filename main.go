package main

import (
	"fmt"
	"os"

	"attuned-release/internal/app"
	"attuned-release/internal/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var dryRun bool

func main() {
	rootCmd := &cobra.Command{
		Use:   "attuned-release",
		Short: "TUI for managing GitHub release PRs",
		RunE:  run,
	}

	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate operations without making changes")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	model := app.New(cfg, dryRun)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	return nil
}
