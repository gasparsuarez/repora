package tui

import (
	"context"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gasparsuarez/repora/internal/app"
	"github.com/gasparsuarez/repora/internal/domain"
)

// Layout margins keep content away from the terminal edges.
const (
	contentMarginX   = 2
	contentMarginTop = 1
)

type screen int

const (
	screenHome screen = iota
	screenWorkspace
	screenRepository
)

type repoRow struct {
	repo        domain.Repository
	state       domain.RepositoryState
	loading     bool
	unavailable bool
	busy        bool
	busyLabel   string
}

// Deps are the dependencies required to build the root model.
type Deps struct {
	Config       *domain.Config
	ConfigPath   string
	Version      string
	Status       *app.StatusService
	Repos        *app.RepositoryService
	Git          *app.GitService
	PullRequests PullRequestCreator
	Workspaces   *app.WorkspaceService
	Warnings     []string
	NoColor      bool
	Reload       Reloader
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg          *domain.Config
	configPath   string
	version      string
	status       *app.StatusService
	repos        *app.RepositoryService
	git          *app.GitService
	pullRequests PullRequestCreator
	workspaces   *app.WorkspaceService
	warnings     []string
	reload       Reloader
	st           styles

	screen   screen
	wsIndex  int
	rows     []repoRow
	rowIndex int

	repoTab       repoTab
	detail        app.RepositoryDetail
	detailLoading bool
	detailGen     int
	changes       []changeItem
	changeIndex   int
	branchIndex   int
	historyIndex  int

	diffOpen    bool
	diffLoading bool
	diffTitle   string
	diffContent string
	diffErr     error
	diffScroll  int
	commitOpen  bool

	commitContent string
	commitErr     error
	commitLoading bool
	commitScroll  int

	gen            int
	ch             chan tea.Msg
	cancel         context.CancelFunc
	repoSeq        map[domain.RepositoryID]int
	refreshTickSeq int

	homeStates  map[domain.RepositoryID]domain.RepositoryState
	homeLoaded  bool
	homeLoading bool
	homeGen     int
	homeCh      chan tea.Msg
	homeCancel  context.CancelFunc
	homeOffset  int

	tick func(time.Duration, func(time.Time) tea.Msg) tea.Cmd

	prompt    promptState
	prForm    pullRequestForm
	confirm   confirmState
	errDetail string
	errOpen   bool
	errScroll int

	width, height int
	notice        string
	noticeError   bool
	noticeSeq     int
	showHelp      bool
}

// New builds the root model.
func New(deps Deps) *Model {
	return &Model{
		cfg:          deps.Config,
		configPath:   deps.ConfigPath,
		version:      deps.Version,
		status:       deps.Status,
		repos:        deps.Repos,
		git:          deps.Git,
		pullRequests: deps.PullRequests,
		workspaces:   deps.Workspaces,
		warnings:     deps.Warnings,
		reload:       deps.Reload,
		st:           newStyles(deps.NoColor),
		screen:       screenHome,
		prompt:       newPrompt(),
		tick:         tea.Tick,
		repoSeq:      make(map[domain.RepositoryID]int),
	}
}

// Init implements tea.Model. It loads the workspace summaries shown on the
// explorer and starts the automatic refresh loop. Both are read-only and
// bounded.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.loadHomeSummaries(), m.scheduleRefresh())
}

// refreshInterval returns the configured automatic refresh interval, or zero
// when automatic refresh is disabled.
func (m *Model) refreshInterval() time.Duration {
	if m.cfg == nil {
		return 0
	}
	seconds := m.cfg.Preferences.RefreshIntervalSeconds()
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// scheduleRefresh arms the next automatic refresh tick. The sequence number
// invalidates a pending tick when the configuration changes.
func (m *Model) scheduleRefresh() tea.Cmd {
	m.refreshTickSeq++
	seq := m.refreshTickSeq

	interval := m.refreshInterval()
	if interval <= 0 || m.tick == nil {
		return nil
	}
	return m.tick(interval, func(time.Time) tea.Msg {
		return refreshTickMsg{seq: seq}
	})
}

// autoRefresh performs one automatic refresh for the visible screen. It never
// runs while a modal is open, a write is in flight, or a refresh is already
// running, and it never changes the selection.
func (m *Model) autoRefresh() tea.Cmd {
	if m.prompt.active() || m.prForm.active || m.confirm.active || m.errOpen || m.showHelp || m.diffOpen || m.commitOpen {
		return nil
	}
	if m.anyRepoBusy() {
		return nil
	}

	switch m.screen {
	case screenHome:
		if m.homeLoading {
			return nil
		}
		return m.loadHomeSummaries()
	case screenWorkspace:
		if m.cancel != nil {
			return nil
		}
		return m.startRefreshQuiet()
	case screenRepository:
		row := m.currentRow()
		if row == nil || row.unavailable || row.loading || m.detailLoading {
			return nil
		}
		return tea.Batch(m.refreshRepoIDQuiet(row.repo.ID), m.loadDetail())
	}
	return nil
}

// openWorkspace switches to the dashboard of the selected workspace.
func (m *Model) openWorkspace() tea.Cmd {
	if len(m.cfg.Workspaces) == 0 {
		m.notice = "no workspaces configured"
		return nil
	}

	m.screen = screenWorkspace
	m.rowIndex = 0
	m.notice = ""
	m.noticeError = false

	return m.rebuildRowsAndRefresh()
}

// rebuildRowsAndRefresh regenerates the repository rows for the current
// workspace and starts a refresh. It keeps the row selection when it still
// points at a valid row.
func (m *Model) rebuildRowsAndRefresh() tea.Cmd {
	if m.wsIndex < 0 || m.wsIndex >= len(m.cfg.Workspaces) {
		return nil
	}

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	ws := m.cfg.Workspaces[m.wsIndex]
	repos := m.cfg.RepositoriesOf(ws)

	m.rows = make([]repoRow, len(repos))
	for i, repo := range repos {
		row := repoRow{repo: repo, loading: true}
		if info, err := os.Stat(repo.Path); err != nil || !info.IsDir() {
			row.loading = false
			row.unavailable = true
		}
		m.rows[i] = row
	}

	if m.rowIndex >= len(m.rows) {
		m.rowIndex = max(0, len(m.rows)-1)
	}

	return m.startRefresh()
}

// closeWorkspace cancels any in-flight refresh and returns home.
func (m *Model) closeWorkspace() {
	m.mergeRowsIntoHome()
	m.discardPullRequestForm()

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.gen++
	m.ch = nil
	m.rows = nil
	m.screen = screenHome
	m.notice = ""
	m.noticeError = false
}

func (m *Model) startRefresh() tea.Cmd {
	return m.startRefreshMode(true)
}

// startRefreshQuiet refreshes without replacing a known state with a spinner.
// It is used by the automatic refresh loop so background work does not flicker.
func (m *Model) startRefreshQuiet() tea.Cmd {
	return m.startRefreshMode(false)
}

func (m *Model) startRefreshMode(markLoading bool) tea.Cmd {
	svc := m.status
	if svc == nil {
		return nil
	}

	repos := make([]domain.Repository, 0, len(m.rows))
	seqByID := make(map[domain.RepositoryID]int, len(m.rows))
	for i := range m.rows {
		if m.rows[i].unavailable {
			continue
		}
		if markLoading || m.rows[i].state.RepositoryID == "" {
			m.rows[i].loading = true
		}
		seqByID[m.rows[i].repo.ID] = m.nextRepoSeq(m.rows[i].repo.ID)
		repos = append(repos, m.rows[i].repo)
	}
	if len(repos) == 0 {
		return nil
	}

	if m.cancel != nil {
		m.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.gen++
	gen := m.gen

	ch := make(chan tea.Msg)
	m.ch = ch

	start := func() tea.Msg {
		go func() {
			svc.Refresh(ctx, repos, func(state domain.RepositoryState) {
				select {
				case ch <- statusMsg{gen: gen, seq: seqByID[state.RepositoryID], state: state}:
				case <-ctx.Done():
				}
			})
			select {
			case ch <- batchDoneMsg{gen: gen}:
			case <-ctx.Done():
			}
			close(ch)
		}()
		return nil
	}

	return tea.Batch(start, waitFor(ch))
}

// nextRepoSeq returns a monotonically increasing request id for a repository.
// Status results carrying an older id are discarded, so a slow snapshot can
// never overwrite a newer one.
func (m *Model) nextRepoSeq(id domain.RepositoryID) int {
	if m.repoSeq == nil {
		m.repoSeq = make(map[domain.RepositoryID]int)
	}
	m.repoSeq[id]++
	return m.repoSeq[id]
}

func (m *Model) repoSeqCurrent(id domain.RepositoryID, seq int) bool {
	return seq != 0 && m.repoSeq[id] == seq
}

func waitFor(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *Model) applyStatus(state domain.RepositoryState) {
	for i := range m.rows {
		if m.rows[i].repo.ID != state.RepositoryID {
			continue
		}
		m.rows[i].state = state
		m.rows[i].loading = false
		if m.rows[i].busy && m.rows[i].busyLabel == busyRefreshing {
			m.rows[i].busy = false
			m.rows[i].busyLabel = ""
		}
		if m.screen == screenRepository && m.rowIndex == i {
			m.setChanges(buildChanges(state))
		}
		return
	}
}

// repoRowByID returns the row for a repository, if it is currently listed.
func (m *Model) repoRowByID(id domain.RepositoryID) *repoRow {
	for i := range m.rows {
		if m.rows[i].repo.ID == id {
			return &m.rows[i]
		}
	}
	return nil
}

// anyRepoBusy reports whether a write or post-write refresh is in progress.
func (m *Model) anyRepoBusy() bool {
	for _, row := range m.rows {
		if row.busy {
			return true
		}
	}
	return false
}

func (m *Model) loadingCount() int {
	n := 0
	for _, row := range m.rows {
		if row.loading {
			n++
		}
	}
	return n
}

// contentWidth is the usable width inside the horizontal margins.
func (m *Model) contentWidth() int {
	width := m.width - 2*contentMarginX
	if width < 20 {
		width = 20
	}
	return width
}
