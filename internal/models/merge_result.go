package models

// MergeResult represents the result of merging a single PR
type MergeResult struct {
	// RepoName (e.g., "frontend/attuned-web")
	RepoName string
	// PrNumber is the PR number
	PrNumber uint64
	// PrTitle is the PR title
	PrTitle string
	// PrType (dev->staging or staging->main)
	PrType PrType
	// Success indicates whether merge succeeded
	Success bool
	// Error message if failed
	Error *string
	// URL is the PR URL
	URL string
}
