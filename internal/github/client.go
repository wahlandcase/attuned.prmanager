package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/wahlandcase/attuned.prmanager/internal/models"
)

// CheckAuth verifies gh CLI is authenticated
func CheckAuth() error {
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not authenticated with GitHub CLI. Run 'gh auth login' first")
	}
	return nil
}

// GetExistingPR gets an existing open PR for the given head -> base branch
func GetExistingPR(repoPath, headBranch, baseBranch string) (*models.GhPr, error) {
	cmd := exec.Command("gh", "pr", "list",
		"--head", headBranch,
		"--base", baseBranch,
		"--state", "open",
		"--json", "number,url,title,state",
	)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh pr list failed: %s", string(output))
	}

	var prs []models.GhPr
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse gh pr list output: %w", err)
	}

	if len(prs) == 0 {
		return nil, nil
	}

	return &prs[0], nil
}

// CreatePR creates a new pull request
func CreatePR(repoPath, headBranch, baseBranch, title, body string) (*models.GhPr, error) {
	cmd := exec.Command("gh", "pr", "create",
		"--head", headBranch,
		"--base", baseBranch,
		"--title", title,
		"--body", body,
	)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh pr create failed: %s", string(output))
	}

	// gh pr create outputs the URL
	url := strings.TrimSpace(string(output))

	// Extract PR number from URL (e.g., https://github.com/org/repo/pull/123)
	parts := strings.Split(url, "/")
	var number uint64
	if len(parts) > 0 {
		number, _ = strconv.ParseUint(parts[len(parts)-1], 10, 64)
	}

	return &models.GhPr{
		Number: number,
		URL:    url,
		Title:  title,
		State:  "open",
	}, nil
}

// UpdatePR updates an existing PR's title and body
func UpdatePR(repoPath string, prNumber uint64, title, body string) (*models.GhPr, error) {
	cmd := exec.Command("gh", "pr", "edit",
		strconv.FormatUint(prNumber, 10),
		"--title", title,
		"--body", body,
	)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh pr edit failed: %s", string(output))
	}

	// Get the updated PR info
	return GetPR(repoPath, prNumber)
}

// GetPR gets PR details by number
func GetPR(repoPath string, prNumber uint64) (*models.GhPr, error) {
	cmd := exec.Command("gh", "pr", "view",
		strconv.FormatUint(prNumber, 10),
		"--json", "number,url,title,state",
	)
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr view failed: %w", err)
	}

	var pr models.GhPr
	if err := json.Unmarshal(output, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse gh pr view output: %w", err)
	}

	return &pr, nil
}

// GetOpenReleasePRs gets open release PRs for a repo (dev->staging and staging->main)
func GetOpenReleasePRs(repoPath, mainBranch string) (*models.RepoPrStatus, error) {
	devToStaging, err := GetExistingPR(repoPath, "dev", "staging")
	if err != nil {
		return nil, fmt.Errorf("checking dev->staging: %w", err)
	}

	stagingToMain, err := GetExistingPR(repoPath, "staging", mainBranch)
	if err != nil {
		return nil, fmt.Errorf("checking staging->%s: %w", mainBranch, err)
	}

	return &models.RepoPrStatus{
		DevToStaging:  devToStaging,
		StagingToMain: stagingToMain,
	}, nil
}

// GeneratePRBody generates PR body with ticket links using Linear magic words
func GeneratePRBody(tickets []string, linearOrg string) string {
	if len(tickets) == 0 {
		return ""
	}

	var lines []string
	for _, t := range tickets {
		line := fmt.Sprintf("### - Closes [%s](https://linear.app/%s/issue/%s)", t, linearOrg, strings.ToLower(t))
		lines = append(lines, line)
	}

	return fmt.Sprintf("# Tickets\n\n%s", strings.Join(lines, "\n"))
}

// MergePR merges a PR using regular merge (not squash)
func MergePR(repoPath string, prNumber uint64) error {
	cmd := exec.Command("gh", "pr", "merge",
		strconv.FormatUint(prNumber, 10),
		"--merge",
		"--delete-branch=false",
	)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr merge failed: %s", string(output))
	}

	return nil
}

// CreateOrUpdatePR creates a new PR or updates an existing one
func CreateOrUpdatePR(repoPath, headBranch, baseBranch, title string, tickets []string, linearOrg string) (*models.GhPr, bool, error) {
	body := GeneratePRBody(tickets, linearOrg)

	// Check for existing PR
	existing, err := GetExistingPR(repoPath, headBranch, baseBranch)
	if err != nil {
		return nil, false, err
	}

	if existing != nil {
		// Update existing PR
		pr, err := UpdatePR(repoPath, existing.Number, title, body)
		if err != nil {
			return nil, false, err
		}
		return pr, true, nil // true = updated
	}

	// Create new PR
	pr, err := CreatePR(repoPath, headBranch, baseBranch, title, body)
	if err != nil {
		return nil, false, err
	}
	return pr, false, nil // false = created
}
