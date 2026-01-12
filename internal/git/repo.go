package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wahlandcase/attuned.prmanager/internal/models"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// IsGitRepo checks if the path is a git repository
func IsGitRepo(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

// GetRepoInfo opens a repository and gets basic info
func GetRepoInfo(path, displayName string) (*models.RepoInfo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}

	mainBranch, err := DetectMainBranch(repo)
	if err != nil {
		return nil, err
	}

	info := models.NewRepoInfo(path, displayName, mainBranch)
	return &info, nil
}

// GetCurrentRepoInfo gets info for the current working directory
func GetCurrentRepoInfo() (*models.RepoInfo, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Walk up to find git root
	path := cwd
	for {
		if IsGitRepo(path) {
			break
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil, os.ErrNotExist
		}
		path = parent
	}

	// Use directory name as display name
	displayName := filepath.Base(path)
	return GetRepoInfo(path, displayName)
}

// DetectMainBranch determines if the repo uses "main" or "master"
func DetectMainBranch(repo *git.Repository) (string, error) {
	// Check remote refs first
	refs, err := repo.References()
	if err != nil {
		return "main", nil
	}

	hasRemoteMain := false
	hasRemoteMaster := false
	hasLocalMain := false
	hasLocalMaster := false

	refs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		if name == "refs/remotes/origin/main" {
			hasRemoteMain = true
		}
		if name == "refs/remotes/origin/master" {
			hasRemoteMaster = true
		}
		if name == "refs/heads/main" {
			hasLocalMain = true
		}
		if name == "refs/heads/master" {
			hasLocalMaster = true
		}
		return nil
	})

	// Prefer remote refs
	if hasRemoteMain {
		return "main", nil
	}
	if hasRemoteMaster {
		return "master", nil
	}

	// Fall back to local refs
	if hasLocalMain {
		return "main", nil
	}
	if hasLocalMaster {
		return "master", nil
	}

	// Default to main
	return "main", nil
}

// FetchBranches fetches specified branches from origin using git CLI (to inherit SSH agent)
func FetchBranches(repoPath string, branches []string) error {
	args := append([]string{"fetch", "origin"}, branches...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))
		if strings.Contains(outputStr, "couldn't find remote ref") {
			return &BranchNotFoundError{Branches: branches}
		}
		// Provide a more helpful error message
		if outputStr != "" {
			return &GitError{Command: "fetch", Output: outputStr}
		}
		return &GitError{Command: "fetch", Output: "Failed to fetch from remote (check network/auth)"}
	}

	return nil
}

// GitError provides better context for git command failures
type GitError struct {
	Command string
	Output  string
}

func (e *GitError) Error() string {
	return "git " + e.Command + ": " + e.Output
}

// BranchNotFoundError indicates a branch was not found on remote
type BranchNotFoundError struct {
	Branches []string
}

func (e *BranchNotFoundError) Error() string {
	return "Branch not found on remote: " + strings.Join(e.Branches, ", ")
}

// FindAttunedRepos finds all git repositories in the attuned directory structure
func FindAttunedRepos(basePath, frontendGlob, backendGlob string) ([]models.RepoInfo, error) {
	var repos []models.RepoInfo

	// Process each glob pattern with its category name
	globs := []struct {
		category string
		pattern  string
	}{
		{"frontend", frontendGlob},
		{"backend", backendGlob},
	}

	for _, g := range globs {
		pattern := filepath.Join(basePath, g.pattern)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid %s glob %q: %w", g.category, g.pattern, err)
		}

		for _, path := range matches {
			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				continue
			}

			repoName := filepath.Base(path)

			if IsGitRepo(path) {
				displayName := g.category + "/" + repoName

				// Check for nested git repos inside this repo (like attuned-services)
				nestedRepos := findNestedRepos(path, g.category, repoName)

				if len(nestedRepos) > 0 {
					// This is a parent repo with nested repos - add the nested ones
					repos = append(repos, nestedRepos...)
				} else {
					// Regular repo, add it directly
					if repoInfo, err := GetRepoInfo(path, displayName); err == nil {
						repos = append(repos, *repoInfo)
					}
				}
			}
		}
	}

	// Sort: group by category (frontend/backend), then nested repos at end of category, then by name
	sort.Slice(repos, func(i, j int) bool {
		a, b := repos[i], repos[j]

		// Extract category from display_name (first part)
		catA := strings.Split(a.DisplayName, "/")[0]
		catB := strings.Split(b.DisplayName, "/")[0]

		// First sort by category
		if catA != catB {
			return catA < catB
		}

		// Within same category: non-nested repos first, then nested repos grouped by parent
		if a.ParentRepo == nil && b.ParentRepo != nil {
			return true // non-nested before nested
		}
		if a.ParentRepo != nil && b.ParentRepo == nil {
			return false // nested after non-nested
		}
		if a.ParentRepo != nil && b.ParentRepo != nil {
			if *a.ParentRepo != *b.ParentRepo {
				return *a.ParentRepo < *b.ParentRepo
			}
		}

		return a.DisplayName < b.DisplayName
	})

	return repos, nil
}

// findNestedRepos finds nested git repos inside a parent repo (like attuned-services)
func findNestedRepos(parentPath, subdir, parentName string) []models.RepoInfo {
	var nested []models.RepoInfo

	entries, err := os.ReadDir(parentPath)
	if err != nil {
		return nested
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		repoName := entry.Name()
		// Skip hidden directories and common non-repo dirs
		if strings.HasPrefix(repoName, ".") || repoName == "node_modules" {
			continue
		}

		path := filepath.Join(parentPath, repoName)
		if IsGitRepo(path) {
			displayName := subdir + "/" + parentName + "/" + repoName

			if repoInfo, err := GetRepoInfo(path, displayName); err == nil {
				info := repoInfo.WithParent(parentName)
				nested = append(nested, info)
			}
		}
	}

	return nested
}

// HasBranch checks if a branch exists in the repository
func HasBranch(repoPath, branchName string) bool {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return false
	}

	// Check remote ref first
	_, err = repo.Reference(plumbing.NewRemoteReferenceName("origin", branchName), true)
	if err == nil {
		return true
	}

	// Check local ref
	_, err = repo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	return err == nil
}
