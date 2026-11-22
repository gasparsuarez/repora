package tui

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/goleak"

	"github.com/gasparsuarez/repora/internal/app"
	"github.com/gasparsuarez/repora/internal/domain"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// Timers scheduled by the notice auto-dismissal are runtime timers, not
		// goroutines, but ignore the Bubble Tea runtime if a test ever starts it.
		goleak.IgnoreTopFunction("github.com/charmbracelet/bubbletea.(*Program).eventLoop"),
	)
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07]*\x07`)

func stripANSI(s string) string {
	s = ansiPattern.ReplaceAllString(s, "")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}

func assertGolden(t *testing.T, name string, m *Model) {
	t.Helper()

	got := stripANSI(m.View())
	path := filepath.Join("testdata", "golden", name+".golden")

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v\nrun: go test ./internal/tui -update", path, err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch for %s\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
	}
}

func homeGoldenModel(t *testing.T, width, height int) *Model {
	t.Helper()

	cfg := &domain.Config{
		Version: 1,
		Repositories: []domain.Repository{
			{ID: "api", Name: "API", Path: "/tmp/api"},
			{ID: "web", Name: "Web", Path: "/tmp/web"},
			{ID: "infra", Name: "Infrastructure", Path: "/tmp/infra"},
			{ID: "shared", Name: "Shared", Path: "/tmp/shared"},
			{ID: "dotfiles", Name: "Dotfiles", Path: "/tmp/dotfiles"},
		},
		Workspaces: []domain.Workspace{
			{ID: "product", Name: "Product X", Description: "Customer-facing product",
				RepositoryIDs: []domain.RepositoryID{"api", "web", "infra", "shared"}},
			{ID: "personal", Name: "Personal", RepositoryIDs: []domain.RepositoryID{"dotfiles"}},
			{ID: "empty", Name: "Empty", Description: "Nothing assigned yet"},
		},
	}

	m := testNew(cfg, "/home/dev/.config/repora/config.yaml", nil)
	m.version = "dev"
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})

	m.homeLoaded = true
	m.homeStates = map[domain.RepositoryID]domain.RepositoryState{
		"api": {RepositoryID: "api", WorkingTree: domain.WorkingTree{Untracked: []string{"notes.md"}}},
		"web": {RepositoryID: "web", Branch: domain.BranchState{Head: "develop", Upstream: "origin/develop", HasUpstream: true}},
		"infra": {RepositoryID: "infra", Branch: domain.BranchState{
			Head: "main", Upstream: "origin/main", HasUpstream: true, Behind: 2,
		}},
		"shared": {RepositoryID: "shared", WorkingTree: domain.WorkingTree{
			Conflicts: []domain.FileChange{{Path: "go.mod", Index: domain.CodeUnmerged}},
		}},
		"dotfiles": {RepositoryID: "dotfiles"},
	}

	return m
}

func workspaceGoldenModel(t *testing.T, width, height int) *Model {
	t.Helper()

	m := homeGoldenModel(t, width, height)
	m.screen = screenWorkspace
	m.wsIndex = 0
	m.rows = []repoRow{
		{
			repo: domain.Repository{ID: "api", Name: "API", Path: "/tmp/api"},
			state: domain.RepositoryState{
				RepositoryID: "api",
				Branch:       domain.BranchState{Head: "feature/payment-flow", Upstream: "origin/feature/payment-flow", HasUpstream: true, Ahead: 1},
				WorkingTree: domain.WorkingTree{
					Staged:   []domain.FileChange{{Path: "handler.go", Index: domain.CodeModified}},
					Unstaged: []domain.FileChange{{Path: "README.md", Worktree: domain.CodeModified}},
				},
			},
		},
		{
			repo: domain.Repository{ID: "web", Name: "Web", Path: "/tmp/web"},
			state: domain.RepositoryState{
				RepositoryID: "web",
				Branch:       domain.BranchState{Head: "develop", Upstream: "origin/develop", HasUpstream: true},
			},
		},
		{
			repo: domain.Repository{ID: "infra", Name: "Infrastructure", Path: "/tmp/infra"},
			state: domain.RepositoryState{
				RepositoryID: "infra",
				Branch:       domain.BranchState{Head: "main", Upstream: "origin/main", HasUpstream: true, Behind: 2},
			},
		},
		{
			repo: domain.Repository{ID: "shared", Name: "Shared", Path: "/tmp/shared"},
			state: domain.RepositoryState{
				RepositoryID: "shared",
				Branch:       domain.BranchState{Head: "main"},
				WorkingTree:  domain.WorkingTree{Conflicts: []domain.FileChange{{Path: "go.mod", Index: domain.CodeUnmerged}}},
			},
		},
	}
	return m
}

func repositoryGoldenModel(t *testing.T, width, height int) *Model {
	t.Helper()

	m := workspaceGoldenModel(t, width, height)
	m.screen = screenRepository
	m.rowIndex = 0
	m.repoTab = tabOverview
	m.changes = buildChanges(m.rows[0].state)
	m.detail = app.RepositoryDetail{
		Branches: []domain.Branch{
			{Name: "feature/payment-flow", IsCurrent: true, HasUpstream: true, Upstream: "origin/feature/payment-flow", Ahead: 1},
			{Name: "develop", HasUpstream: true, Upstream: "origin/develop"},
			{Name: "main", HasUpstream: true, Upstream: "origin/main"},
			{Name: "origin/feature/payment-flow", IsRemote: true},
			{Name: "origin/develop", IsRemote: true},
		},
		Commits: []domain.Commit{
			{Hash: "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678", Short: "a1b2c3d", Subject: "Add payment retry logic", Author: "Ana Ruiz"},
			{Hash: "9f8e7d6c5b4a39281706f5e4d3c2b1a098765432", Short: "9f8e7d6", Subject: "Refactor settlement service", Author: "Luis M."},
		},
		Remotes: []domain.Remote{{Name: "origin", FetchURL: "git@github.com:acme/api.git"}},
	}
	return m
}

func TestGoldenHome(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"120x40", 120, 40},
	} {
		t.Run(size.name, func(t *testing.T) {
			assertGolden(t, "home_"+size.name, homeGoldenModel(t, size.width, size.height))
		})
	}
}

func TestGoldenWorkspace(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"120x40", 120, 40},
	} {
		t.Run(size.name, func(t *testing.T) {
			assertGolden(t, "workspace_"+size.name, workspaceGoldenModel(t, size.width, size.height))
		})
	}
}

func TestGoldenRepository(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"120x40", 120, 40},
	} {
		t.Run(size.name, func(t *testing.T) {
			assertGolden(t, "repository_"+size.name, repositoryGoldenModel(t, size.width, size.height))
		})
	}
}

func TestGoldenDeterministic(t *testing.T) {
	m := repositoryGoldenModel(t, 100, 32)
	first := stripANSI(m.View())
	second := stripANSI(m.View())
	if first != second {
		t.Fatal("rendering is not deterministic")
	}
	if regexp.MustCompile(`\b\d+(s|m|h|d|mo|y) ago`).MatchString(first) {
		t.Error("time-dependent text leaked into the golden fixture")
	}
}

func historyGoldenModel(t *testing.T, width, height int) *Model {
	t.Helper()

	m := repositoryGoldenModel(t, width, height)
	m.repoTab = tabHistory
	m.detail.Commits = []domain.Commit{
		{
			Hash: "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678", Short: "a1b2c3d",
			Subject: "Add payment retry logic with provider fallback",
			Author:  "Repora Sandbox",
			Refs:    []string{"HEAD -> main", "origin/main"},
		},
		{
			Hash: "9f8e7d6c5b4a39281706f5e4d3c2b1a098765432", Short: "9f8e7d6",
			Subject: "Refactor settlement service",
			Author:  "Luis M.",
		},
	}
	m.historyIndex = 0
	return m
}

func TestGoldenHistory(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"120x40", 120, 40},
	} {
		t.Run(size.name, func(t *testing.T) {
			assertGolden(t, "history_"+size.name, historyGoldenModel(t, size.width, size.height))
		})
	}
}

func promptGoldenModel(t *testing.T, width, height int) *Model {
	t.Helper()

	m := homeGoldenModel(t, width, height)
	m.prompt = newPrompt()
	m.prompt.kind = promptWorkspaceName
	m.prompt.input.Placeholder = "Product X"
	m.prompt.input.Width = max(20, width-12)
	m.prompt.input.SetValue("New Workspace")
	m.prompt.input.CursorEnd()
	return m
}

func pullRequestGoldenModel(t *testing.T, width, height int) *Model {
	t.Helper()

	m, _ := pullRequestModelAt(t, width, height)
	m.Update(key('o'))
	return m
}

func TestGoldenPullRequest(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"120x40", 120, 40},
	} {
		t.Run(size.name, func(t *testing.T) {
			assertGolden(t, "pull_request_"+size.name, pullRequestGoldenModel(t, size.width, size.height))
		})
	}
}

func TestGoldenPrompt(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"120x40", 120, 40},
	} {
		t.Run(size.name, func(t *testing.T) {
			assertGolden(t, "prompt_new_workspace_"+size.name, promptGoldenModel(t, size.width, size.height))
		})
	}
}
