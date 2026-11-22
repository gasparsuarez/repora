package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gasparsuarez/repora/internal/domain"
)

func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}

func TestLoadCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.yaml")

	cfg, warnings, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v", warnings)
	}
	if cfg.Version != domain.CurrentConfigVersion {
		t.Errorf("version = %d", cfg.Version)
	}
	if len(cfg.Workspaces) != 0 || len(cfg.Repositories) != 0 {
		t.Errorf("default config is not empty: %+v", cfg)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

func TestLoadRefreshIntervalDefaultAndExplicitZero(t *testing.T) {
	dir := t.TempDir()

	withoutKey := writeConfig(t, dir, `version: 1
repositories: []
workspaces: []
`)
	cfg, _, err := Load(withoutKey)
	if err != nil {
		t.Fatalf("Load without key: %v", err)
	}
	if got := cfg.Preferences.RefreshIntervalSeconds(); got != domain.DefaultRefreshInterval {
		t.Errorf("interval = %d, want default %d", got, domain.DefaultRefreshInterval)
	}

	dir = t.TempDir()
	withZero := writeConfig(t, dir, `version: 1
repositories: []
workspaces: []
preferences:
  refresh_interval: 0
`)
	cfg, _, err = Load(withZero)
	if err != nil {
		t.Fatalf("Load with explicit zero: %v", err)
	}
	if got := cfg.Preferences.RefreshIntervalSeconds(); got != 0 {
		t.Errorf("interval = %d, want 0 to disable", got)
	}
}

func TestLoadValidAndExpandsHome(t *testing.T) {
	dir := t.TempDir()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	path := writeConfig(t, dir, `version: 1
repositories:
  - id: api
    name: API
    path: ~/code/api
workspaces:
  - id: w
    name: W
    repositories: [api]
`)

	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join(home, "code", "api")
	if cfg.Repositories[0].Path != want {
		t.Errorf("path = %q, want %q", cfg.Repositories[0].Path, want)
	}
}

func TestLoadRejectsRelativePath(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `version: 1
repositories:
  - id: api
    name: API
    path: code/api
workspaces: []
`)

	_, _, err := Load(path)
	if err == nil {
		t.Fatal("expected error for relative path")
	}
	if !strings.Contains(err.Error(), "must be absolute") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadWarnsAboutMissingPath(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, `version: 1
repositories:
  - id: ghost
    name: Ghost
    path: `+filepath.Join(dir, "missing")+`
workspaces: []
`)

	cfg, warnings, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repositories) != 1 {
		t.Fatalf("repositories = %d", len(cfg.Repositories))
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "does not exist") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestLoadRejectsUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "version: 2\nrepositories: []\nworkspaces: []\n")

	_, _, err := Load(path)
	if err == nil {
		t.Fatal("expected version error")
	}
	if !strings.Contains(err.Error(), "unsupported configuration version") {
		t.Errorf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("original file was removed: %v", statErr)
	}
}

func TestWriteFileAtomicKeepsOriginalOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A directory in place of the target makes the final rename fail.
	if err := os.Mkdir(path+".blocked", 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := WriteFileAtomic(path+".blocked", []byte("replacement"), 0o600); err == nil {
		t.Fatal("expected an error when replacing a non-empty directory")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "original" {
		t.Errorf("original was modified: %q", data)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".repora-") {
			t.Errorf("temporary file left behind: %s", entry.Name())
		}
	}
}

func TestWriteFileAtomicWritesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")

	if err := WriteFileAtomic(path, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "version: 1\n" {
		t.Errorf("content = %q", data)
	}
}

func TestResolvePathPrecedence(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "explicit.yaml")

	got, err := ResolvePath(explicit)
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if got != explicit {
		t.Errorf("got %q, want %q", got, explicit)
	}

	t.Setenv(EnvConfigPath, filepath.Join(dir, "env.yaml"))
	got, err = ResolvePath("")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if want := filepath.Join(dir, "env.yaml"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv(EnvConfigPath, "")
	got, err = ResolvePath("")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if want := filepath.Join(dir, "xdg", DefaultDirName, DefaultFileName); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
