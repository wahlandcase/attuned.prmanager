package models

import "strings"

// RepoInfo contains information about a git repository
type RepoInfo struct {
	// Path to the repository
	Path string
	// DisplayName (e.g., "frontend/web-app")
	DisplayName string
	// MainBranch name ("main" or "master")
	MainBranch string
	// ParentRepo name if this is a nested repo (e.g., "attuned-services")
	ParentRepo *string
}

// IsFrontend returns true if this repo belongs to the frontend category
func (r RepoInfo) IsFrontend() bool {
	return strings.Contains(r.DisplayName, "frontend/") || strings.HasPrefix(r.DisplayName, "frontend")
}

// InColumn returns true if this repo belongs to the given column (0=frontend, 1=backend/other)
func (r RepoInfo) InColumn(column int) bool {
	if column == 0 {
		return r.IsFrontend()
	}
	return !r.IsFrontend()
}

// NewRepoInfo creates a new RepoInfo
func NewRepoInfo(path, displayName, mainBranch string) RepoInfo {
	return RepoInfo{
		Path:        path,
		DisplayName: displayName,
		MainBranch:  mainBranch,
		ParentRepo:  nil,
	}
}

// WithParent sets the parent repo and returns the RepoInfo
func (r RepoInfo) WithParent(parent string) RepoInfo {
	r.ParentRepo = &parent
	return r
}

// ShortName returns just the last segment of DisplayName (after the last "/")
func (r RepoInfo) ShortName() string {
	if idx := strings.LastIndex(r.DisplayName, "/"); idx != -1 {
		return r.DisplayName[idx+1:]
	}
	return r.DisplayName
}
