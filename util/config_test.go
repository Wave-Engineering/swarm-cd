package util

import (
	"os"
	"path/filepath"
	"strings"
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

// ---------- Config conflict detection tests ----------

// TestConfigConflict_InlineStacksNoFile verifies that when stacks are
// defined inline in config.yaml and no stacks.yaml exists on disk,
// the conflict is detected and the "won't persist" warning is emitted.
func TestConfigConflict_InlineStacksNoFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	origConfigs := Configs
	origConflict := Conflict
	defer func() {
		Configs = origConfigs
		Conflict = origConflict
	}()

	// Write a config.yaml with inline stacks (but no inline repos)
	cfgYAML := `
stacks:
  my-stack:
    repo: my-repo
    branch: main
`
	if err := os.WriteFile("config.yaml", []byte(cfgYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	// Provide repos.yaml so LoadConfigs can load repos from split file
	if err := os.WriteFile("repos.yaml", []byte("my-repo:\n  url: https://example.com\n"), 0644); err != nil {
		t.Fatalf("write repos.yaml: %v", err)
	}
	// No stacks.yaml on disk

	Configs = Config{} // reset
	if err := LoadConfigs(); err != nil {
		t.Fatalf("LoadConfigs: %v", err)
	}

	if !Conflict.StacksInline {
		t.Error("expected StacksInline=true")
	}
	if Conflict.StacksFileExists {
		t.Error("expected StacksFileExists=false")
	}

	warnings := ConfigWarnings()
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "API changes to stacks won't persist") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'won't persist' warning for stacks, got %v", warnings)
	}
}

// TestConfigConflict_InlineStacksWithFile verifies that when stacks are
// defined inline in config.yaml AND stacks.yaml also exists on disk,
// the "ignored" warning is emitted.
func TestConfigConflict_InlineStacksWithFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	origConfigs := Configs
	origConflict := Conflict
	defer func() {
		Configs = origConfigs
		Conflict = origConflict
	}()

	cfgYAML := `
stacks:
  my-stack:
    repo: my-repo
    branch: main
`
	if err := os.WriteFile("config.yaml", []byte(cfgYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	// Also create stacks.yaml on disk
	if err := os.WriteFile("stacks.yaml", []byte("other-stack:\n  repo: other\n"), 0644); err != nil {
		t.Fatalf("write stacks.yaml: %v", err)
	}
	// Provide repos.yaml so LoadConfigs can load repos from split file
	if err := os.WriteFile("repos.yaml", []byte("my-repo:\n  url: https://example.com\n"), 0644); err != nil {
		t.Fatalf("write repos.yaml: %v", err)
	}

	Configs = Config{} // reset
	if err := LoadConfigs(); err != nil {
		t.Fatalf("LoadConfigs: %v", err)
	}

	if !Conflict.StacksInline {
		t.Error("expected StacksInline=true")
	}
	if !Conflict.StacksFileExists {
		t.Error("expected StacksFileExists=true")
	}

	warnings := ConfigWarnings()
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "stacks.yaml is ignored") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'stacks.yaml is ignored' warning, got %v", warnings)
	}
}

// TestConfigConflict_SplitOnly verifies that when stacks are only in
// stacks.yaml (not inline), no conflict is detected.
func TestConfigConflict_SplitOnly(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	origConfigs := Configs
	origConflict := Conflict
	defer func() {
		Configs = origConfigs
		Conflict = origConflict
	}()

	// config.yaml without inline stacks or repos
	cfgYAML := `
update_interval: 60
`
	if err := os.WriteFile("config.yaml", []byte(cfgYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	// stacks.yaml with stack definitions
	stacksYAML := `
my-stack:
  repo: my-repo
  branch: main
`
	if err := os.WriteFile("stacks.yaml", []byte(stacksYAML), 0644); err != nil {
		t.Fatalf("write stacks.yaml: %v", err)
	}
	// repos.yaml with repo definitions
	reposYAML := `
my-repo:
  url: https://example.com/repo
`
	if err := os.WriteFile("repos.yaml", []byte(reposYAML), 0644); err != nil {
		t.Fatalf("write repos.yaml: %v", err)
	}

	Configs = Config{} // reset
	if err := LoadConfigs(); err != nil {
		t.Fatalf("LoadConfigs: %v", err)
	}

	if Conflict.StacksInline {
		t.Error("expected StacksInline=false")
	}
	if Conflict.ReposInline {
		t.Error("expected ReposInline=false")
	}

	warnings := ConfigWarnings()
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for split-only config, got %v", warnings)
	}
}

// ---------- Repos conflict detection tests (same matrix) ----------

// TestConfigConflict_InlineReposNoFile verifies that when repos are
// defined inline in config.yaml and no repos.yaml exists on disk,
// the conflict is detected and the "won't persist" warning is emitted.
func TestConfigConflict_InlineReposNoFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	origConfigs := Configs
	origConflict := Conflict
	defer func() {
		Configs = origConfigs
		Conflict = origConflict
	}()

	cfgYAML := `
repos:
  my-repo:
    url: https://example.com/repo
`
	if err := os.WriteFile("config.yaml", []byte(cfgYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	// Provide stacks.yaml so LoadConfigs can load stacks from split file
	if err := os.WriteFile("stacks.yaml", []byte("my-stack:\n  repo: my-repo\n  branch: main\n"), 0644); err != nil {
		t.Fatalf("write stacks.yaml: %v", err)
	}
	// No repos.yaml on disk

	Configs = Config{} // reset
	if err := LoadConfigs(); err != nil {
		t.Fatalf("LoadConfigs: %v", err)
	}

	if !Conflict.ReposInline {
		t.Error("expected ReposInline=true")
	}
	if Conflict.ReposFileExists {
		t.Error("expected ReposFileExists=false")
	}

	warnings := ConfigWarnings()
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "API changes to repos won't persist") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'won't persist' warning for repos, got %v", warnings)
	}
}

// TestConfigConflict_InlineReposWithFile verifies that when repos are
// defined inline in config.yaml AND repos.yaml also exists on disk,
// the "ignored" warning is emitted.
func TestConfigConflict_InlineReposWithFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	origConfigs := Configs
	origConflict := Conflict
	defer func() {
		Configs = origConfigs
		Conflict = origConflict
	}()

	cfgYAML := `
repos:
  my-repo:
    url: https://example.com/repo
`
	if err := os.WriteFile("config.yaml", []byte(cfgYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	// Also create repos.yaml on disk
	if err := os.WriteFile("repos.yaml", []byte("other-repo:\n  url: https://other.com\n"), 0644); err != nil {
		t.Fatalf("write repos.yaml: %v", err)
	}
	// Provide stacks.yaml so LoadConfigs can load stacks from split file
	if err := os.WriteFile("stacks.yaml", []byte("my-stack:\n  repo: my-repo\n  branch: main\n"), 0644); err != nil {
		t.Fatalf("write stacks.yaml: %v", err)
	}

	Configs = Config{} // reset
	if err := LoadConfigs(); err != nil {
		t.Fatalf("LoadConfigs: %v", err)
	}

	if !Conflict.ReposInline {
		t.Error("expected ReposInline=true")
	}
	if !Conflict.ReposFileExists {
		t.Error("expected ReposFileExists=true")
	}

	warnings := ConfigWarnings()
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "repos.yaml is ignored") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'repos.yaml is ignored' warning, got %v", warnings)
	}
}

// TestConfigConflict_ReposSplitOnly verifies that when repos are only in
// repos.yaml (not inline), no repo conflict is detected.
func TestConfigConflict_ReposSplitOnly(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origDir)

	origConfigs := Configs
	origConflict := Conflict
	defer func() {
		Configs = origConfigs
		Conflict = origConflict
	}()

	// config.yaml without inline repos
	cfgYAML := `
update_interval: 60
`
	if err := os.WriteFile("config.yaml", []byte(cfgYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	reposYAML := `
my-repo:
  url: https://example.com/repo
`
	if err := os.WriteFile("repos.yaml", []byte(reposYAML), 0644); err != nil {
		t.Fatalf("write repos.yaml: %v", err)
	}
	// Provide stacks.yaml so LoadConfigs can load stacks from split file
	if err := os.WriteFile("stacks.yaml", []byte("my-stack:\n  repo: my-repo\n  branch: main\n"), 0644); err != nil {
		t.Fatalf("write stacks.yaml: %v", err)
	}

	Configs = Config{} // reset
	if err := LoadConfigs(); err != nil {
		t.Fatalf("LoadConfigs: %v", err)
	}

	if Conflict.ReposInline {
		t.Error("expected ReposInline=false")
	}

	warnings := ConfigWarnings()
	for _, w := range warnings {
		if strings.Contains(w, "repos") {
			t.Errorf("expected no repo warnings for split-only config, got %q", w)
		}
	}
}

// ---------- resolve_image ----------

// loadConfigWith runs LoadConfigs in a temp dir holding the given config.yaml and minimal
// split repos.yaml and stacks.yaml, and restores the package state afterwards.
func loadConfigWith(t *testing.T, cfgYAML string) error {
	t.Helper()
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	origConfigs := Configs
	origConflict := Conflict
	t.Cleanup(func() {
		Configs = origConfigs
		Conflict = origConflict
	})

	files := map[string]string{
		"config.yaml": cfgYAML,
		"repos.yaml":  "my-repo:\n  url: https://example.com/repo\n",
		"stacks.yaml": "my-stack:\n  repo: my-repo\n  branch: main\n",
	}
	for name, body := range files {
		if err := os.WriteFile(name, []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	Configs = Config{}
	return LoadConfigs()
}

// TestResolveImageDefault verifies that a config.yaml without resolve_image keeps
// docker's own default, so existing deployments behave exactly as before.
func TestResolveImageDefault(t *testing.T) {
	if err := loadConfigWith(t, "update_interval: 60\n"); err != nil {
		t.Fatalf("LoadConfigs: %v", err)
	}
	if Configs.ResolveImage != "always" {
		t.Errorf("expected ResolveImage %q, got %q", "always", Configs.ResolveImage)
	}
}

// TestResolveImageChanged verifies that resolve_image is read from config.yaml.
func TestResolveImageChanged(t *testing.T) {
	if err := loadConfigWith(t, "update_interval: 60\nresolve_image: changed\n"); err != nil {
		t.Fatalf("LoadConfigs: %v", err)
	}
	if Configs.ResolveImage != "changed" {
		t.Errorf("expected ResolveImage %q, got %q", "changed", Configs.ResolveImage)
	}
}

// TestResolveImageInvalid verifies that an unknown resolve_image fails at startup with
// an error naming the key and the allowed values, never a silent fallback.
func TestResolveImageInvalid(t *testing.T) {
	for _, bad := range []string{"sometimes", `""`, "Changed"} {
		err := loadConfigWith(t, "resolve_image: "+bad+"\n")
		if err == nil {
			t.Errorf("resolve_image: %s: expected an error, got none", bad)
			continue
		}
		for _, want := range []string{"resolve_image", "always, changed, never"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("resolve_image: %s: error %q does not mention %q", bad, err, want)
			}
		}
	}
}
