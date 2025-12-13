package models

// GhPr represents GitHub PR info returned from gh CLI
type GhPr struct {
	Number uint64 `json:"number"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

// RepoPrStatus contains info about open PRs for a repo
type RepoPrStatus struct {
	DevToStaging  *GhPr
	StagingToMain *GhPr
}
