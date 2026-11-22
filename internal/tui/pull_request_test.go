package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gasparsuarez/repora/internal/domain"
)

type fakePRCreator struct {
	repo   domain.Repository
	remote string
	head   string
	base   string
	title  string
	body   string
	calls  int
	result domain.PullRequest
	err    error
}

func (f *fakePRCreator) Create(_ context.Context, repo domain.Repository, remoteName, head, base, title, body string) (domain.PullRequest, error) {
	f.calls++
	f.repo = repo
	f.remote = remoteName
	f.head = head
	f.base = base
	f.title = title
	f.body = body
	return f.result, f.err
}

func pullRequestModel(t *testing.T) (*Model, *fakePRCreator) {
	t.Helper()
	return pullRequestModelAt(t, 120, 32)
}

func pullRequestModelAt(t *testing.T, width, height int) (*Model, *fakePRCreator) {
	t.Helper()

	m := repositoryModel(t)
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m.repoTab = tabBranches
	m.detail.Remotes = []domain.Remote{
		{Name: "origin", FetchURL: "git@github.com:acme/api.git", DefaultBranch: "main"},
		{Name: "upstream", FetchURL: "git@github.com:acme/upstream.git", DefaultBranch: "release"},
	}
	m.detail.Branches = []domain.Branch{
		{Name: "main", IsCurrent: true, Upstream: "origin/main", HasUpstream: true},
		{Name: "feature/retry", Upstream: "origin/feature/retry", HasUpstream: true,
			LastCommit: domain.Commit{Short: "bbb", Subject: "Add retry logic"}},
		{Name: "origin/main", IsRemote: true, LastCommit: domain.Commit{Short: "aaa"}},
		{Name: "origin/develop", IsRemote: true, LastCommit: domain.Commit{Short: "ddd"}},
		{Name: "origin/feature/retry", IsRemote: true, LastCommit: domain.Commit{Short: "bbb"}},
		{Name: "upstream/main", IsRemote: true, LastCommit: domain.Commit{Short: "aaa"}},
		{Name: "upstream/release", IsRemote: true, LastCommit: domain.Commit{Short: "eee"}},
		{Name: "upstream/feature/retry", IsRemote: true, LastCommit: domain.Commit{Short: "bbb"}},
	}
	m.branchIndex = indexOfBranch(m.detail.Branches, "feature/retry")

	creator := &fakePRCreator{result: domain.PullRequest{Number: 8, URL: "https://github.com/acme/api/pull/8"}}
	m.pullRequests = creator
	return m, creator
}

func TestPullRequestFormOpensFromBranches(t *testing.T) {
	m, _ := pullRequestModel(t)

	_, _ = m.Update(key('o'))

	if !m.prForm.active {
		t.Fatal("pull request form did not open")
	}
	if m.prForm.head != "feature/retry" {
		t.Errorf("head = %q", m.prForm.head)
	}
	if got := m.detail.Remotes[m.prForm.remoteIndex].Name; got != "origin" {
		t.Errorf("remote = %q, want origin", got)
	}
	if got := m.prForm.baseName(); got != "main" {
		t.Errorf("base = %q, want main", got)
	}
	if got := m.prForm.title.Value(); got != "Add retry logic" {
		t.Errorf("title = %q", got)
	}
	if m.prForm.warning != "" {
		t.Errorf("unexpected warning: %q", m.prForm.warning)
	}
	if out := m.View(); !strings.Contains(out, "FROM") || !strings.Contains(out, "DESCRIPTION") {
		t.Errorf("form view missing fields:\n%s", out)
	}
}

func TestPullRequestFormChangesRemoteAndBase(t *testing.T) {
	m, _ := pullRequestModel(t)
	_, _ = m.Update(key('o'))

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.prForm.focus != prFieldBase {
		t.Fatalf("focus = %d, want base", m.prForm.focus)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := m.prForm.baseName(); got != "develop" {
		t.Fatalf("base = %q, want develop", got)
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.prForm.focus != prFieldRemote {
		t.Fatalf("focus = %d, want remote", m.prForm.focus)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := m.detail.Remotes[m.prForm.remoteIndex].Name; got != "upstream" {
		t.Fatalf("remote = %q, want upstream", got)
	}
	if got := m.prForm.baseName(); got != "release" {
		t.Fatalf("base = %q, want release", got)
	}
	if m.prForm.warning != "" {
		t.Errorf("unexpected warning: %q", m.prForm.warning)
	}
}

func TestPullRequestFormSubmitSuccess(t *testing.T) {
	m, creator := pullRequestModel(t)
	_, _ = m.Update(key('o'))
	m.prForm.title.SetValue("Improve retries")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("submit produced no command")
	}
	msg := cmd()
	done, ok := msg.(pullRequestDoneMsg)
	if !ok {
		t.Fatalf("msg = %T", msg)
	}
	_, _ = m.Update(done)

	if creator.calls != 1 {
		t.Fatalf("creator calls = %d", creator.calls)
	}
	if creator.remote != "origin" || creator.head != "feature/retry" || creator.base != "main" {
		t.Errorf("request = %+v", creator)
	}
	if creator.title != "Improve retries" {
		t.Errorf("title = %q", creator.title)
	}
	if m.prForm.active {
		t.Error("form stayed open after success")
	}
	if !strings.Contains(m.notice, "PR #8") || !strings.Contains(m.notice, "pull/8") {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestPullRequestFormErrorKeepsInput(t *testing.T) {
	m, creator := pullRequestModel(t)
	creator.err = domain.ErrBranchNotPushed
	_, _ = m.Update(key('o'))
	m.prForm.title.SetValue("Keep this title")
	m.prForm.body.SetValue("Keep this body")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	msg := cmd()
	_, _ = m.Update(msg)

	if !m.prForm.active {
		t.Fatal("form closed after an error")
	}
	if m.prForm.err == nil {
		t.Fatal("form has no error")
	}
	if got := m.prForm.title.Value(); got != "Keep this title" {
		t.Errorf("title was lost: %q", got)
	}
	if got := m.prForm.body.Value(); got != "Keep this body" {
		t.Errorf("body was lost: %q", got)
	}
	if m.rows[0].busy {
		t.Error("row stayed busy after the error")
	}
}

func TestPullRequestEscCancelsLoading(t *testing.T) {
	m, _ := pullRequestModel(t)
	_, _ = m.Update(key('o'))
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if !m.prForm.loading {
		t.Fatal("form is not loading")
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.prForm.active {
		t.Error("cancel closed the form")
	}
	if m.prForm.loading {
		t.Error("cancel left the form loading")
	}
	if m.rows[0].busy {
		t.Error("cancel left the row busy")
	}
	if !strings.Contains(m.notice, "cancelled") {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestPullRequestKeyIsScopedToBranches(t *testing.T) {
	m, _ := pullRequestModel(t)
	m.repoTab = tabChanges

	_, _ = m.Update(key('o'))
	if m.prForm.active {
		t.Fatal("pull request form opened outside Branches")
	}
}

func TestBranchesFooterKeepsBackAt80Columns(t *testing.T) {
	m := repositoryModel(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.repoTab = tabBranches

	out := m.View()
	if !strings.Contains(out, "Esc Back") {
		t.Errorf("Branches footer dropped the back action:\n%s", out)
	}
	if !strings.Contains(out, "o PR") {
		t.Errorf("Branches footer dropped the pull request action:\n%s", out)
	}
}

func TestAutoRefreshPausesForPullRequestForm(t *testing.T) {
	m, _ := pullRequestModel(t)
	_, _ = m.Update(key('o'))
	if cmd := m.autoRefresh(); cmd != nil {
		t.Error("automatic refresh ran while the pull request form was open")
	}
}
