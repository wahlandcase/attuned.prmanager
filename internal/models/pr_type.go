package models

import "fmt"

// PrType represents the type of pull request to create
type PrType int

const (
	// DevToStaging represents dev -> staging
	DevToStaging PrType = iota
	// StagingToMain represents staging -> main/master
	StagingToMain
)

// BaseBranch returns the base branch for this PR type
func (p PrType) BaseBranch(mainBranch string) string {
	switch p {
	case DevToStaging:
		return "staging"
	case StagingToMain:
		return mainBranch
	default:
		return ""
	}
}

// HeadBranch returns the head branch for this PR type
func (p PrType) HeadBranch() string {
	switch p {
	case DevToStaging:
		return "dev"
	case StagingToMain:
		return "staging"
	default:
		return ""
	}
}

// Display returns a display string for this PR type
func (p PrType) Display(mainBranch string) string {
	switch p {
	case DevToStaging:
		return "dev → staging"
	case StagingToMain:
		return fmt.Sprintf("staging → %s", mainBranch)
	default:
		return ""
	}
}

// DefaultTitle returns the default PR title
func (p PrType) DefaultTitle(mainBranch string) string {
	return p.Display(mainBranch)
}
