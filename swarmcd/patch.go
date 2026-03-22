package swarmcd

import (
	"fmt"
	"os"
	"path"

	"github.com/m-adawi/swarm-cd/util"
)

// PatchRequest contains optional fields for updating a stack's configuration.
type PatchRequest struct {
	RepoURL     *string `json:"repo_url"`
	RefType     *string `json:"ref_type"`
	RefValue    *string `json:"ref_value"`
	ComposeFile *string `json:"compose_file"`
}

// PatchResult contains the outcome of a PATCH operation.
type PatchResult struct {
	Status  *StackStatus
	Warning string
}

// PatchStack atomically updates the configuration for a named stack.
// Lock ordering: stateMu first, then repo.lock (safe because reconciliation
// acquires repo.lock then releases it before stateMu).
func PatchStack(name string, req PatchRequest) (*PatchResult, error) {
	// Validate ref_type/ref_value pairing
	if err := validateRefFields(req); err != nil {
		return nil, &ValidationError{Msg: err.Error()}
	}

	stateMu.Lock()
	defer stateMu.Unlock()

	status, ok := stackStatus[name]
	if !ok {
		return nil, &NotFoundError{Msg: fmt.Sprintf("stack '%s' not found", name)}
	}

	stackCfg := config.StackConfigs[name]
	if stackCfg == nil {
		return nil, fmt.Errorf("stack config not found for '%s'", name)
	}

	var warning string

	// Handle repo_url change
	if req.RepoURL != nil && *req.RepoURL != status.RepoURL {
		repoName := stackCfg.Repo
		repo := repos[repoName]
		if repo == nil {
			return nil, fmt.Errorf("repo '%s' not found for stack '%s'", repoName, name)
		}

		// Check if other stacks share this repo
		sharedStacks := findSharedRepoStacks(repoName, name)
		if len(sharedStacks) > 0 {
			warning = fmt.Sprintf("repo '%s' is also used by: %s", repoName, joinNames(sharedStacks))
		}

		// Clone-to-temp-then-swap under repo.lock
		repo.lock.Lock()
		err := cloneToTempThenSwap(repo, *req.RepoURL)
		repo.lock.Unlock()

		if err != nil {
			return nil, &ValidationError{Msg: fmt.Sprintf("failed to clone new URL: %s", err)}
		}

		// Update repo config
		repoCfg := config.RepoConfigs[repoName]
		if repoCfg != nil {
			repoCfg.Url = *req.RepoURL
		}
		repo.url = *req.RepoURL
		status.RepoURL = *req.RepoURL
	}

	// Handle ref_type/ref_value change
	if req.RefType != nil && req.RefValue != nil {
		refType := *req.RefType
		refValue := *req.RefValue

		status.RefType = refType
		status.RefValue = refValue

		// Update the swarmStack objects and config
		if refType == "branch" {
			stackCfg.Branch = refValue
			stackCfg.Tag = ""
			updateSwarmStackRef(name, refValue, "")
		} else if refType == "tag" {
			stackCfg.Tag = refValue
			stackCfg.Branch = ""
			updateSwarmStackRef(name, "", refValue)
		}
	}

	// Handle compose_file change
	if req.ComposeFile != nil {
		repoName := stackCfg.Repo
		repo := repos[repoName]
		if repo == nil {
			return nil, fmt.Errorf("repo '%s' not found for stack '%s'", repoName, name)
		}

		composePath := path.Join(repo.path, *req.ComposeFile)
		if _, err := os.Stat(composePath); os.IsNotExist(err) {
			return nil, &ValidationError{Msg: fmt.Sprintf("compose file '%s' does not exist in repo checkout", *req.ComposeFile)}
		}

		stackCfg.ComposeFile = *req.ComposeFile
		status.ComposeFile = *req.ComposeFile
		updateSwarmStackComposeFile(name, *req.ComposeFile)
	}

	// Persist config to disk
	if err := util.PersistConfigs(); err != nil {
		return nil, fmt.Errorf("failed to persist config: %w", err)
	}

	// Return a deep copy of the updated status
	cp := *status
	if status.LastChangeAt != nil {
		t := *status.LastChangeAt
		cp.LastChangeAt = &t
	}
	if status.LastDeployedAt != nil {
		t := *status.LastDeployedAt
		cp.LastDeployedAt = &t
	}

	return &PatchResult{
		Status:  &cp,
		Warning: warning,
	}, nil
}

// ValidationError indicates a 400-class input error.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

// NotFoundError indicates a 404-class lookup failure.
type NotFoundError struct {
	Msg string
}

func (e *NotFoundError) Error() string { return e.Msg }

func validateRefFields(req PatchRequest) error {
	// Both must be present or both must be absent
	if (req.RefType != nil) != (req.RefValue != nil) {
		return fmt.Errorf("ref_type and ref_value must be provided together")
	}
	if req.RefType != nil {
		rt := *req.RefType
		if rt != "branch" && rt != "tag" {
			return fmt.Errorf("ref_type must be 'branch' or 'tag', got '%s'", rt)
		}
		if *req.RefValue == "" {
			return fmt.Errorf("ref_value cannot be empty")
		}
	}
	return nil
}

// findSharedRepoStacks returns the names of other stacks that share the given repo,
// excluding the named stack. Must be called with stateMu held.
func findSharedRepoStacks(repoName, excludeStack string) []string {
	var shared []string
	for stackName, stackCfg := range config.StackConfigs {
		if stackName != excludeStack && stackCfg.Repo == repoName {
			shared = append(shared, stackName)
		}
	}
	return shared
}

func joinNames(names []string) string {
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}

// cloneToTempThenSwap clones the new URL into a temp directory, and on success
// swaps it in place of the old repo. Must be called with repo.lock held.
func cloneToTempThenSwap(repo *stackRepo, newURL string) error {
	tmpPath := repo.path + ".tmp"

	// Clean up any leftover temp dir
	_ = os.RemoveAll(tmpPath)

	// Clone new URL to temp path
	_, err := newStackRepo(repo.name, tmpPath, newURL, repo.auth)
	if err != nil {
		// Clean up failed clone
		_ = os.RemoveAll(tmpPath)
		return err
	}

	// Remove old repo and rename temp to final
	if err := os.RemoveAll(repo.path); err != nil {
		return fmt.Errorf("could not remove old repo: %w", err)
	}
	if err := os.Rename(tmpPath, repo.path); err != nil {
		return fmt.Errorf("could not rename temp repo: %w", err)
	}

	// Re-open the repo at the original path
	newRepo, err := newStackRepo(repo.name, repo.path, newURL, repo.auth)
	if err != nil {
		return fmt.Errorf("could not reopen repo at original path: %w", err)
	}

	repo.gitRepoObject = newRepo.gitRepoObject
	return nil
}

// updateSwarmStackRef updates the branch/tag fields on the swarmStack object.
// Must be called with stateMu held.
func updateSwarmStackRef(name string, branch string, tag string) {
	for _, s := range stacks {
		if s.name == name {
			s.branch = branch
			s.tag = tag
			return
		}
	}
}

// updateSwarmStackComposeFile updates the composePath field on the swarmStack object.
// Must be called with stateMu held.
func updateSwarmStackComposeFile(name string, composePath string) {
	for _, s := range stacks {
		if s.name == name {
			s.composePath = composePath
			return
		}
	}
}
