package domain

import (
	"fmt"
	"strings"
)

const (
	maxNameLength = 80

	// DefaultStatusWorkers is the default size of the status refresh worker pool.
	DefaultStatusWorkers = 4

	// DefaultRefreshInterval is the default automatic refresh interval in seconds.
	DefaultRefreshInterval = 30

	// CurrentConfigVersion is the schema version this build reads and writes.
	CurrentConfigVersion = 1
)

// Repository is a local Git repository registered globally and referenced by
// workspaces.
type Repository struct {
	ID        RepositoryID `yaml:"id"`
	Name      string       `yaml:"name"`
	Path      string       `yaml:"path"`
	CreatedAt string       `yaml:"created_at,omitempty"`
}

// Validate checks the structural invariants of a repository entry.
func (r Repository) Validate() error {
	if err := r.ID.Validate(); err != nil {
		return err
	}
	if err := ValidateName(r.Name); err != nil {
		return fmt.Errorf("repository %q: %w", r.ID, err)
	}
	if strings.TrimSpace(r.Path) == "" {
		return fmt.Errorf("repository %q: path is empty", r.ID)
	}
	return nil
}

// Workspace is a logical grouping of repositories. It does not map to a
// filesystem directory.
type Workspace struct {
	ID            WorkspaceID    `yaml:"id"`
	Name          string         `yaml:"name"`
	Description   string         `yaml:"description,omitempty"`
	RepositoryIDs []RepositoryID `yaml:"repositories,omitempty"`
}

// Validate checks the structural invariants of a workspace entry.
func (w Workspace) Validate() error {
	if err := w.ID.Validate(); err != nil {
		return err
	}
	if err := ValidateName(w.Name); err != nil {
		return fmt.Errorf("workspace %q: %w", w.ID, err)
	}

	seen := make(map[RepositoryID]struct{}, len(w.RepositoryIDs))
	for _, id := range w.RepositoryIDs {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("workspace %q: %w", w.ID, err)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("workspace %q: %w: %q", w.ID, ErrDuplicateID, id)
		}
		seen[id] = struct{}{}
	}

	return nil
}

// Preferences holds user-configurable behavior.
type Preferences struct {
	Theme string `yaml:"theme,omitempty"`
	// RefreshInterval points at the automatic refresh interval in seconds.
	// A nil pointer means the key was absent and the default applies; an
	// explicit zero disables automatic refresh.
	RefreshInterval  *int        `yaml:"refresh_interval,omitempty"`
	StatusWorkers    int         `yaml:"status_workers,omitempty"`
	DefaultWorkspace WorkspaceID `yaml:"default_workspace,omitempty"`
}

// RefreshIntervalSeconds returns the effective automatic refresh interval.
func (p Preferences) RefreshIntervalSeconds() int {
	if p.RefreshInterval == nil {
		return DefaultRefreshInterval
	}
	return *p.RefreshInterval
}

// Normalized returns preferences with defaults applied and values clamped.
func (p Preferences) Normalized() Preferences {
	out := p
	if out.Theme == "" {
		out.Theme = "dark"
	}

	interval := DefaultRefreshInterval
	if out.RefreshInterval != nil {
		interval = *out.RefreshInterval
	}
	switch {
	case interval < 0:
		interval = 0
	case interval > 0 && interval < 5:
		interval = 5
	}
	out.RefreshInterval = &interval

	if out.StatusWorkers == 0 {
		out.StatusWorkers = DefaultStatusWorkers
	}
	if out.StatusWorkers < 1 {
		out.StatusWorkers = 1
	}
	if out.StatusWorkers > 16 {
		out.StatusWorkers = 16
	}
	return out
}

// Config is the persisted configuration aggregate.
type Config struct {
	Version      int          `yaml:"version"`
	Repositories []Repository `yaml:"repositories"`
	Workspaces   []Workspace  `yaml:"workspaces"`
	Preferences  Preferences  `yaml:"preferences"`
}

// Validate checks every structural invariant of the configuration.
func (c *Config) Validate() error {
	if c.Version == 0 {
		c.Version = CurrentConfigVersion
	}
	if c.Version != CurrentConfigVersion {
		return fmt.Errorf("%w: %d (supported: %d)", ErrUnsupportedVersion, c.Version, CurrentConfigVersion)
	}

	repoByID := make(map[RepositoryID]struct{}, len(c.Repositories))
	seenPaths := make(map[string]RepositoryID, len(c.Repositories))
	for _, repo := range c.Repositories {
		if err := repo.Validate(); err != nil {
			return err
		}
		if _, ok := repoByID[repo.ID]; ok {
			return fmt.Errorf("%w: repository %q", ErrDuplicateID, repo.ID)
		}
		repoByID[repo.ID] = struct{}{}

		if other, ok := seenPaths[repo.Path]; ok {
			return fmt.Errorf("%w: %q used by %q and %q", ErrDuplicatePath, repo.Path, other, repo.ID)
		}
		seenPaths[repo.Path] = repo.ID
	}

	wsByID := make(map[WorkspaceID]struct{}, len(c.Workspaces))
	for _, ws := range c.Workspaces {
		if err := ws.Validate(); err != nil {
			return err
		}
		if _, ok := wsByID[ws.ID]; ok {
			return fmt.Errorf("%w: workspace %q", ErrDuplicateID, ws.ID)
		}
		wsByID[ws.ID] = struct{}{}

		for _, repoID := range ws.RepositoryIDs {
			if _, ok := repoByID[repoID]; !ok {
				return fmt.Errorf("%w: workspace %q -> %q", ErrDanglingReference, ws.ID, repoID)
			}
		}
	}

	if p := c.Preferences.DefaultWorkspace; p != "" {
		if _, ok := wsByID[p]; !ok {
			return fmt.Errorf("%w: default_workspace %q", ErrDanglingReference, p)
		}
	}

	c.Preferences = c.Preferences.Normalized()
	return nil
}

// RepositoryByID resolves a registered repository.
func (c *Config) RepositoryByID(id RepositoryID) (Repository, bool) {
	for _, repo := range c.Repositories {
		if repo.ID == id {
			return repo, true
		}
	}
	return Repository{}, false
}

// WorkspaceByID resolves a workspace.
func (c *Config) WorkspaceByID(id WorkspaceID) (Workspace, bool) {
	for _, ws := range c.Workspaces {
		if ws.ID == id {
			return ws, true
		}
	}
	return Workspace{}, false
}

// RepositoriesOf returns the repositories associated with a workspace, in the
// order defined by the workspace.
func (c *Config) RepositoriesOf(ws Workspace) []Repository {
	out := make([]Repository, 0, len(ws.RepositoryIDs))
	for _, id := range ws.RepositoryIDs {
		if repo, ok := c.RepositoryByID(id); ok {
			out = append(out, repo)
		}
	}
	return out
}

// ValidateName checks a display name.
func ValidateName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%w: name is empty", ErrInvalidName)
	}
	if len([]rune(trimmed)) > maxNameLength {
		return fmt.Errorf("%w: %d characters (max %d)", ErrInvalidName, len([]rune(trimmed)), maxNameLength)
	}
	return nil
}
