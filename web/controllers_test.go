package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/m-adawi/swarm-cd/swarmcd"
)

// stubGetStackStatus is used to inject test data into getStacks.
// We swap swarmcd's internal state via its exported helpers for testing.

// setupTestStacks populates the swarmcd package-level state with test data
// and returns a cleanup function to restore the original state.
func setupTestStacks(t *testing.T, data map[string]*swarmcd.StackStatus) func() {
	t.Helper()
	swarmcd.SetStackStatusForTest(data)
	return func() {
		swarmcd.SetStackStatusForTest(map[string]*swarmcd.StackStatus{})
	}
}

func TestGetStacks_ReturnsAllFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Now().Truncate(time.Second)
	earlier := now.Add(-1 * time.Hour)

	cleanup := setupTestStacks(t, map[string]*swarmcd.StackStatus{
		"my-stack": {
			Error:          "some error",
			Revision:       "abc12345",
			RepoURL:        "https://github.com/example/repo",
			RefType:        "branch",
			RefValue:       "main",
			ComposeFile:    "docker-compose.yaml",
			LastChangeAt:   &earlier,
			LastDeployedAt: &now,
		},
	})
	defer cleanup()

	r := gin.New()
	r.GET("/stacks", getStacks)

	req := httptest.NewRequest(http.MethodGet, "/stacks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var stacks []stackResponse
	if err := json.Unmarshal(w.Body.Bytes(), &stacks); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(stacks))
	}

	s := stacks[0]

	if s.Name != "my-stack" {
		t.Errorf("expected name %q, got %q", "my-stack", s.Name)
	}
	if s.RepoURL != "https://github.com/example/repo" {
		t.Errorf("expected repo_url %q, got %q", "https://github.com/example/repo", s.RepoURL)
	}
	if s.RefType != "branch" {
		t.Errorf("expected ref_type %q, got %q", "branch", s.RefType)
	}
	if s.RefValue != "main" {
		t.Errorf("expected ref_value %q, got %q", "main", s.RefValue)
	}
	if s.Revision != "abc12345" {
		t.Errorf("expected revision %q, got %q", "abc12345", s.Revision)
	}
	if s.ComposeFile != "docker-compose.yaml" {
		t.Errorf("expected compose_file %q, got %q", "docker-compose.yaml", s.ComposeFile)
	}
	if s.Error != "some error" {
		t.Errorf("expected error %q, got %q", "some error", s.Error)
	}
	if s.LastChangeAt == nil {
		t.Fatal("expected last_change_at to be non-nil")
	}
	if !s.LastChangeAt.Equal(earlier) {
		t.Errorf("expected last_change_at %v, got %v", earlier, *s.LastChangeAt)
	}
	if s.LastDeployedAt == nil {
		t.Fatal("expected last_deployed_at to be non-nil")
	}
	if !s.LastDeployedAt.Equal(now) {
		t.Errorf("expected last_deployed_at %v, got %v", now, *s.LastDeployedAt)
	}

	// Verify JSON field names are snake_case by checking raw JSON
	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to decode raw response: %v", err)
	}
	expectedKeys := []string{"name", "repo_url", "ref_type", "ref_value", "revision", "compose_file", "error", "last_change_at", "last_deployed_at"}
	for _, key := range expectedKeys {
		if _, ok := raw[0][key]; !ok {
			t.Errorf("expected JSON key %q not found in response", key)
		}
	}
}

func TestGetStacks_SortOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cleanup := setupTestStacks(t, map[string]*swarmcd.StackStatus{
		"charlie": {
			RepoURL:  "https://github.com/example/repo",
			RefType:  "branch",
			RefValue: "main",
		},
		"alpha": {
			RepoURL:  "https://github.com/example/repo",
			RefType:  "tag",
			RefValue: "v1.0.0",
		},
		"bravo": {
			RepoURL:  "https://github.com/example/repo",
			RefType:  "branch",
			RefValue: "develop",
		},
	})
	defer cleanup()

	r := gin.New()
	r.GET("/stacks", getStacks)

	req := httptest.NewRequest(http.MethodGet, "/stacks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var stacks []stackResponse
	if err := json.Unmarshal(w.Body.Bytes(), &stacks); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(stacks) != 3 {
		t.Fatalf("expected 3 stacks, got %d", len(stacks))
	}

	expectedOrder := []string{"alpha", "bravo", "charlie"}
	for i, expected := range expectedOrder {
		if stacks[i].Name != expected {
			t.Errorf("position %d: expected %q, got %q", i, expected, stacks[i].Name)
		}
	}
}
