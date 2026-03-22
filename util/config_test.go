package util

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPersistConfigs_RoundTrip verifies that persisting configs and reloading
// them produces the same data.
func TestPersistConfigs_RoundTrip(t *testing.T) {
	// Work in a temp dir so we don't pollute the project root
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	// Save original configs and restore after test
	origConfigs := Configs
	defer func() { Configs = origConfigs }()

	Configs.RepoConfigs = map[string]*RepoConfig{
		"my-repo": {Url: "https://github.com/example/repo", Username: "user", Password: "pass"},
	}
	Configs.StackConfigs = map[string]*StackConfig{
		"my-stack": {Repo: "my-repo", Branch: "main", ComposeFile: "docker-compose.yaml"},
	}

	// Persist
	if err := PersistConfigs(); err != nil {
		t.Fatalf("PersistConfigs: %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(dir, "repos.yaml")); err != nil {
		t.Fatalf("repos.yaml not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stacks.yaml")); err != nil {
		t.Fatalf("stacks.yaml not found: %v", err)
	}

	// Modify the in-memory config to verify reload overwrites it
	Configs.RepoConfigs = nil
	Configs.StackConfigs = nil

	// Reload from split files
	if err := readRepoConfigs(); err != nil {
		t.Fatalf("readRepoConfigs: %v", err)
	}
	if err := readStackConfigs(); err != nil {
		t.Fatalf("readStackConfigs: %v", err)
	}

	// Verify round-trip
	if len(Configs.RepoConfigs) != 1 {
		t.Fatalf("expected 1 repo config, got %d", len(Configs.RepoConfigs))
	}
	rc := Configs.RepoConfigs["my-repo"]
	if rc == nil {
		t.Fatal("expected 'my-repo' in RepoConfigs")
	}
	if rc.Url != "https://github.com/example/repo" {
		t.Errorf("expected url %q, got %q", "https://github.com/example/repo", rc.Url)
	}
	if rc.Username != "user" {
		t.Errorf("expected username %q, got %q", "user", rc.Username)
	}
	if rc.Password != "pass" {
		t.Errorf("expected password %q, got %q", "pass", rc.Password)
	}

	if len(Configs.StackConfigs) != 1 {
		t.Fatalf("expected 1 stack config, got %d", len(Configs.StackConfigs))
	}
	sc := Configs.StackConfigs["my-stack"]
	if sc == nil {
		t.Fatal("expected 'my-stack' in StackConfigs")
	}
	if sc.Repo != "my-repo" {
		t.Errorf("expected repo %q, got %q", "my-repo", sc.Repo)
	}
	if sc.Branch != "main" {
		t.Errorf("expected branch %q, got %q", "main", sc.Branch)
	}
	if sc.ComposeFile != "docker-compose.yaml" {
		t.Errorf("expected compose_file %q, got %q", "docker-compose.yaml", sc.ComposeFile)
	}
}

// TestPersistConfigs_AtomicWrite verifies that persistence uses a temp file
// that is renamed (atomic write pattern).
func TestPersistConfigs_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	origConfigs := Configs
	defer func() { Configs = origConfigs }()

	Configs.RepoConfigs = map[string]*RepoConfig{
		"test-repo": {Url: "https://example.com"},
	}
	Configs.StackConfigs = map[string]*StackConfig{
		"test-stack": {Repo: "test-repo", Branch: "main"},
	}

	if err := PersistConfigs(); err != nil {
		t.Fatalf("PersistConfigs: %v", err)
	}

	// After successful persist, .tmp files should NOT exist (they were renamed)
	if _, err := os.Stat(filepath.Join(dir, "repos.yaml.tmp")); !os.IsNotExist(err) {
		t.Errorf("repos.yaml.tmp should not exist after successful persist")
	}
	if _, err := os.Stat(filepath.Join(dir, "stacks.yaml.tmp")); !os.IsNotExist(err) {
		t.Errorf("stacks.yaml.tmp should not exist after successful persist")
	}

	// Final files should exist
	if _, err := os.Stat(filepath.Join(dir, "repos.yaml")); err != nil {
		t.Errorf("repos.yaml should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stacks.yaml")); err != nil {
		t.Errorf("stacks.yaml should exist: %v", err)
	}
}

// TestPersistConfigs_InlineVsSplit verifies that PersistConfigs always writes to
// split files regardless of how config was originally loaded.
func TestPersistConfigs_InlineVsSplit(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	origConfigs := Configs
	defer func() { Configs = origConfigs }()

	// Simulate inline config (repos and stacks set directly in Config)
	Configs.RepoConfigs = map[string]*RepoConfig{
		"inline-repo": {Url: "https://inline.example.com"},
	}
	Configs.StackConfigs = map[string]*StackConfig{
		"inline-stack": {Repo: "inline-repo", Tag: "v1.0.0", ComposeFile: "compose.yml"},
	}

	if err := PersistConfigs(); err != nil {
		t.Fatalf("PersistConfigs: %v", err)
	}

	// Verify split files were created
	reposData, err := os.ReadFile(filepath.Join(dir, "repos.yaml"))
	if err != nil {
		t.Fatalf("read repos.yaml: %v", err)
	}
	if len(reposData) == 0 {
		t.Error("repos.yaml should not be empty")
	}

	stacksData, err := os.ReadFile(filepath.Join(dir, "stacks.yaml"))
	if err != nil {
		t.Fatalf("read stacks.yaml: %v", err)
	}
	if len(stacksData) == 0 {
		t.Error("stacks.yaml should not be empty")
	}

	// Reload and verify tag field persisted
	Configs.RepoConfigs = nil
	Configs.StackConfigs = nil

	if err := readRepoConfigs(); err != nil {
		t.Fatalf("readRepoConfigs: %v", err)
	}
	if err := readStackConfigs(); err != nil {
		t.Fatalf("readStackConfigs: %v", err)
	}

	sc := Configs.StackConfigs["inline-stack"]
	if sc == nil {
		t.Fatal("expected 'inline-stack' in StackConfigs after reload")
	}
	if sc.Tag != "v1.0.0" {
		t.Errorf("expected tag %q, got %q", "v1.0.0", sc.Tag)
	}
	if sc.ComposeFile != "compose.yml" {
		t.Errorf("expected compose_file %q, got %q", "compose.yml", sc.ComposeFile)
	}
}
