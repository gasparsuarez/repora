package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gasparsuarez/repora/internal/app"
	"github.com/gasparsuarez/repora/internal/domain"
)

type fakeConfigStore struct {
	cfg   *domain.Config
	err   error
	saves int
}

func (s *fakeConfigStore) Load() (*domain.Config, []string, error) { return s.cfg, nil, nil }

func (s *fakeConfigStore) Save(cfg *domain.Config) error {
	if s.err != nil {
		return s.err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	s.cfg = cfg.Clone()
	s.saves++
	return nil
}

type stubGit struct {
	root string
	err  error
}

func (g *stubGit) Status(context.Context, domain.Repository) domain.RepositoryState {
	return domain.RepositoryState{}
}
func (g *stubGit) Root(context.Context, string) (string, error) {
	if g.err != nil {
		return "", g.err
	}
	return g.root, nil
}
func (g *stubGit) Branches(context.Context, domain.Repository) ([]domain.Branch, error) {
	return nil, nil
}
func (g *stubGit) Log(context.Context, domain.Repository, int) ([]domain.Commit, error) {
	return nil, nil
}
func (g *stubGit) Remotes(context.Context, domain.Repository) ([]domain.Remote, error) {
	return nil, nil
}
func (g *stubGit) RefOID(context.Context, domain.Repository, string) (string, error) {
	return "0000000", nil
}
func (g *stubGit) Diff(context.Context, domain.Repository, domain.DiffTarget, string) (string, error) {
	return "", nil
}
func (g *stubGit) ShowCommit(context.Context, domain.Repository, string) (string, error) {
	return "", nil
}

func crudModel(t *testing.T, cfg *domain.Config) (*Model, *fakeConfigStore) {
	t.Helper()

	store := &fakeConfigStore{cfg: cfg}
	m := New(Deps{
		Config:     cfg,
		ConfigPath: "/tmp/config.yaml",
		NoColor:    true,
		Workspaces: app.NewWorkspaceService(store),
		Repos:      app.NewRepositoryService(&stubGit{root: "/tmp/code/api"}, store),
	})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.tick = nil // timers are exercised separately
	return m, store
}

func submitPrompt(t *testing.T, m *Model, text string) tea.Cmd {
	t.Helper()
	typeText(t, m, text)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return cmd
}

func TestCreateWorkspaceFromHome(t *testing.T) {
	m, store := crudModel(t, configWith("alpha"))

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.prompt.kind != promptWorkspaceName {
		t.Fatalf("prompt kind = %v", m.prompt.kind)
	}

	// Step 1: name advances to the optional description step.
	_ = submitPrompt(t, m, "Beta Project")
	if m.prompt.kind != promptWorkspaceDescription {
		t.Fatalf("prompt kind = %v, want description step", m.prompt.kind)
	}

	// Step 2: description and create.
	cmd := submitPrompt(t, m, "Team project")
	if cmd == nil {
		t.Fatal("no command returned")
	}
	msg := cmd()
	changed, ok := msg.(configChangedMsg)
	if !ok || changed.err != nil {
		t.Fatalf("unexpected message: %#v", msg)
	}

	_, _ = m.Update(changed)

	if len(m.cfg.Workspaces) != 2 {
		t.Fatalf("workspaces = %d, want 2", len(m.cfg.Workspaces))
	}
	if got := m.cfg.Workspaces[m.wsIndex].ID; got != "beta-project" {
		t.Errorf("selected = %q, want beta-project", got)
	}
	if store.saves != 1 {
		t.Errorf("saves = %d, want 1", store.saves)
	}
	if got := store.cfg.Workspaces[1].Description; got != "Team project" {
		t.Errorf("description = %q", got)
	}
	if !strings.Contains(m.notice, "create workspace") {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestCreateWorkspaceWithoutDescription(t *testing.T) {
	m, store := crudModel(t, configWith("alpha"))

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	_ = submitPrompt(t, m, "No Description")
	cmd := submitPrompt(t, m, "")

	msg := cmd()
	if changed, ok := msg.(configChangedMsg); !ok || changed.err != nil {
		t.Fatalf("unexpected message: %#v", msg)
	}
	_ = msg
	if got := store.cfg.Workspaces[1].Description; got != "" {
		t.Errorf("description = %q, want empty", got)
	}
}

func TestRenameWorkspaceUpdatesDescription(t *testing.T) {
	cfg := configWith("alpha")
	cfg.Workspaces[0].Description = "Old description"

	m, store := crudModel(t, cfg)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m.prompt.input.SetValue("Alpha Renamed")
	m.prompt.input.CursorEnd()
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.prompt.kind != promptWorkspaceRenameDescription {
		t.Fatalf("prompt kind = %v, want rename description step", m.prompt.kind)
	}
	if got := m.prompt.input.Value(); got != "Old description" {
		t.Errorf("description prefill = %q", got)
	}

	m.prompt.input.SetValue("New description")
	m.prompt.input.CursorEnd()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd()
	if changed, ok := msg.(configChangedMsg); !ok || changed.err != nil {
		t.Fatalf("unexpected message: %#v", msg)
	}
	if got := store.cfg.Workspaces[0].Name; got != "Alpha Renamed" {
		t.Errorf("name = %q", got)
	}
	if got := store.cfg.Workspaces[0].Description; got != "New description" {
		t.Errorf("description = %q", got)
	}
}

func TestCreateWorkspaceValidation(t *testing.T) {
	m, store := crudModel(t, configWith("alpha"))

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Error("empty name should not start a command")
	}
	if m.prompt.err == nil {
		t.Error("expected an inline validation error")
	}
	if store.saves != 0 {
		t.Errorf("saves = %d, want 0", store.saves)
	}
}

func TestRenameWorkspacePrefillsCurrentName(t *testing.T) {
	m, _ := crudModel(t, configWith("alpha", "beta"))

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.prompt.kind != promptWorkspaceRename {
		t.Fatalf("prompt kind = %v", m.prompt.kind)
	}
	if got := m.prompt.input.Value(); got != "ALPHA" {
		t.Errorf("prefill = %q, want ALPHA", got)
	}
	if m.prompt.target != "alpha" {
		t.Errorf("target = %q", m.prompt.target)
	}
}

func TestDeleteWorkspaceRequiresConfirmation(t *testing.T) {
	m, store := crudModel(t, configWith("alpha", "beta"))

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !m.confirm.active || m.confirm.action != confirmDeleteWorkspace {
		t.Fatalf("confirm = %+v", m.confirm)
	}
	if out := m.View(); !strings.Contains(out, "Delete workspace") {
		t.Errorf("confirm view missing title:\n%s", out)
	}

	// Enter cancels: the destructive choice is not the default.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || m.confirm.active {
		t.Fatal("Enter should cancel the confirmation")
	}
	if len(m.cfg.Workspaces) != 2 || store.saves != 0 {
		t.Fatal("cancelling must not change the configuration")
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("confirming produced no command")
	}
	msg := cmd()
	changed, ok := msg.(configChangedMsg)
	if !ok || changed.err != nil {
		t.Fatalf("unexpected message: %#v", msg)
	}
	_, _ = m.Update(changed)

	if len(m.cfg.Workspaces) != 1 || m.cfg.Workspaces[0].ID != "beta" {
		t.Fatalf("workspaces = %+v", m.cfg.Workspaces)
	}
}

func TestAddRepositoryToWorkspace(t *testing.T) {
	cfg := configWith("alpha")
	m, store := crudModel(t, cfg)
	m.screen = screenWorkspace
	m.rows = nil

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.prompt.kind != promptRepoPath {
		t.Fatalf("prompt kind = %v", m.prompt.kind)
	}

	cmd := submitPrompt(t, m, "/tmp/code/api")
	if cmd == nil {
		t.Fatal("no command returned")
	}
	msg := cmd()
	changed, ok := msg.(configChangedMsg)
	if !ok || changed.err != nil {
		t.Fatalf("unexpected message: %#v", msg)
	}
	_, _ = m.Update(changed)

	if len(store.cfg.Repositories) != 1 || store.cfg.Repositories[0].ID != "api" {
		t.Fatalf("repositories = %+v", store.cfg.Repositories)
	}
	if ids := store.cfg.Workspaces[0].RepositoryIDs; len(ids) != 1 || ids[0] != "api" {
		t.Fatalf("associations = %v", ids)
	}
	if !strings.Contains(m.notice, "add repository") {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestAddRepositoryReportsGitFailure(t *testing.T) {
	cfg := configWith("alpha")
	store := &fakeConfigStore{cfg: cfg}
	m := New(Deps{
		Config:     cfg,
		ConfigPath: "/tmp/config.yaml",
		NoColor:    true,
		Workspaces: app.NewWorkspaceService(store),
		Repos:      app.NewRepositoryService(&stubGit{err: domain.ErrNotARepository}, store),
	})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.screen = screenWorkspace

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	cmd := submitPrompt(t, m, "/tmp/not-a-repo")
	msg := cmd()
	changed := msg.(configChangedMsg)

	if changed.err == nil {
		t.Fatal("expected an error for a non-repository path")
	}
	_, _ = m.Update(changed)

	if !m.noticeError {
		t.Error("notice should be flagged as an error")
	}
	if !strings.Contains(m.errDetail, "not a git repository") {
		t.Errorf("errDetail = %q", m.errDetail)
	}
	if store.saves != 0 {
		t.Errorf("saves = %d, want 0", store.saves)
	}
}

func TestRemoveRepositoryFromWorkspace(t *testing.T) {
	cfg := configWith("alpha")
	cfg.Repositories = []domain.Repository{{ID: "api", Name: "API", Path: "/tmp/api"}}
	cfg.Workspaces[0].RepositoryIDs = []domain.RepositoryID{"api"}

	m, store := crudModel(t, cfg)
	m.screen = screenWorkspace
	m.rows = []repoRow{{repo: cfg.Repositories[0]}}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !m.confirm.active || m.confirm.action != confirmRemoveRepository {
		t.Fatalf("confirm = %+v", m.confirm)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("confirming produced no command")
	}
	msg := cmd()
	changed := msg.(configChangedMsg)
	if changed.err != nil {
		t.Fatalf("unexpected error: %v", changed.err)
	}
	_, _ = m.Update(changed)

	if ids := store.cfg.Workspaces[0].RepositoryIDs; len(ids) != 0 {
		t.Fatalf("associations = %v, want none", ids)
	}
	if len(store.cfg.Repositories) != 1 {
		t.Fatal("registration must survive disassociation")
	}
}

func TestConfigChangeSaveErrorKeepsState(t *testing.T) {
	m, store := crudModel(t, configWith("alpha"))
	store.err = errors.New("permission denied")

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	_ = submitPrompt(t, m, "Beta")
	cmd := submitPrompt(t, m, "")
	msg := cmd()
	changed := msg.(configChangedMsg)

	if changed.err == nil {
		t.Fatal("expected a save error")
	}
	_, _ = m.Update(changed)

	if len(m.cfg.Workspaces) != 1 {
		t.Fatalf("in-memory config changed after a failed save: %+v", m.cfg.Workspaces)
	}
	if !m.noticeError {
		t.Error("expected an error notice")
	}
}
