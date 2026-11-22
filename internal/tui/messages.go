package tui

import (
	"github.com/gasparsuarez/repora/internal/app"
	"github.com/gasparsuarez/repora/internal/domain"
)

// statusMsg carries one repository snapshot produced by a refresh. seq is the
// per-repository request id used to discard out-of-order results.
type statusMsg struct {
	gen   int
	seq   int
	state domain.RepositoryState
}

// refreshTickMsg fires when the automatic refresh interval elapses.
type refreshTickMsg struct {
	seq int
}

// homeStatusMsg carries one repository snapshot for the explorer.
type homeStatusMsg struct {
	gen   int
	state domain.RepositoryState
}

// homeDoneMsg signals that the explorer refresh finished.
type homeDoneMsg struct {
	gen int
}

// batchDoneMsg signals that a refresh batch finished.
type batchDoneMsg struct {
	gen int
}

// editorClosedMsg reports that the external editor exited.
type editorClosedMsg struct {
	err error
}

// configReloadedMsg carries the result of re-reading the configuration.
type configReloadedMsg struct {
	cfg      *domain.Config
	warnings []string
	status   *app.StatusService
	err      error
}

// Reloader re-reads the configuration from disk and rebuilds derived services.
type Reloader func() (*domain.Config, []string, *app.StatusService, error)

// detailMsg carries branches, history, and remotes for the repository screen.
type detailMsg struct {
	gen    int
	repoID domain.RepositoryID
	detail app.RepositoryDetail
}

// commitDetailMsg carries the patch for a selected commit.
type commitDetailMsg struct {
	gen     int
	content string
	err     error
}

// diffMsg carries a rendered diff.
type diffMsg struct {
	gen     int
	title   string
	content string
	err     error
}

// writeDoneMsg reports the result of a mutating Git operation.
type writeDoneMsg struct {
	label  string
	repoID domain.RepositoryID
	err    error
}

// pullRequestDoneMsg reports the result of creating a pull request.
type pullRequestDoneMsg struct {
	gen    int
	repoID domain.RepositoryID
	pr     domain.PullRequest
	err    error
}

// noticeExpiredMsg clears a success notice after its display window.
type noticeExpiredMsg struct {
	seq int
}

// configChangedMsg reports the result of a workspace or registration change.
type configChangedMsg struct {
	action   string
	cfg      *domain.Config
	selectWS domain.WorkspaceID
	err      error
}
