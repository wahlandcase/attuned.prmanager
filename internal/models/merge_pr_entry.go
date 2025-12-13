package models

// MergePrEntry represents an entry for a PR in the merge selection list
type MergePrEntry struct {
	// Repo is the repository info
	Repo RepoInfo
	// PrNumber is the PR number
	PrNumber uint64
	// PrTitle is the PR title
	PrTitle string
	// URL is the PR URL
	URL string
	// PrType is the PR type
	PrType PrType
}
