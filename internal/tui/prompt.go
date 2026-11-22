package tui

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/gasparsuarez/repora/internal/domain"
	"github.com/gasparsuarez/repora/internal/tui/components"
)

type promptKind int

const (
	promptNone promptKind = iota
	promptCommit
	promptNewBranch
	promptWorkspaceName
	promptWorkspaceDescription
	promptWorkspaceRename
	promptWorkspaceRenameDescription
	promptRepoPath
)

type promptState struct {
	kind        promptKind
	input       textinput.Model
	err         error
	target      domain.WorkspaceID
	pendingName string
}

func newPrompt() promptState {
	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = 200
	return promptState{input: input}
}

func (p promptState) active() bool { return p.kind != promptNone }

func (m *Model) openCommitPrompt() tea.Cmd {
	row := m.currentRow()
	if row == nil {
		return nil
	}
	if m.git == nil {
		m.notice = "write operations are unavailable"
		m.noticeError = true
		return nil
	}
	if row.state.StagedCount() == 0 {
		m.notice = "nothing staged to commit"
		m.noticeError = false
		return nil
	}

	m.prompt = newPrompt()
	m.prompt.kind = promptCommit
	m.prompt.input.Placeholder = "commit message"
	m.prompt.input.Width = max(20, m.contentWidth()-12)
	return m.prompt.input.Focus()
}

func (m *Model) openBranchPrompt() tea.Cmd {
	if _, ok := m.currentRepo(); !ok {
		return nil
	}
	if m.git == nil {
		m.notice = "write operations are unavailable"
		m.noticeError = true
		return nil
	}

	m.prompt = newPrompt()
	m.prompt.kind = promptNewBranch
	m.prompt.input.Placeholder = "feature/my-branch"
	m.prompt.input.Width = max(20, m.contentWidth()-12)
	return m.prompt.input.Focus()
}

func (m *Model) openWorkspaceNamePrompt() tea.Cmd {
	if !m.configReady() {
		return nil
	}

	m.prompt = newPrompt()
	m.prompt.kind = promptWorkspaceName
	m.prompt.input.Placeholder = "Product X"
	m.prompt.input.Width = max(20, m.contentWidth()-12)
	return m.prompt.input.Focus()
}

func (m *Model) openWorkspaceRenamePrompt() tea.Cmd {
	if !m.configReady() {
		return nil
	}
	if m.wsIndex < 0 || m.wsIndex >= len(m.cfg.Workspaces) {
		return nil
	}

	ws := m.cfg.Workspaces[m.wsIndex]
	m.prompt = newPrompt()
	m.prompt.kind = promptWorkspaceRename
	m.prompt.target = ws.ID
	m.prompt.input.Placeholder = "Workspace name"
	m.prompt.input.Width = max(20, m.contentWidth()-12)
	m.prompt.input.SetValue(ws.Name)
	m.prompt.input.CursorEnd()
	return m.prompt.input.Focus()
}

func (m *Model) openRepoPathPrompt() tea.Cmd {
	if !m.configReady() {
		return nil
	}
	if m.selectedWorkspaceID() == "" {
		m.notice = "open a workspace first"
		m.noticeError = false
		return nil
	}

	m.prompt = newPrompt()
	m.prompt.kind = promptRepoPath
	m.prompt.input.Placeholder = "~/code/project/api"
	m.prompt.input.Width = max(20, m.contentWidth()-12)
	return m.prompt.input.Focus()
}

func (m *Model) handlePromptKey(key tea.KeyMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		m.prompt = newPrompt()
		return nil
	case "enter":
		return m.submitPrompt()
	}

	var cmd tea.Cmd
	m.prompt.input, cmd = m.prompt.input.Update(key)
	m.prompt.err = nil
	return cmd
}

func (m *Model) submitPrompt() tea.Cmd {
	value := strings.TrimSpace(m.prompt.input.Value())
	repo, hasRepo := m.currentRepo()

	switch m.prompt.kind {
	case promptCommit:
		if err := domain.ValidateCommitMessage(value); err != nil {
			m.prompt.err = errors.New("commit message cannot be empty")
			return nil
		}
		if !hasRepo {
			m.prompt = newPrompt()
			return nil
		}
		message := value
		m.prompt = newPrompt()
		return m.runWrite("commit", repo, func(ctx context.Context) error {
			return m.git.Commit(ctx, repo, message)
		})

	case promptNewBranch:
		if err := domain.ValidateBranchName(value); err != nil {
			m.prompt.err = err
			return nil
		}
		if !hasRepo {
			m.prompt = newPrompt()
			return nil
		}
		name := value
		m.prompt = newPrompt()
		return m.runWrite("create branch "+name, repo, func(ctx context.Context) error {
			return m.git.CreateBranch(ctx, repo, name)
		})

	case promptWorkspaceName:
		if err := domain.ValidateName(value); err != nil {
			m.prompt.err = errors.New("workspace name cannot be empty")
			return nil
		}
		return m.openDescriptionStep(value, promptWorkspaceDescription, "", "")

	case promptWorkspaceDescription:
		name := m.prompt.pendingName
		m.prompt = newPrompt()
		return m.createWorkspaceCmd(name, value)

	case promptWorkspaceRename:
		if err := domain.ValidateName(value); err != nil {
			m.prompt.err = errors.New("workspace name cannot be empty")
			return nil
		}
		target := m.prompt.target
		return m.openDescriptionStep(value, promptWorkspaceRenameDescription, target, m.workspaceDescription(target))

	case promptWorkspaceRenameDescription:
		name := m.prompt.pendingName
		target := m.prompt.target
		m.prompt = newPrompt()
		return m.renameWorkspaceCmd(target, name, value)

	case promptRepoPath:
		if value == "" {
			m.prompt.err = errors.New("path cannot be empty")
			return nil
		}
		m.prompt = newPrompt()
		return m.addRepositoryCmd(value)
	}

	m.prompt = newPrompt()
	return nil
}

// runWrite executes a mutating operation and reports the result as a message.
func (m *Model) runWrite(label string, repo domain.Repository, fn func(context.Context) error) tea.Cmd {
	if row := m.repoRowByID(repo.ID); row != nil && row.busy {
		m.notice = "operation in progress: " + row.busyLabel
		m.noticeError = false
		return nil
	}

	m.notice = label + "…"
	m.noticeError = false
	m.setRowBusy(repo.ID, label)

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		return writeDoneMsg{label: label, repoID: repo.ID, err: fn(ctx)}
	}
}

func (m *Model) stageSelected() tea.Cmd {
	repo, ok := m.currentRepo()
	if !ok || m.git == nil {
		return nil
	}
	if m.changeIndex < 0 || m.changeIndex >= len(m.changes) {
		return nil
	}

	item := m.changes[m.changeIndex]
	switch item.Section {
	case "unstaged", "untracked":
		path := item.Path
		return m.runWrite("stage "+path, repo, func(ctx context.Context) error {
			return m.git.Stage(ctx, repo, []string{path})
		})
	case "staged":
		path := item.Path
		return m.runWrite("unstage "+path, repo, func(ctx context.Context) error {
			return m.git.Unstage(ctx, repo, []string{path})
		})
	case "conflicts":
		m.notice = "conflicts must be resolved manually in a terminal"
		m.noticeError = true
	}
	return nil
}

func (m *Model) openErrorDetail() {
	if m.errDetail == "" {
		return
	}
	m.errOpen = true
	m.errScroll = 0
}

func (m *Model) renderPrompt() string {
	crumbs, title := m.promptContext()

	var b strings.Builder
	b.WriteString(m.brandHeader())
	b.WriteString("\n" + components.Rule(m.st, m.contentWidth()) + "\n")
	b.WriteString(components.Breadcrumb(m.st, crumbs, m.contentWidth(), ""))
	b.WriteString("\n\n")

	body := []string{
		m.st.Bar.Render(title),
		"",
		m.prompt.input.View(),
		"",
		m.promptHint(),
	}
	b.WriteString(m.renderPanelWith(strings.Join(body, "\n"), len(body)+2, m.st.CardSelected))

	return m.frame(b.String(), [][2]string{{"Enter", "Submit"}, {"Esc", "Cancel"}})
}

// promptContext returns the breadcrumb trail and the panel title for the active
// prompt, so every form shares the screen shell.
func (m *Model) promptContext() ([]string, string) {
	workspace := ""
	if m.wsIndex >= 0 && m.wsIndex < len(m.cfg.Workspaces) {
		workspace = m.cfg.Workspaces[m.wsIndex].Name
	}

	switch m.prompt.kind {
	case promptCommit:
		return []string{"Workspaces", workspace, m.repoName()}, "COMMIT MESSAGE"
	case promptNewBranch:
		return []string{"Workspaces", workspace, m.repoName()}, "NEW BRANCH"
	case promptWorkspaceName:
		return []string{"Workspaces"}, "NEW WORKSPACE"
	case promptWorkspaceDescription:
		return []string{"Workspaces", m.prompt.pendingName}, "DESCRIPTION"
	case promptWorkspaceRename:
		return []string{"Workspaces", workspace}, "RENAME WORKSPACE"
	case promptWorkspaceRenameDescription:
		return []string{"Workspaces", m.prompt.pendingName}, "DESCRIPTION"
	case promptRepoPath:
		return []string{"Workspaces", workspace}, "ADD REPOSITORY"
	default:
		return []string{"Workspaces"}, ""
	}
}

func (m *Model) repoName() string {
	if repo, ok := m.currentRepo(); ok {
		return repo.Name
	}
	return ""
}

// promptHint returns the contextual helper line shown under the input.
func (m *Model) promptHint() string {
	switch {
	case m.prompt.err != nil:
		return m.st.Danger.Render(truncate(m.prompt.err.Error(), max(10, m.contentWidth()-8)))
	case m.prompt.kind == promptCommit:
		if row := m.currentRow(); row != nil {
			return m.st.Bar.Render(plural(row.state.StagedCount(), "file staged", "files staged"))
		}
		return ""
	case m.prompt.kind == promptWorkspaceDescription || m.prompt.kind == promptWorkspaceRenameDescription:
		return m.st.Bar.Render("Optional. Enter saves without a description.")
	case m.prompt.kind == promptRepoPath:
		return m.st.Bar.Render("The path is resolved to the repository root.")
	case m.prompt.kind == promptWorkspaceName:
		return m.st.Bar.Render("The workspace id is derived from the name.")
	default:
		return ""
	}
}

// openDescriptionStep moves from the name step to the optional description
// step, keeping the pending name and target.
func (m *Model) openDescriptionStep(name string, kind promptKind, target domain.WorkspaceID, current string) tea.Cmd {
	m.prompt = newPrompt()
	m.prompt.kind = kind
	m.prompt.pendingName = name
	m.prompt.target = target
	m.prompt.input.Placeholder = "Optional description"
	m.prompt.input.Width = max(20, m.contentWidth()-12)
	if current != "" {
		m.prompt.input.SetValue(current)
		m.prompt.input.CursorEnd()
	}
	return m.prompt.input.Focus()
}

func (m *Model) workspaceDescription(id domain.WorkspaceID) string {
	if ws, ok := m.cfg.WorkspaceByID(id); ok {
		return ws.Description
	}
	return ""
}
