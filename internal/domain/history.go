package domain

import "time"

// Branch is a local or remote branch snapshot.
type Branch struct {
	Name        string
	IsCurrent   bool
	IsRemote    bool
	Upstream    string
	HasUpstream bool
	Gone        bool // upstream was deleted
	Ahead       int
	Behind      int
	LastCommit  Commit
}

// Commit is a single history entry.
type Commit struct {
	Hash    string
	Short   string
	Subject string
	Author  string
	Email   string
	Date    time.Time
	Refs    []string
}

// Remote is a configured Git remote.
type Remote struct {
	Name          string
	FetchURL      string
	PushURL       string
	DefaultBranch string // from refs/remotes/<name>/HEAD, when available
}

// DiffTarget selects which side of the working tree a diff describes.
type DiffTarget string

// Diff targets.
const (
	DiffUnstaged DiffTarget = "unstaged"
	DiffStaged   DiffTarget = "staged"
	DiffCommit   DiffTarget = "commit"
)
