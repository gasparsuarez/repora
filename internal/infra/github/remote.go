// Package github creates pull requests through the authenticated GitHub CLI.
package github

import (
	"fmt"
	"net/url"
	"strings"
)

// RemoteRepository is the owner/repository/host parsed from a Git remote URL.
type RemoteRepository struct {
	Host  string
	Owner string
	Name  string
}

// ParseRemoteURL parses the common GitHub remote URL forms:
//
//	git@github.com:owner/repo.git
//	ssh://git@github.com/owner/repo.git
//	https://github.com/owner/repo.git
func ParseRemoteURL(raw string) (RemoteRepository, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RemoteRepository{}, fmt.Errorf("empty remote URL")
	}

	var host, path string
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return RemoteRepository{}, fmt.Errorf("invalid remote URL %q: %w", raw, err)
		}
		host = u.Hostname()
		path = u.Path
	} else {
		// SCP-like syntax: [user@]host:path
		index := strings.Index(raw, ":")
		if index < 0 {
			return RemoteRepository{}, fmt.Errorf("not a network remote: %q", raw)
		}
		host = raw[:index]
		path = raw[index+1:]
		if at := strings.LastIndex(host, "@"); at >= 0 {
			host = host[at+1:]
		}
	}

	host = strings.ToLower(strings.TrimSpace(host))
	path = strings.Trim(strings.TrimSpace(path), "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")

	if host == "" {
		return RemoteRepository{}, fmt.Errorf("remote has no host: %q", raw)
	}

	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return RemoteRepository{}, fmt.Errorf("remote is not an owner/repository path: %q", raw)
	}

	return RemoteRepository{
		Host:  host,
		Owner: parts[0],
		Name:  parts[1],
	}, nil
}
