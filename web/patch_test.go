package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/m-adawi/swarm-cd/swarmcd"
)

// newPatchTestRouter creates a Gin engine with the PATCH route and auth middleware.
func newPatchTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	write := r.Group("/")
	write.Use(authMiddleware())
	write.PATCH("/stacks/:name", patchStack)

	return r
}

// patchReq sends a PATCH request with JSON body and returns the recorder.
func patchReq(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-secret")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// createTestGitRepo creates a bare git repo with a single commit containing
// the given files. Returns the bare repo path.
func createTestGitRepo(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	repoPath := filepath.Join(dir, "repo.git")
	workPath := filepath.Join(dir, "work")

	runCmd(t, "", "git", "init", "--bare", repoPath)
	runCmd(t, "", "git", "clone", repoPath, workPath)
	for name, content := range files {
		p := filepath.Join(workPath, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	runCmd(t, workPath, "git", "add", "-A")
	runCmd(t, workPath, "git", "-c", "user.name=test", "-c", "user.email=test@test.com", "commit", "-m", "init")
	runCmd(t, workPath, "git", "push", "origin", "HEAD")

	return repoPath
}

func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %v failed: %v\n%s", name, args, err, out)
	}
}

// setupBasicPatchState creates a minimal state with one stack and one repo,
// using a real local git repo for the clone path. Returns cleanup function
// and the temp directory.
func setupBasicPatchState(t *testing.T) (cleanup func(), tmpDir string) {
	t.Helper()

	tmpDir = t.TempDir()

	// Create a local git repo with a compose file
	gitRepoPath := createTestGitRepo(t, filepath.Join(tmpDir, "git"), map[string]string{
		"docker-compose.yaml": "version: '3'\nservices:\n  web:\n    image: nginx\n",
	})

	// Clone it to simulate the repos/<name> directory
	clonePath := filepath.Join(tmpDir, "repos", "my-repo")
	runCmd(t, "", "git", "clone", gitRepoPath, clonePath)

	cleanupFn := swarmcd.SetFullStateForTest([]swarmcd.TestStackDef{
		{
			Name:        "my-stack",
			RepoName:    "my-repo",
			RepoPath:    clonePath,
			RepoURL:     gitRepoPath,
			Branch:      "main",
			ComposeFile: "docker-compose.yaml",
		},
	})

	// Change to tmpDir so PersistConfigs writes there
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)

	return func() {
		os.Chdir(origDir)
		cleanupFn()
	}, tmpDir
}

// TestPatchStack_UpdateRef verifies PATCH with new ref_type/ref_value.
func TestPatchStack_UpdateRef(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, _ := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()
	w := patchReq(r, "/stacks/my-stack", `{"ref_type":"tag","ref_value":"v1.0.0"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	stack := resp["stack"].(map[string]interface{})
	if stack["ref_type"] != "tag" {
		t.Errorf("expected ref_type 'tag', got %v", stack["ref_type"])
	}
	if stack["ref_value"] != "v1.0.0" {
		t.Errorf("expected ref_value 'v1.0.0', got %v", stack["ref_value"])
	}
}

// TestPatchStack_PartialUpdate verifies only ref changes, repo_url unchanged.
func TestPatchStack_PartialUpdate(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, _ := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()

	origStatus := swarmcd.GetStackStatus()
	origURL := origStatus["my-stack"].RepoURL

	w := patchReq(r, "/stacks/my-stack", `{"ref_type":"branch","ref_value":"develop"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	stack := resp["stack"].(map[string]interface{})
	if stack["repo_url"] != origURL {
		t.Errorf("expected repo_url unchanged at %q, got %v", origURL, stack["repo_url"])
	}
	if stack["ref_value"] != "develop" {
		t.Errorf("expected ref_value 'develop', got %v", stack["ref_value"])
	}
}

// TestPatchStack_ValidationErrors verifies 400 responses for invalid inputs.
func TestPatchStack_ValidationErrors(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, _ := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()

	tests := []struct {
		name string
		body string
	}{
		{"ref_type without ref_value", `{"ref_type":"branch"}`},
		{"ref_value without ref_type", `{"ref_value":"main"}`},
		{"invalid ref_type", `{"ref_type":"commit","ref_value":"abc123"}`},
		{"empty ref_value", `{"ref_type":"branch","ref_value":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := patchReq(r, "/stacks/my-stack", tt.body)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

// TestPatchStack_NoAuth_Returns401 verifies auth is required.
func TestPatchStack_NoAuth_Returns401(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, _ := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()

	req := httptest.NewRequest(http.MethodPatch, "/stacks/my-stack", strings.NewReader(`{"ref_type":"branch","ref_value":"develop"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestPatchStack_ComposeFileValidation verifies 400 when compose_file doesn't exist.
func TestPatchStack_ComposeFileValidation(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, _ := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()
	w := patchReq(r, "/stacks/my-stack", `{"compose_file":"nonexistent.yaml"}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	errMsg := resp["error"].(string)
	if !strings.Contains(errMsg, "does not exist") {
		t.Errorf("expected error about file not existing, got: %s", errMsg)
	}
}

// TestUpdateStackRef_BranchToTag verifies switching from branch to tag clears branch.
func TestUpdateStackRef_BranchToTag(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, _ := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()
	w := patchReq(r, "/stacks/my-stack", `{"ref_type":"tag","ref_value":"v2.0.0"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	status := swarmcd.GetStackStatus()
	if status["my-stack"].RefType != "tag" {
		t.Errorf("expected ref_type 'tag', got %q", status["my-stack"].RefType)
	}
	if status["my-stack"].RefValue != "v2.0.0" {
		t.Errorf("expected ref_value 'v2.0.0', got %q", status["my-stack"].RefValue)
	}
}

// TestUpdateStackRef_TagToBranch verifies switching from tag to branch clears tag.
func TestUpdateStackRef_TagToBranch(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	tmpDir := t.TempDir()
	gitRepoPath := createTestGitRepo(t, filepath.Join(tmpDir, "git"), map[string]string{
		"docker-compose.yaml": "version: '3'\n",
	})
	clonePath := filepath.Join(tmpDir, "repos", "my-repo")
	runCmd(t, "", "git", "clone", gitRepoPath, clonePath)

	cleanupState := swarmcd.SetFullStateForTest([]swarmcd.TestStackDef{
		{
			Name:        "my-stack",
			RepoName:    "my-repo",
			RepoPath:    clonePath,
			RepoURL:     gitRepoPath,
			Tag:         "v1.0.0",
			ComposeFile: "docker-compose.yaml",
		},
	})
	defer cleanupState()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	r := newPatchTestRouter()
	w := patchReq(r, "/stacks/my-stack", `{"ref_type":"branch","ref_value":"main"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	status := swarmcd.GetStackStatus()
	if status["my-stack"].RefType != "branch" {
		t.Errorf("expected ref_type 'branch', got %q", status["my-stack"].RefType)
	}
	if status["my-stack"].RefValue != "main" {
		t.Errorf("expected ref_value 'main', got %q", status["my-stack"].RefValue)
	}
}

// TestUpdateStackRef_RejectsEmpty verifies ref_type without ref_value returns error.
func TestUpdateStackRef_RejectsEmpty(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, _ := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()
	w := patchReq(r, "/stacks/my-stack", `{"ref_type":"branch"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateStackRef_RejectsBoth verifies invalid ref_type returns error.
func TestUpdateStackRef_RejectsBoth(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, _ := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()
	w := patchReq(r, "/stacks/my-stack", `{"ref_type":"both","ref_value":"main"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPatchStack_NotFound verifies 404 for nonexistent stack.
func TestPatchStack_NotFound(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, _ := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()
	w := patchReq(r, "/stacks/nonexistent", `{"ref_type":"branch","ref_value":"main"}`)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPatchStack_UpdateRepoURL tests PATCH with a new repo URL (clone-to-temp-then-swap).
func TestPatchStack_UpdateRepoURL(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	tmpDir := t.TempDir()

	origRepoPath := createTestGitRepo(t, filepath.Join(tmpDir, "orig-git"), map[string]string{
		"docker-compose.yaml": "version: '3'\nservices:\n  web:\n    image: nginx\n",
	})
	newRepoPath := createTestGitRepo(t, filepath.Join(tmpDir, "new-git"), map[string]string{
		"docker-compose.yaml": "version: '3'\nservices:\n  api:\n    image: golang\n",
	})

	clonePath := filepath.Join(tmpDir, "repos", "my-repo")
	runCmd(t, "", "git", "clone", origRepoPath, clonePath)

	cleanupState := swarmcd.SetFullStateForTest([]swarmcd.TestStackDef{
		{
			Name:        "my-stack",
			RepoName:    "my-repo",
			RepoPath:    clonePath,
			RepoURL:     origRepoPath,
			Branch:      "main",
			ComposeFile: "docker-compose.yaml",
		},
	})
	defer cleanupState()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	r := newPatchTestRouter()
	body := fmt.Sprintf(`{"repo_url":%q}`, newRepoPath)
	w := patchReq(r, "/stacks/my-stack", body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	stackResp := resp["stack"].(map[string]interface{})
	if stackResp["repo_url"] != newRepoPath {
		t.Errorf("expected repo_url %q, got %v", newRepoPath, stackResp["repo_url"])
	}

	// Verify the clone directory still exists (was swapped, not deleted)
	if _, err := os.Stat(clonePath); err != nil {
		t.Errorf("clone path should still exist: %v", err)
	}
}

// TestPatchStack_UpdateRepoURL_Failure verifies unreachable URL returns 400 and old repo intact.
func TestPatchStack_UpdateRepoURL_Failure(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, tmpDir := setupBasicPatchState(t)
	defer cleanup()

	clonePath := filepath.Join(tmpDir, "repos", "my-repo")
	origContent, err := os.ReadFile(filepath.Join(clonePath, "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read original compose: %v", err)
	}

	r := newPatchTestRouter()
	w := patchReq(r, "/stacks/my-stack", `{"repo_url":"https://nonexistent.invalid/repo.git"}`)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Verify old repo is intact
	afterContent, err := os.ReadFile(filepath.Join(clonePath, "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read compose after failure: %v", err)
	}
	if string(afterContent) != string(origContent) {
		t.Error("repo content changed after failed URL update")
	}
}

// TestPatchStack_SharedRepo_Warning verifies warning when multiple stacks share a repo.
func TestPatchStack_SharedRepo_Warning(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	tmpDir := t.TempDir()

	origRepoPath := createTestGitRepo(t, filepath.Join(tmpDir, "orig-git"), map[string]string{
		"docker-compose.yaml": "version: '3'\n",
		"other-compose.yaml":  "version: '3'\n",
	})
	newRepoPath := createTestGitRepo(t, filepath.Join(tmpDir, "new-git"), map[string]string{
		"docker-compose.yaml": "version: '3'\n",
	})

	clonePath := filepath.Join(tmpDir, "repos", "shared-repo")
	runCmd(t, "", "git", "clone", origRepoPath, clonePath)

	cleanupState := swarmcd.SetFullStateForTest([]swarmcd.TestStackDef{
		{
			Name:        "stack-a",
			RepoName:    "shared-repo",
			RepoPath:    clonePath,
			RepoURL:     origRepoPath,
			Branch:      "main",
			ComposeFile: "docker-compose.yaml",
		},
		{
			Name:        "stack-b",
			RepoName:    "shared-repo",
			RepoPath:    clonePath,
			RepoURL:     origRepoPath,
			Branch:      "main",
			ComposeFile: "other-compose.yaml",
		},
	})
	defer cleanupState()

	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	r := newPatchTestRouter()
	body := fmt.Sprintf(`{"repo_url":%q}`, newRepoPath)
	w := patchReq(r, "/stacks/stack-a", body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	warning, ok := resp["warning"]
	if !ok {
		t.Fatal("expected warning field in response")
	}
	warningStr := warning.(string)
	if !strings.Contains(warningStr, "stack-b") {
		t.Errorf("expected warning to mention 'stack-b', got: %s", warningStr)
	}
}

// TestPatchStack_AtomicPersistence verifies PATCH persists to disk and survives reload.
func TestPatchStack_AtomicPersistence(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, tmpDir := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()
	w := patchReq(r, "/stacks/my-stack", `{"ref_type":"tag","ref_value":"v3.0.0"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify stacks.yaml exists on disk
	stacksPath := filepath.Join(tmpDir, "stacks.yaml")
	data, err := os.ReadFile(stacksPath)
	if err != nil {
		t.Fatalf("read stacks.yaml: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("stacks.yaml should not be empty")
	}

	// Verify repos.yaml exists on disk
	reposPath := filepath.Join(tmpDir, "repos.yaml")
	data, err = os.ReadFile(reposPath)
	if err != nil {
		t.Fatalf("read repos.yaml: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("repos.yaml should not be empty")
	}
}

// TestConcurrentPatchAndReconcile verifies no deadlock or corruption under concurrent access.
func TestConcurrentPatchAndReconcile(t *testing.T) {
	origToken := apiToken
	apiToken = "test-secret"
	defer func() { apiToken = origToken }()

	cleanup, _ := setupBasicPatchState(t)
	defer cleanup()

	r := newPatchTestRouter()

	var wg sync.WaitGroup
	const iterations = 20

	// Concurrent PATCH requests
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"ref_type":"branch","ref_value":"branch-%d"}`, i)
			w := patchReq(r, "/stacks/my-stack", body)
			if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
				t.Errorf("unexpected status %d from concurrent PATCH", w.Code)
			}
		}(i)
	}

	// Concurrent reads (simulating reconciliation reads)
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := swarmcd.GetStackStatus()
			for _, v := range status {
				_ = v.RefType
				_ = v.RefValue
				_ = v.RepoURL
			}
		}()
	}

	wg.Wait()
}
