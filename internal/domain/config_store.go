package domain

// ConfigStore loads and persists the configuration aggregate.
type ConfigStore interface {
	// Load reads the configuration. Non-fatal problems are returned as warnings.
	Load() (*Config, []string, error)

	// Save validates and persists the configuration atomically.
	Save(*Config) error
}

// Clone returns a deep copy safe to mutate on a single goroutine without racing
// with readers of the original.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	out := *c
	out.Repositories = append([]Repository(nil), c.Repositories...)

	out.Workspaces = make([]Workspace, len(c.Workspaces))
	for i, ws := range c.Workspaces {
		copied := ws
		copied.RepositoryIDs = append([]RepositoryID(nil), ws.RepositoryIDs...)
		out.Workspaces[i] = copied
	}

	if c.Preferences.RefreshInterval != nil {
		interval := *c.Preferences.RefreshInterval
		out.Preferences.RefreshInterval = &interval
	}

	return &out
}
