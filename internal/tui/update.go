package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gasparsuarez/repora/internal/app"
	"github.com/gasparsuarez/repora/internal/domain"
)

// noticeDuration is how long a success notice stays visible.
const noticeDuration = 4 * time.Second

// Update implements tea.Model. It wraps update to schedule the auto-dismissal
// of success notices without touching every call site.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	before := m.notice

	model, cmd := m.update(msg)

	if m.notice != before {
		m.noticeSeq++
		if m.notice != "" && !m.noticeError && m.tick != nil {
			seq := m.noticeSeq
			tick := m.tick(noticeDuration, func(time.Time) tea.Msg {
				return noticeExpiredMsg{seq: seq}
			})
			cmd = tea.Batch(cmd, tick)
		}
	}

	return model, cmd
}

func (m *Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case noticeExpiredMsg:
		if msg.seq == m.noticeSeq && !m.noticeError {
			m.notice = ""
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.syncHomeWindow()
		return m, nil

	case tea.KeyMsg:
		return m, m.handleKey(msg)

	case homeStatusMsg:
		if msg.gen != m.homeGen {
			return m, nil
		}
		if m.homeStates == nil {
			m.homeStates = make(map[domain.RepositoryID]domain.RepositoryState)
		}
		m.homeStates[msg.state.RepositoryID] = msg.state
		return m, m.waitHome()

	case homeDoneMsg:
		if msg.gen != m.homeGen {
			return m, nil
		}
		m.homeLoading = false
		m.homeCancel = nil
		return m, m.waitHome()

	case refreshTickMsg:
		if msg.seq != m.refreshTickSeq {
			return m, nil
		}
		return m, tea.Batch(m.autoRefresh(), m.scheduleRefresh())

	case statusMsg:
		if !m.repoSeqCurrent(msg.state.RepositoryID, msg.seq) {
			return m, nil
		}
		m.applyStatus(msg.state)
		return m, m.wait()

	case batchDoneMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		for i := range m.rows {
			m.rows[i].loading = false
		}
		m.cancel = nil
		return m, m.wait()

	case editorClosedMsg:
		if msg.err != nil {
			m.notice = "editor: " + msg.err.Error()
			m.noticeError = true
			return m, nil
		}
		return m, m.reloadCmd()

	case configReloadedMsg:
		if msg.err != nil {
			m.notice = "reload failed: " + msg.err.Error()
			m.noticeError = true
			return m, nil
		}
		if msg.cfg == nil {
			return m, nil
		}
		return m, tea.Batch(m.applyReload(msg.cfg, msg.warnings, msg.status), m.scheduleRefresh())

	case detailMsg:
		if msg.gen != m.detailGen || m.screen != screenRepository {
			return m, nil
		}
		if repo, ok := m.currentRepo(); !ok || repo.ID != msg.repoID {
			return m, nil
		}
		m.detailLoading = false
		m.applyDetail(msg.detail)
		return m, nil

	case commitDetailMsg:
		if msg.gen != m.detailGen {
			return m, nil
		}
		m.commitLoading = false
		m.commitContent = msg.content
		m.commitErr = msg.err
		m.commitScroll = 0
		return m, nil

	case diffMsg:
		if msg.gen != m.detailGen {
			return m, nil
		}
		m.diffLoading = false
		m.diffOpen = true
		m.diffTitle = msg.title
		m.diffContent = msg.content
		m.diffErr = msg.err
		m.diffScroll = 0
		return m, nil

	case writeDoneMsg:
		// Keep the repository busy until the post-write status arrives, so a
		// repeated key cannot act on the pre-write snapshot.
		if m.status != nil {
			m.setRowBusy(msg.repoID, busyRefreshing)
		} else {
			m.setRowBusy(msg.repoID, "")
		}
		if msg.err != nil {
			hint := app.Explain(msg.err)
			m.errDetail = msg.err.Error()
			if hint != "" {
				m.errDetail += "\n\nHint: " + hint
			}
			short := msg.err.Error()
			if hint != "" {
				short = hint
			}
			m.notice = msg.label + " failed: " + truncate(short, max(20, m.contentWidth()-20))
			m.noticeError = true
		} else {
			m.notice = msg.label + " · done"
			m.noticeError = false
		}

		cmds := []tea.Cmd{m.refreshRepoID(msg.repoID)}
		if m.screen == screenRepository {
			if repo, ok := m.currentRepo(); ok && repo.ID == msg.repoID {
				cmds = append(cmds, m.loadDetail())
			}
		}
		return m, tea.Batch(cmds...)

	case configChangedMsg:
		if msg.err != nil {
			m.errDetail = msg.err.Error()
			m.notice = msg.action + " failed: " + truncate(msg.err.Error(), max(20, m.width-30))
			m.noticeError = true
			if msg.cfg != nil {
				m.applyConfig(msg.cfg, msg.selectWS)
				if m.screen == screenWorkspace {
					return m, m.rebuildRowsAndRefresh()
				}
				if m.screen == screenHome {
					return m, m.reloadHome()
				}
			}
			return m, nil
		}

		m.applyConfig(msg.cfg, msg.selectWS)
		m.notice = msg.action + " · done"
		m.noticeError = false

		if m.screen == screenWorkspace {
			return m, m.rebuildRowsAndRefresh()
		}
		if m.screen == screenHome {
			return m, m.reloadHome()
		}
		return m, nil

	default:
		if m.prompt.active() {
			var cmd tea.Cmd
			m.prompt.input, cmd = m.prompt.input.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *Model) wait() tea.Cmd {
	if m.ch == nil {
		return nil
	}
	return waitFor(m.ch)
}

func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// Ctrl+C is global: first press cancels work, a second press quits.
	if key == "ctrl+c" {
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
			m.notice = "operation cancelled"
			m.noticeError = false
			return nil
		}
		return tea.Quit
	}

	if m.showHelp {
		m.showHelp = false
		return nil
	}

	if m.prompt.active() {
		return m.handlePromptKey(msg)
	}

	if m.confirm.active {
		return m.handleConfirmKey(key)
	}

	if m.errOpen {
		page := max(1, m.height-8)
		switch key {
		case "esc", "q", "enter":
			m.errOpen = false
		case "down", "j":
			m.errScroll++
		case "up", "k":
			m.errScroll--
		case "pgdown", " ":
			m.errScroll += page
		case "pgup":
			m.errScroll -= page
		case "g":
			m.errScroll = 0
		case "G":
			m.errScroll = len(strings.Split(m.errWrapped(), "\n"))
		}
		m.clampErrScroll(page)
		return nil
	}

	if key == "?" {
		m.showHelp = true
		return nil
	}

	// Configuration access is available from every primary screen.
	switch key {
	case ",":
		return m.openConfigEditor()
	case "ctrl+r":
		return m.reloadCmd()
	}

	if m.screen == screenHome {
		return m.handleHomeKey(key)
	}
	if m.screen == screenRepository {
		return m.handleRepositoryKey(key)
	}
	return m.handleWorkspaceKey(key)
}

func (m *Model) handleHomeKey(key string) tea.Cmd {
	defer m.syncHomeWindow()

	switch key {
	case "q":
		return tea.Quit

	case "n":
		return m.openWorkspaceNamePrompt()
	case "e":
		return m.openWorkspaceRenamePrompt()
	case "d":
		m.openDeleteWorkspaceConfirm()
		return nil
	case "r":
		return m.reloadHome()

	case "up", "k":
		if m.wsIndex > 0 {
			m.wsIndex--
		}
	case "down", "j":
		if m.wsIndex < len(m.cfg.Workspaces)-1 {
			m.wsIndex++
		}
	case "g":
		m.wsIndex = 0
	case "G":
		if n := len(m.cfg.Workspaces); n > 0 {
			m.wsIndex = n - 1
		}
	case "enter":
		return m.openWorkspace()
	}
	return nil
}

func (m *Model) handleWorkspaceKey(key string) tea.Cmd {
	if key == "f" || key == "p" || key == "P" {
		if row := m.currentRow(); row != nil && row.busy {
			m.notice = "operation in progress: " + row.busyLabel
			m.noticeError = false
			return nil
		}
	}

	switch key {
	case "esc":
		m.closeWorkspace()

	case "up", "k":
		if m.rowIndex > 0 {
			m.rowIndex--
		}
	case "down", "j":
		if m.rowIndex < len(m.rows)-1 {
			m.rowIndex++
		}
	case "g":
		m.rowIndex = 0
	case "G":
		if n := len(m.rows); n > 0 {
			m.rowIndex = n - 1
		}

	case "r":
		if m.cancel != nil {
			m.notice = "a refresh is already running"
			m.noticeError = false
			return nil
		}
		m.notice = ""
		return m.startRefresh()

	case "a":
		return m.openRepoPathPrompt()
	case "x":
		m.openRemoveRepositoryConfirm()
		return nil

	case "f":
		if !m.gitReady() {
			return nil
		}
		if repo, ok := m.currentRepo(); ok {
			return m.runWrite("fetch "+repo.Name, repo, func(ctx context.Context) error {
				return m.git.Fetch(ctx, repo)
			})
		}
	case "p":
		if !m.gitReady() {
			return nil
		}
		if repo, ok := m.currentRepo(); ok {
			return m.runWrite("pull "+repo.Name, repo, func(ctx context.Context) error {
				return m.git.Pull(ctx, repo)
			})
		}
	case "P":
		if !m.gitReady() {
			return nil
		}
		if repo, ok := m.currentRepo(); ok {
			return m.runWrite("push "+repo.Name, repo, func(ctx context.Context) error {
				return m.git.Push(ctx, repo)
			})
		}
	case "e":
		m.openErrorDetail()

	case "enter":
		return m.openRepository()

	case "q":
		m.closeWorkspace()
	}
	return nil
}
