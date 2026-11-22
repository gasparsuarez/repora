package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/gasparsuarez/repora/internal/domain"
)

// PullRequestService validates pull request requests against the local Git
// state and delegates creation to the configured forge.
type PullRequestService struct {
	git   domain.GitPort
	forge domain.ForgePort
}

// NewPullRequestService returns a service backed by Git reads and a forge.
func NewPullRequestService(git domain.GitPort, forge domain.ForgePort) *PullRequestService {
	return &PullRequestService{git: git, forge: forge}
}

// Create validates the head/base branches and creates the pull request.
func (s *PullRequestService) Create(ctx context.Context, repo domain.Repository, remoteName, head, base, title, body string) (domain.PullRequest, error) {
	if err := domain.ValidatePullRequest(title, head, base); err != nil {
		return domain.PullRequest{}, err
	}
	if s.git == nil {
		return domain.PullRequest{}, fmt.Errorf("%w: Git reads are unavailable", domain.ErrUnavailable)
	}
	if s.forge == nil {
		return domain.PullRequest{}, fmt.Errorf("%w: install the GitHub CLI and run `gh auth login`", domain.ErrForgeUnavailable)
	}

	remotes, err := s.git.Remotes(ctx, repo)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if len(remotes) == 0 {
		return domain.PullRequest{}, domain.ErrNoRemote
	}

	var remote domain.Remote
	found := false
	for _, candidate := range remotes {
		if candidate.Name == remoteName {
			remote = candidate
			found = true
			break
		}
	}
	if !found {
		return domain.PullRequest{}, fmt.Errorf("%w: %q", domain.ErrRemoteNotFound, remoteName)
	}

	if _, err := s.git.RefOID(ctx, repo, "refs/remotes/"+remoteName+"/"+base); err != nil {
		return domain.PullRequest{}, fmt.Errorf("%w: %s/%s: %v", domain.ErrBaseBranchNotFound, remoteName, base, err)
	}

	remoteOID, err := s.git.RefOID(ctx, repo, "refs/remotes/"+remoteName+"/"+head)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("%w: %s/%s: %v", domain.ErrBranchNotPushed, remoteName, head, err)
	}
	if localOID, err := s.git.RefOID(ctx, repo, "refs/heads/"+head); err == nil && localOID != remoteOID {
		return domain.PullRequest{}, fmt.Errorf("%w: local %s and %s/%s point to different commits", domain.ErrBranchNotPushed, head, remoteName, head)
	}

	return s.forge.CreatePullRequest(ctx, domain.PullRequestRequest{
		Repository: repo,
		Remote:     remote,
		Head:       strings.TrimSpace(head),
		Base:       strings.TrimSpace(base),
		Title:      strings.TrimSpace(title),
		Body:       body,
	})
}
