package domain

import "context"

// GitPort is the read-only Git contract required by application services.
// It is implemented by the infrastructure Git adapter.
type GitPort interface {
	// Status returns a snapshot of the repository state. Failures are reported
	// inside the returned state so partial information can still be rendered.
	Status(ctx context.Context, repo Repository) RepositoryState

	// Root validates that path is a Git working tree and returns its canonical
	// repository root.
	Root(ctx context.Context, path string) (string, error)

	// Branches lists local and remote branches.
	Branches(ctx context.Context, repo Repository) ([]Branch, error)

	// Log returns the most recent commits, newest first.
	Log(ctx context.Context, repo Repository, limit int) ([]Commit, error)

	// Remotes lists configured remotes.
	Remotes(ctx context.Context, repo Repository) ([]Remote, error)

	// RefOID resolves a fully qualified ref to its object id.
	RefOID(ctx context.Context, repo Repository, ref string) (string, error)

	// Diff returns a text diff for a single path, or for the whole worktree when
	// path is empty.
	Diff(ctx context.Context, repo Repository, target DiffTarget, path string) (string, error)

	// ShowCommit returns the patch and stat for a single commit.
	ShowCommit(ctx context.Context, repo Repository, hash string) (string, error)
}
