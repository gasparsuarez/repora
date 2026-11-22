package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gasparsuarez/repora/internal/app"
	"github.com/gasparsuarez/repora/internal/domain"
)

type fakeStatusGit struct{}

func (fakeStatusGit) Status(_ context.Context, repo domain.Repository) domain.RepositoryState {
	return domain.RepositoryState{
		RepositoryID: repo.ID,
		FetchedAt:    time.Now(),
		Branch:       domain.BranchState{Head: "main"},
	}
}

func (fakeStatusGit) Root(_ context.Context, path string) (string, error) { return path, nil }

func (fakeStatusGit) Branches(_ context.Context, _ domain.Repository) ([]domain.Branch, error) {
	return nil, nil
}

func (fakeStatusGit) Log(_ context.Context, _ domain.Repository, _ int) ([]domain.Commit, error) {
	return nil, nil
}

func (fakeStatusGit) Remotes(_ context.Context, _ domain.Repository) ([]domain.Remote, error) {
	return nil, nil
}

func (fakeStatusGit) RefOID(_ context.Context, _ domain.Repository, ref string) (string, error) {
	return ref, nil
}

func (fakeStatusGit) Diff(_ context.Context, _ domain.Repository, _ domain.DiffTarget, _ string) (string, error) {
	return "", nil
}

func (fakeStatusGit) ShowCommit(_ context.Context, _ domain.Repository, _ string) (string, error) {
	return "", nil
}

func statusService() *app.StatusService {
	return app.NewStatusService(fakeStatusGit{}, 1)
}

func key(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestStatusRefreshUpdatesChangesAndPreservesSelection(t *testing.T) {
	m := repositoryModel(t)
	m.repoTab = tabChanges
	m.changeIndex = 1

	if got := m.changes[m.changeIndex].Path; got != "README.md" {
		t.Fatalf("fixture selection = %q, want README.md", got)
	}

	seq := m.nextRepoSeq("api")
	state := domain.RepositoryState{
		RepositoryID: "api",
		WorkingTree: domain.WorkingTree{
			Staged:    []domain.FileChange{{Path: "README.md", Index: domain.CodeModified}},
			Unstaged:  []domain.FileChange{{Path: "handler.go", Worktree: domain.CodeModified}},
			Untracked: []string{"notes/todo.md"},
		},
	}

	_, _ = m.Update(statusMsg{seq: seq, state: state})

	if got := m.changes[m.changeIndex].Path; got != "README.md" {
		t.Fatalf("selection moved to %q after refresh", got)
	}
	if got := m.changes[m.changeIndex].Section; got != "staged" {
		t.Fatalf("selection section = %q, want staged", got)
	}
	if m.rows[0].state.StagedCount() != 1 {
		t.Errorf("row state was not replaced: %+v", m.rows[0].state.WorkingTree)
	}
}

func TestStaleStatusCannotOverwriteNewer(t *testing.T) {
	m := repositoryModel(t)

	staleSeq := m.nextRepoSeq("api")
	stale := domain.RepositoryState{
		RepositoryID: "api",
		Branch:       domain.BranchState{Head: "old"},
	}
	_, _ = m.Update(statusMsg{seq: staleSeq, state: stale})

	freshSeq := m.nextRepoSeq("api")
	fresh := domain.RepositoryState{
		RepositoryID: "api",
		Branch:       domain.BranchState{Head: "feature/new"},
	}
	_, _ = m.Update(statusMsg{seq: freshSeq, state: fresh})

	// The stale result finishes last and must be discarded.
	_, _ = m.Update(statusMsg{seq: staleSeq, state: stale})

	if got := m.rows[0].state.Branch.Head; got != "feature/new" {
		t.Fatalf("head = %q, want the newer snapshot", got)
	}
}

func TestCommitStatusClearsChangesAndRefreshingBusy(t *testing.T) {
	m := repositoryModel(t)
	m.rows[0].busy = true
	m.rows[0].busyLabel = busyRefreshing
	m.repoTab = tabChanges

	seq := m.nextRepoSeq("api")
	_, _ = m.Update(statusMsg{
		seq:   seq,
		state: domain.RepositoryState{RepositoryID: "api"},
	})

	if len(m.changes) != 0 {
		t.Fatalf("changes = %+v, want none after a clean commit", m.changes)
	}
	if m.rows[0].busy {
		t.Error("row stayed busy after the fresh status arrived")
	}
	if out := m.View(); strings.Contains(out, "Changes (") {
		t.Errorf("tab still advertises staged files:\n%s", out)
	}
}

func TestDetailRefreshPreservesBranchAndHistorySelection(t *testing.T) {
	m := repositoryModel(t)
	m.branchIndex = 1 // feature/x
	m.historyIndex = 0

	before := append([]changeItem(nil), m.changes...)
	detail := app.RepositoryDetail{
		Branches: []domain.Branch{
			{Name: "main", IsCurrent: true},
			{Name: "new-branch"},
			{Name: "feature/x"},
		},
		Commits: []domain.Commit{
			{Hash: "newcommit"},
			{Hash: "abcdef1234567890", Short: "abcdef1", Subject: "Add retry logic"},
		},
	}

	_, _ = m.Update(detailMsg{repoID: "api", detail: detail})

	if got := m.detail.Branches[m.branchIndex].Name; got != "feature/x" {
		t.Errorf("selected branch = %q, want feature/x", got)
	}
	if got := m.detail.Commits[m.historyIndex].Hash; got != "abcdef1234567890" {
		t.Errorf("selected commit = %q, want the previous commit", got)
	}
	if len(m.changes) != len(before) {
		t.Errorf("detail refresh changed the working-tree list: %+v", m.changes)
	}
}

func TestMutatingKeysAreScopedToActiveTab(t *testing.T) {
	m, writer := writeModel(t)

	m.repoTab = tabOverview
	for _, k := range []rune{' ', 'c', 'A', 'u', 'n', 'd'} {
		_, cmd := m.Update(key(k))
		if cmd != nil {
			t.Errorf("%q produced a command on the Overview tab", k)
		}
	}
	if len(writer.calls) != 0 {
		t.Errorf("writer was called from Overview: %v", writer.calls)
	}
	if m.prompt.active() {
		t.Error("commit/branch prompt opened from Overview")
	}
	if m.confirm.active {
		t.Error("branch deletion confirmation opened from Overview")
	}

	m.repoTab = tabBranches
	for _, k := range []rune{' ', 'c', 'A', 'u'} {
		_, cmd := m.Update(key(k))
		if cmd != nil {
			t.Errorf("%q produced a command on the Branches tab", k)
		}
	}
	if len(writer.calls) != 0 {
		t.Errorf("writer was called from Branches: %v", writer.calls)
	}
}

func TestMutatingKeysAreIgnoredWhileBusy(t *testing.T) {
	m, writer := writeModel(t)
	m.rows[0].busy = true
	m.rows[0].busyLabel = "commit"

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})

	if cmd != nil {
		t.Fatal("busy repository accepted a new write")
	}
	if len(writer.calls) != 0 {
		t.Errorf("writer was called: %v", writer.calls)
	}
	if !strings.Contains(m.notice, "operation in progress") {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestWriteCompletionStaysBusyUntilFreshStatus(t *testing.T) {
	m := repositoryModel(t)
	m.status = statusService()

	_, _ = m.Update(writeDoneMsg{label: "commit", repoID: "api"})

	if !m.rows[0].busy || m.rows[0].busyLabel != busyRefreshing {
		t.Fatalf("row busy = %v (%q), want refreshing", m.rows[0].busy, m.rows[0].busyLabel)
	}

	seq := m.repoSeq["api"]
	if seq == 0 {
		t.Fatal("write completion did not schedule a status refresh")
	}
	_, _ = m.Update(statusMsg{
		seq:   seq,
		state: domain.RepositoryState{RepositoryID: "api"},
	})

	if m.rows[0].busy {
		t.Error("row stayed busy after the fresh status arrived")
	}
}

func TestScheduleRefreshUsesConfiguredInterval(t *testing.T) {
	m := testNew(configWith("alpha"), "/tmp/config.yaml", nil)
	interval := 12
	m.cfg.Preferences.RefreshInterval = &interval

	var durations []time.Duration
	m.tick = func(d time.Duration, fn func(time.Time) tea.Msg) tea.Cmd {
		durations = append(durations, d)
		return func() tea.Msg { return fn(time.Now()) }
	}

	cmd := m.scheduleRefresh()
	if cmd == nil {
		t.Fatal("expected a timer command")
	}
	msg := cmd()
	if _, ok := msg.(refreshTickMsg); !ok {
		t.Fatalf("msg = %T, want refreshTickMsg", msg)
	}

	_, next := m.Update(msg)
	if next == nil {
		t.Fatal("tick did not schedule the next refresh")
	}
	if len(durations) != 2 {
		t.Fatalf("scheduled %d ticks, want 2", len(durations))
	}
	for _, d := range durations {
		if d != 12*time.Second {
			t.Errorf("interval = %v, want 12s", d)
		}
	}
}

func TestScheduleRefreshDisabledAtZero(t *testing.T) {
	m := testNew(configWith("alpha"), "/tmp/config.yaml", nil)
	disabled := 0
	m.cfg.Preferences.RefreshInterval = &disabled

	called := false
	m.tick = func(time.Duration, func(time.Time) tea.Msg) tea.Cmd {
		called = true
		return nil
	}

	if cmd := m.scheduleRefresh(); cmd != nil {
		t.Fatal("disabled automatic refresh produced a timer")
	}
	if called {
		t.Fatal("timer was scheduled with refresh_interval: 0")
	}

	stale := m.refreshTickSeq - 1
	if _, cmd := m.Update(refreshTickMsg{seq: stale}); cmd != nil {
		t.Error("stale tick should be ignored")
	}
}

func TestAutoRefreshPausesForModalsAndBusyRows(t *testing.T) {
	m := repositoryModel(t)
	m.status = statusService()

	m.prompt = newPrompt()
	m.prompt.kind = promptCommit
	if cmd := m.autoRefresh(); cmd != nil {
		t.Error("automatic refresh ran while a prompt was open")
	}

	m.prompt = newPrompt()
	m.diffOpen = true
	if cmd := m.autoRefresh(); cmd != nil {
		t.Error("automatic refresh ran while the diff overlay was open")
	}

	m.diffOpen = false
	m.rows[0].busy = true
	if cmd := m.autoRefresh(); cmd != nil {
		t.Error("automatic refresh ran while a write was in flight")
	}
}

func TestAutoRefreshWorkspaceDoesNotFlickerLoadedRows(t *testing.T) {
	m := repositoryModel(t)
	m.status = statusService()
	m.screen = screenWorkspace
	m.rows[0].loading = false

	cmd := m.autoRefresh()
	if cmd == nil {
		t.Fatal("expected a workspace refresh command")
	}
	if m.rows[0].loading {
		t.Error("automatic refresh replaced a loaded row with a spinner")
	}
	if m.cancel != nil {
		m.cancel()
	}
}

func TestAutoRefreshHomeUsesCachedSummaries(t *testing.T) {
	m := repositoryModel(t)
	m.status = statusService()
	m.screen = screenHome
	m.homeLoaded = true
	m.homeStates = map[domain.RepositoryID]domain.RepositoryState{
		"api": {RepositoryID: "api"},
	}

	cmd := m.autoRefresh()
	if cmd == nil {
		t.Fatal("expected a home refresh command")
	}
	if !m.homeLoading {
		t.Error("home refresh did not start")
	}
	if len(m.homeStates) == 0 {
		t.Error("home refresh discarded cached states")
	}
	if m.homeCancel != nil {
		m.homeCancel()
	}
}
