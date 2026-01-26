package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const historyMaxAge = 24 * time.Hour

// historyEntry is the persisted form of sessionPR
type historyEntry struct {
	RepoName  string    `json:"repo_name"`
	URL       string    `json:"url"`
	PrType    string    `json:"pr_type"`
	CreatedAt time.Time `json:"created_at"`
}

func historyPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "attpr-history.json"), nil
}

// loadHistory loads and prunes old entries from the history file
func loadHistory() []sessionPR {
	path, err := historyPath()
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entries []historyEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}

	// Filter to entries within 24h
	cutoff := time.Now().Add(-historyMaxAge)
	var valid []historyEntry
	for _, e := range entries {
		if e.CreatedAt.After(cutoff) {
			valid = append(valid, e)
		}
	}

	// Rewrite file if we pruned anything
	if len(valid) != len(entries) {
		saveHistoryEntries(valid)
	}

	// Convert to sessionPR
	var result []sessionPR
	for _, e := range valid {
		result = append(result, sessionPR{
			repoName:  e.RepoName,
			url:       e.URL,
			prType:    e.PrType,
			createdAt: e.CreatedAt,
		})
	}
	return result
}

// saveHistory saves the current session PRs to disk
func saveHistory(prs []sessionPR) {
	var entries []historyEntry
	for _, pr := range prs {
		entries = append(entries, historyEntry{
			RepoName:  pr.repoName,
			URL:       pr.url,
			PrType:    pr.prType,
			CreatedAt: pr.createdAt,
		})
	}
	saveHistoryEntries(entries)
}

func saveHistoryEntries(entries []historyEntry) {
	path, err := historyPath()
	if err != nil {
		return
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(path, data, 0644)
}
