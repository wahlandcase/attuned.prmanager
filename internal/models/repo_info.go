package models

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
