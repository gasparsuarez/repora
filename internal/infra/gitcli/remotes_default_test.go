package gitcli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gasparsuarez/repora/internal/domain"
)

type remotesExec struct {
	symbolic string
	fail     bool
}

func (e *remotesExec) Execute(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
	switch {
	case len(args) > 0 && args[0] == "remote":
		return []byte("origin\tgit@github.com:acme/api.git (fetch)\norigin\tgit@github.com:acme/api.git (push)\n"), nil, nil
	case len(args) > 0 && args[0] == "symbolic-ref":
		if e.fail {
			return nil, []byte("not a symbolic ref"), errors.New("exit status 1")
		}
		return []byte(e.symbolic + "\n"), nil, nil
	}
	return nil, nil, errors.New("unexpected command: " + strings.Join(args, " "))
}

func TestRemotesResolveDefaultBranch(t *testing.T) {
	repo := NewRepository(&remotesExec{symbolic: "origin/main"})
	remotes, err := repo.Remotes(context.Background(), domain.Repository{ID: "api", Path: "/tmp/api"})
	if err != nil {
		t.Fatalf("Remotes: %v", err)
	}
	if len(remotes) != 1 || remotes[0].DefaultBranch != "main" {
		t.Fatalf("remotes = %+v", remotes)
	}
}

func TestRemotesMissingDefaultBranch(t *testing.T) {
	repo := NewRepository(&remotesExec{fail: true})
	remotes, err := repo.Remotes(context.Background(), domain.Repository{ID: "api", Path: "/tmp/api"})
	if err != nil {
		t.Fatalf("Remotes: %v", err)
	}
	if len(remotes) != 1 || remotes[0].DefaultBranch != "" {
		t.Fatalf("remotes = %+v", remotes)
	}
}
