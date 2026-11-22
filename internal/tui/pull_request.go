package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/gasparsuarez/repora/internal/domain"
	"github.com/gasparsuarez/repora/internal/tui/components"
)

const (
	prFieldRemote = iota
	prFieldBase
	prFieldTitle
	prFieldBody
	prFieldCount
)

// PullRequestCreator is the application service required by the PR form.
type PullRequestCreator interface {
	Create(ctx context.Context, repo domain.Repository, remoteName, head, base, title, body string) (domain.PullRequest, error)
}

type pullRequestForm struct {
	active  bool
	loading bool
	gen     int

	repoID  domain.RepositoryID
	head    string
	warning string
	err     error
	cancel  context.CancelFunc

	remoteIndex int
	baseNames   []string
	baseIndex   int

	title textinput.Model
	body  textarea.Model
	focus int
}

func newPullRequestForm() pullRequestForm { return pullRequestForm{} }

// discardPullRequestForm cancels any in-flight request and clears the modal.
func (m *Model) discardPullRequestForm() {
	if m.prForm.cancel != nil {
		m.prForm.cancel()
	}
	if m.prForm.repoID != "" {
		m.setRowBusy(m.prForm.repoID, "")
	}
	m.prForm = newPullRequestForm()
}

func (m *Model) openPullRequestForm() tea.Cmd {
	if m.pullRequests == nil {
		m.notice = "pull request creation is unavailable"
		m.noticeError = true
		return nil
	}
	row := m.currentRow()
	if row == nil {
		return nil
	}
	if m.detailLoading && len(m.detail.Branches) == 0 {
		m.notice = "repository details are still loading"
		m.noticeError = false
		return nil
	}
	if len(m.detail.Remotes) == 0 {
		m.notice = "no remotes configured"
		m.noticeError = true
		return nil
	}
	if m.branchIndex < 0 || m.branchIndex >= len(m.detail.Branches) {
		return nil
	}

	branch := m.detail.Branches[m.branchIndex]
	sourceRemote := ""
	if branch.IsRemote {
		sourceRemote = remotePrefix(branch.Name)
	} else if branch.HasUpstream {
		sourceRemote = remotePrefix(branch.Upstream)
	}
	head := branch.Name
	if sourceRemote != "" && branch.IsRemote {
		head = strings.TrimPrefix(branch.Name, sourceRemote+"/")
	}
	if head == "" {
		m.notice = "could not determine the source branch"
		m.noticeError = true
		return nil
	}

	remoteIndex := indexOfRemote(m.detail.Remotes, sourceRemote)
	if remoteIndex < 0 {
		remoteIndex = indexOfRemote(m.detail.Remotes, "origin")
	}
	if remoteIndex < 0 {
		remoteIndex = 0
	}
	remote := m.detail.Remotes[remoteIndex]

	bases := m.pullRequestBases(remote.Name, head)
	if len(bases) == 0 {
		m.notice = "no base branches on " + remote.Name + "; press f in Branches to fetch"
		m.noticeError = true
		return nil
	}

	form := pullRequestForm{
		active:      true,
		repoID:      row.repo.ID,
		head:        head,
		remoteIndex: remoteIndex,
		baseNames:   bases,
		baseIndex:   indexOfDefaultBase(remote, bases),
		focus:       prFieldTitle,
	}
	form.warning = m.pullRequestWarning(form)

	form.title = textinput.New()
	form.title.Prompt = ""
	form.title.Placeholder = "Pull request title"
	form.title.CharLimit = 200
	form.title.Width = max(20, m.contentWidth()-18)
	title := branch.LastCommit.Subject
	if strings.TrimSpace(title) == "" {
		title = head
	}
	form.title.SetValue(title)
	form.title.CursorEnd()

	form.body = textarea.New()
	form.body.Placeholder = "Describe the change…"
	form.body.ShowLineNumbers = false
	form.body.CharLimit = 8000
	form.body.SetWidth(max(20, m.contentWidth()-8))
	form.body.SetHeight(m.pullRequestBodyHeight())
	form.body.Blur()

	m.prForm = form
	m.notice = ""
	m.noticeError = false
	return m.prForm.title.Focus()
}

func remotePrefix(name string) string {
	if i := strings.Index(name, "/"); i >= 0 {
		return name[:i]
	}
	return ""
}

func (m *Model) pullRequestBases(remoteName, head string) []string {
	prefix := remoteName + "/"
	var bases []string
	for _, branch := range m.detail.Branches {
		if !branch.IsRemote || !strings.HasPrefix(branch.Name, prefix) {
			continue
		}
		name := strings.TrimPrefix(branch.Name, prefix)
		if name == "" || name == "HEAD" || name == head {
			continue
		}
		bases = append(bases, name)
	}
	return bases
}

func indexOfDefaultBase(remote domain.Remote, bases []string) int {
	for _, candidate := range []string{remote.DefaultBranch, "main", "master"} {
		if candidate == "" {
			continue
		}
		for i, base := range bases {
			if base == candidate {
				return i
			}
		}
	}
	return 0
}

func indexOfRemote(remotes []domain.Remote, name string) int {
	if name == "" {
		return -1
	}
	for i, remote := range remotes {
		if remote.Name == name {
			return i
		}
	}
	return -1
}

func indexOfLocalBranch(branches []domain.Branch, name string) int {
	for i, branch := range branches {
		if !branch.IsRemote && branch.Name == name {
			return i
		}
	}
	return -1
}

func indexOfRemoteBranch(branches []domain.Branch, remote, name string) int {
	full := remote + "/" + name
	for i, branch := range branches {
		if branch.IsRemote && branch.Name == full {
			return i
		}
	}
	return -1
}

func (m *Model) pullRequestWarning(form pullRequestForm) string {
	if form.remoteIndex < 0 || form.remoteIndex >= len(m.detail.Remotes) {
		return ""
	}
	remote := m.detail.Remotes[form.remoteIndex]
	remoteIndex := indexOfRemoteBranch(m.detail.Branches, remote.Name, form.head)
	if remoteIndex < 0 {
		return "Branch is not pushed to " + remote.Name + "; press P in Branches first."
	}
	localIndex := indexOfLocalBranch(m.detail.Branches, form.head)
	if localIndex >= 0 {
		local := m.detail.Branches[localIndex]
		remoteBranch := m.detail.Branches[remoteIndex]
		if local.LastCommit.Short != "" && remoteBranch.LastCommit.Short != "" &&
			local.LastCommit.Short != remoteBranch.LastCommit.Short {
			return "Local " + form.head + " differs from " + remote.Name + "/" + form.head + "; push or pull first."
		}
	}
	return ""
}

func (m *Model) pullRequestBodyHeight() int {
	if m.height >= 32 {
		return 6
	}
	return 4
}

func (m *Model) syncPullRequestFocus() {
	if m.prForm.focus == prFieldTitle {
		m.prForm.title.Focus()
	} else {
		m.prForm.title.Blur()
	}
	if m.prForm.focus == prFieldBody {
		m.prForm.body.Focus()
	} else {
		m.prForm.body.Blur()
	}
}

func (m *Model) movePullRequestFocus(delta int) {
	if prFieldCount == 0 {
		return
	}
	m.prForm.focus = (m.prForm.focus + delta + int(prFieldCount)) % int(prFieldCount)
	m.syncPullRequestFocus()
}

func (m *Model) handlePullRequestKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if key == "esc" || key == "ctrl+c" {
		if m.prForm.loading {
			m.cancelPullRequest()
			return nil
		}
		m.prForm = newPullRequestForm()
		return nil
	}
	if m.prForm.loading {
		return nil
	}
	if key == "ctrl+s" {
		return m.submitPullRequest()
	}
	if key == "tab" {
		m.movePullRequestFocus(1)
		return nil
	}
	if key == "shift+tab" {
		m.movePullRequestFocus(-1)
		return nil
	}

	switch m.prForm.focus {
	case prFieldRemote:
		switch key {
		case "left", "h":
			m.movePullRequestRemote(-1)
		case "right", "l":
			m.movePullRequestRemote(1)
		case "enter":
			m.movePullRequestFocus(1)
		}
		return nil
	case prFieldBase:
		switch key {
		case "left", "h":
			m.movePullRequestBase(-1)
		case "right", "l":
			m.movePullRequestBase(1)
		case "enter":
			m.movePullRequestFocus(1)
		}
		return nil
	case prFieldTitle:
		if key == "enter" {
			m.prForm.focus = prFieldBody
			m.syncPullRequestFocus()
			return nil
		}
		var cmd tea.Cmd
		m.prForm.title, cmd = m.prForm.title.Update(msg)
		m.prForm.err = nil
		return cmd
	case prFieldBody:
		var cmd tea.Cmd
		m.prForm.body, cmd = m.prForm.body.Update(msg)
		m.prForm.err = nil
		return cmd
	}
	return nil
}

func (m *Model) movePullRequestRemote(delta int) {
	if len(m.detail.Remotes) < 2 {
		return
	}
	m.prForm.remoteIndex = wrapIndex(m.prForm.remoteIndex+delta, len(m.detail.Remotes))
	remote := m.detail.Remotes[m.prForm.remoteIndex]
	m.prForm.baseNames = m.pullRequestBases(remote.Name, m.prForm.head)
	if len(m.prForm.baseNames) == 0 {
		m.prForm.baseIndex = 0
		m.prForm.warning = "No base branches on " + remote.Name + "; press f in Branches to fetch."
	} else {
		m.prForm.baseIndex = indexOfDefaultBase(remote, m.prForm.baseNames)
		m.prForm.warning = m.pullRequestWarning(m.prForm)
	}
	m.prForm.err = nil
}

func (m *Model) movePullRequestBase(delta int) {
	if len(m.prForm.baseNames) < 2 {
		return
	}
	m.prForm.baseIndex = wrapIndex(m.prForm.baseIndex+delta, len(m.prForm.baseNames))
	m.prForm.err = nil
}

func wrapIndex(i, n int) int {
	if n == 0 {
		return 0
	}
	return (i%n + n) % n
}

func (m *Model) cancelPullRequest() {
	if m.prForm.cancel != nil {
		m.prForm.cancel()
		m.prForm.cancel = nil
	}
	m.prForm.gen++
	m.prForm.loading = false
	m.setRowBusy(m.prForm.repoID, "")
	m.notice = "pull request cancelled"
	m.noticeError = false
}

func (m *Model) submitPullRequest() tea.Cmd {
	if m.prForm.loading {
		return nil
	}
	title := strings.TrimSpace(m.prForm.title.Value())
	base := m.prForm.baseName()
	if err := domain.ValidatePullRequest(title, m.prForm.head, base); err != nil {
		m.prForm.err = err
		return nil
	}
	if m.prForm.remoteIndex < 0 || m.prForm.remoteIndex >= len(m.detail.Remotes) {
		m.prForm.err = domain.ErrRemoteNotFound
		return nil
	}
	remote := m.detail.Remotes[m.prForm.remoteIndex]
	repo, ok := m.cfg.RepositoryByID(m.prForm.repoID)
	if !ok {
		m.prForm.err = domain.ErrRepositoryNotFound
		return nil
	}
	svc := m.pullRequests
	if svc == nil {
		m.prForm.err = domain.ErrForgeUnavailable
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	m.prForm.gen++
	gen := m.prForm.gen
	m.prForm.loading = true
	m.prForm.cancel = cancel
	m.prForm.err = nil
	m.setRowBusy(m.prForm.repoID, "creating pull request")
	m.notice = ""

	head := m.prForm.head
	body := m.prForm.body.Value()
	return func() tea.Msg {
		pr, err := svc.Create(ctx, repo, remote.Name, head, base, title, body)
		cancel()
		return pullRequestDoneMsg{gen: gen, repoID: repo.ID, pr: pr, err: err}
	}
}

func (f pullRequestForm) baseName() string {
	if f.baseIndex < 0 || f.baseIndex >= len(f.baseNames) {
		return ""
	}
	return f.baseNames[f.baseIndex]
}

func (m *Model) renderPullRequestForm() string {
	workspace := ""
	if m.wsIndex >= 0 && m.wsIndex < len(m.cfg.Workspaces) {
		workspace = m.cfg.Workspaces[m.wsIndex].Name
	}

	var b strings.Builder
	b.WriteString(m.brandHeader())
	b.WriteString("\n" + components.Rule(m.st, m.contentWidth()) + "\n")
	b.WriteString(components.Breadcrumb(m.st, []string{"Workspaces", workspace, m.repoName(), "Branches"}, m.contentWidth(), ""))
	b.WriteString("\n\n")

	body := m.pullRequestFormBody()
	b.WriteString(m.renderPanelWith(body, strings.Count(body, "\n")+3, m.st.CardSelected))

	actions := [][2]string{{"Tab", "Next"}}
	if m.prForm.focus == prFieldRemote || m.prForm.focus == prFieldBase {
		actions = append(actions, [2]string{"←→", "Change"})
	}
	actions = append(actions, [2]string{"Ctrl+S", "Create"}, [2]string{"Esc", "Cancel"})
	return m.frame(b.String(), actions)
}

func (m *Model) pullRequestFormBody() string {
	remote := "—"
	if m.prForm.remoteIndex >= 0 && m.prForm.remoteIndex < len(m.detail.Remotes) {
		remote = m.detail.Remotes[m.prForm.remoteIndex].Name
	}
	base := m.prForm.baseName()
	if base == "" {
		base = "—"
	}

	lines := []string{
		m.st.Bar.Render("NEW PULL REQUEST"),
		m.prLabel("FROM", false) + m.st.Header.Render(m.repoName()+" · "+m.prForm.head),
		"",
		m.prSelector("ORIGIN", m.prForm.focus == prFieldRemote, remote, len(m.detail.Remotes) > 1),
		m.prSelector("BASE", m.prForm.focus == prFieldBase, base, len(m.prForm.baseNames) > 1),
		"",
		m.prLabel("TITLE", m.prForm.focus == prFieldTitle) + m.prForm.title.View(),
		m.prLabel("DESCRIPTION", m.prForm.focus == prFieldBody),
		indentBlock(m.prForm.body.View(), 2),
		"",
		m.pullRequestHint(),
	}
	return strings.Join(lines, "\n")
}

func (m *Model) prLabel(text string, active bool) string {
	marker := "  "
	style := m.st.Bar
	if active {
		marker = m.st.Accent.Render("› ")
		style = m.st.Selected
	}
	return marker + style.Render(padRight(text, 12))
}

func (m *Model) prSelector(text string, active bool, value string, selectable bool) string {
	marker := "  "
	style := m.st.Header
	if active {
		marker = m.st.Accent.Render("› ")
		style = m.st.Selected
	}
	label := m.st.Bar.Render(padRight(text, 12))
	if !selectable {
		return marker + label + style.Render(value)
	}
	return marker + label + m.st.Dim.Render("◀ ") + style.Render(value) + m.st.Dim.Render(" ▶")
}

func (m *Model) pullRequestHint() string {
	switch {
	case m.prForm.loading:
		return "  " + m.st.Info.Render("◐ creating pull request…")
	case m.prForm.err != nil:
		width := max(10, m.contentWidth()-6)
		return "  " + m.st.Danger.Render(truncate(m.prForm.err.Error(), width))
	case m.prForm.warning != "":
		width := max(10, m.contentWidth()-6)
		return "  " + m.st.Warning.Render("! "+truncate(m.prForm.warning, width-2))
	default:
		return "  " + m.st.Bar.Render("Bases come from the last fetch. Press f in Branches to refresh.")
	}
}

func indentBlock(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}
