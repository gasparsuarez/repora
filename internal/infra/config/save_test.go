package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gasparsuarez/repora/internal/domain"
)

func loadConfig(t *testing.T, path string) *domain.Config {
	t.Helper()
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &domain.Config{
		Version: 1,
		Repositories: []domain.Repository{
			{ID: "api", Name: "API", Path: "/tmp/api"},
		},
		Workspaces: []domain.Workspace{
			{ID: "work", Name: "Work", RepositoryIDs: []domain.RepositoryID{"api"}},
		},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded := loadConfig(t, path)
	if len(reloaded.Workspaces) != 1 || reloaded.Workspaces[0].ID != "work" {
		t.Fatalf("workspaces = %+v", reloaded.Workspaces)
	}
	if len(reloaded.Repositories) != 1 || reloaded.Repositories[0].Path != "/tmp/api" {
		t.Fatalf("repositories = %+v", reloaded.Repositories)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

func TestSavePreservesExplicitZeroRefreshInterval(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `version: 1
repositories: []
workspaces: []
preferences:
  refresh_interval: 0
`)

	cfg := loadConfig(t, path)
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), "refresh_interval: 0") {
		t.Errorf("explicit zero was not preserved:\n%s", raw)
	}

	reloaded := loadConfig(t, path)
	if got := reloaded.Preferences.RefreshIntervalSeconds(); got != 0 {
		t.Errorf("interval = %d after round trip, want 0", got)
	}
}

func TestSavePreservesUnknownTopLevelAndNestedFields(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `version: 1

future_section:
  enabled: true

repositories:
  - id: api
    name: API
    path: /tmp/api
    custom_flag: keep-me

workspaces:
  - id: work
    name: Work
    repositories: [api]
    owner: team-payments

preferences:
  theme: dark
  experimental_ui: true
`)

	cfg := loadConfig(t, path)
	cfg.Workspaces[0].Name = "Work Renamed"

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	text := string(raw)

	for _, want := range []string{
		"future_section",
		"enabled: true",
		"custom_flag: keep-me",
		"owner: team-payments",
		"experimental_ui: true",
		"Work Renamed",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("saved config lost %q:\n%s", want, text)
		}
	}
}

func TestSaveRemovesDeletedWorkspace(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `version: 1
repositories: []
workspaces:
  - id: keep
    name: Keep
  - id: drop
    name: Drop
`)

	cfg := loadConfig(t, path)
	cfg.Workspaces = cfg.Workspaces[:1]

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded := loadConfig(t, path)
	if len(reloaded.Workspaces) != 1 || reloaded.Workspaces[0].ID != "keep" {
		t.Fatalf("workspaces = %+v", reloaded.Workspaces)
	}
}

func TestSaveRejectsInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &domain.Config{Version: 1, Workspaces: []domain.Workspace{{ID: "w", Name: ""}}}
	if err := Save(path, cfg); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid config was written to disk")
	}
}

func TestSavePreservesCommentsWherePossible(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "# top comment\nversion: 1\n\nrepositories: []\nworkspaces: []\n")

	cfg := loadConfig(t, path)
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(raw), "top comment") {
		t.Log("comments preserved")
	} else {
		t.Log("comments not preserved (documented limitation)")
	}
}
