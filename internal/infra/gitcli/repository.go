package gitcli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gasparsuarez/repora/internal/domain"
)

// ReadTimeout bounds local read operations.
const ReadTimeout = 10 * time.Second

// Repository is the Git adapter. It implements the read side of the domain's
// Git port.
type Repository struct {
	ex Executor
}

// NewRepository returns an adapter backed by ex.
func NewRepository(ex Executor) *Repository { return &Repository{ex: ex} }

// Root validates that path is a Git working tree and returns its canonical root.
func (r *Repository) Root(ctx context.Context, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %v", domain.ErrInvalidPath, path, err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}

	ctx, cancel := context.WithTimeout(ctx, ReadTimeout)
	defer cancel()

	inside, stderr, err := r.ex.Execute(ctx, abs, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return "", fmt.Errorf("%w: %s: %s", domain.ErrNotARepository, abs, strings.TrimSpace(string(stderr)))
	}
	if strings.TrimSpace(string(inside)) != "true" {
		return "", fmt.Errorf("%w: %s", domain.ErrBareRepository, abs)
	}

	top, stderr, err := r.ex.Execute(ctx, abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%w: %s: %s", domain.ErrNotARepository, abs, strings.TrimSpace(string(stderr)))
	}

	root := strings.TrimSpace(string(top))
	if root == "" {
		return "", fmt.Errorf("%w: %s: empty repository root", domain.ErrNotARepository, abs)
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return root, nil
}

// Status reads and parses the repository state. Errors are reported inside the
// returned state rather than as a second return value, so partial information
// (for example an unavailable path) can still be rendered.
func (r *Repository) Status(ctx context.Context, repo domain.Repository) domain.RepositoryState {
	state := domain.RepositoryState{
		RepositoryID: repo.ID,
		FetchedAt:    time.Now(),
	}

	ctx, cancel := context.WithTimeout(ctx, ReadTimeout)
	defer cancel()

	stdout, stderr, err := r.ex.Execute(ctx, repo.Path,
		"-c", "core.quotepath=false",
		"status", "--porcelain=v2", "--branch", "--untracked-files=all", "-z",
	)
	if err != nil {
		state.Err = fmt.Errorf("git status in %s: %w", repo.Path, exitSummary(err, stderr))
		return state
	}

	parsed := ParseStatus(repo.ID, stdout)
	parsed.FetchedAt = state.FetchedAt
	parsed.Operation = r.Operation(ctx, repo.Path)
	return parsed
}

// RefOID resolves a fully qualified ref to its object id.
func (r *Repository) RefOID(ctx context.Context, repo domain.Repository, ref string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, ReadTimeout)
	defer cancel()

	stdout, stderr, err := r.ex.Execute(ctx, repo.Path, "rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		return "", fmt.Errorf("resolving %s in %s: %w", ref, repo.Path, exitSummary(err, stderr))
	}
	oid := strings.TrimSpace(string(stdout))
	if oid == "" {
		return "", fmt.Errorf("resolving %s in %s: empty object id", ref, repo.Path)
	}
	return oid, nil
}

// IsRepository reports whether path is a usable Git working tree.
func (r *Repository) IsRepository(ctx context.Context, path string) bool {
	ctx, cancel := context.WithTimeout(ctx, ReadTimeout)
	defer cancel()
	out, _, err := r.ex.Execute(ctx, path, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func exitSummary(err error, stderr []byte) error {
	if exitErr, ok := err.(*ExitError); ok {
		return exitErr
	}
	msg := strings.TrimSpace(string(stderr))
	if msg != "" {
		return fmt.Errorf("%w: %s", err, msg)
	}
	return err
}
