package app

import (
	"context"
	"errors"
	"testing"

	"github.com/gasparsuarez/repora/internal/domain"
)

type fakePRGit struct {
	remotes    []domain.Remote
	remotesErr error
	refs       map[string]string
}

func (f *fakePRGit) Status(context.Context, domain.Repository) domain.RepositoryState {
	return domain.RepositoryState{}
}
func (f *fakePRGit) Root(context.Context, string) (string, error) { return "", nil }
func (f *fakePRGit) Branches(context.Context, domain.Repository) ([]domain.Branch, error) {
	return nil, nil
}
func (f *fakePRGit) Log(context.Context, domain.Repository, int) ([]domain.Commit, error) {
	return nil, nil
}
func (f *fakePRGit) Remotes(context.Context, domain.Repository) ([]domain.Remote, error) {
	return f.remotes, f.remotesErr
}
func (f *fakePRGit) RefOID(_ context.Context, _ domain.Repository, ref string) (string, error) {
	oid, ok := f.refs[ref]
	if !ok {
		return "", errors.New("ref not found")
	}
	return oid, nil
}
func (f *fakePRGit) Diff(context.Context, domain.Repository, domain.DiffTarget, string) (string, error) {
	return "", nil
}
func (f *fakePRGit) ShowCommit(context.Context, domain.Repository, string) (string, error) {
	return "", nil
}

type fakeForge struct {
	req    domain.PullRequestRequest
	result domain.PullRequest
	err    error
}

func (f *fakeForge) CreatePullRequest(_ context.Context, req domain.PullRequestRequest) (domain.PullRequest, error) {
	f.req = req
	return f.result, f.err
}

func pullRequestFixture() (*fakePRGit, *fakeForge) {
	git := &fakePRGit{
		remotes: []domain.Remote{{Name: "origin", FetchURL: "git@github.com:acme/api.git"}},
		refs: map[string]string{
			"refs/remotes/origin/main":          "aaa",
			"refs/remotes/origin/feature/retry": "bbb",
			"refs/heads/feature/retry":          "bbb",
		},
	}
	forge := &fakeForge{result: domain.PullRequest{Number: 7, URL: "https://github.com/acme/api/pull/7"}}
	return git, forge
}

var prRepo = domain.Repository{ID: "api", Name: "API", Path: "/tmp/api"}

func TestCreatePullRequest(t *testing.T) {
	git, forge := pullRequestFixture()
	svc := NewPullRequestService(git, forge)

	pr, err := svc.Create(context.Background(), prRepo, "origin", "feature/retry", "main", "Add retry", "Body")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if pr.Number != 7 {
		t.Fatalf("pr = %+v", pr)
	}
	if forge.req.Remote.Name != "origin" || forge.req.Head != "feature/retry" || forge.req.Base != "main" {
		t.Errorf("request = %+v", forge.req)
	}
}

func TestCreatePullRequestValidation(t *testing.T) {
	git, forge := pullRequestFixture()
	svc := NewPullRequestService(git, forge)

	_, err := svc.Create(context.Background(), prRepo, "origin", "feature/retry", "main", " ", "Body")
	if !errors.Is(err, domain.ErrInvalidPullRequest) {
		t.Fatalf("err = %v, want ErrInvalidPullRequest", err)
	}
}

func TestCreatePullRequestNoRemote(t *testing.T) {
	git, forge := pullRequestFixture()
	git.remotes = nil
	_, err := NewPullRequestService(git, forge).Create(context.Background(), prRepo, "origin", "feature/retry", "main", "Title", "")
	if !errors.Is(err, domain.ErrNoRemote) {
		t.Fatalf("err = %v, want ErrNoRemote", err)
	}
}

func TestCreatePullRequestUnknownRemote(t *testing.T) {
	git, forge := pullRequestFixture()
	_, err := NewPullRequestService(git, forge).Create(context.Background(), prRepo, "upstream", "feature/retry", "main", "Title", "")
	if !errors.Is(err, domain.ErrRemoteNotFound) {
		t.Fatalf("err = %v, want ErrRemoteNotFound", err)
	}
}

func TestCreatePullRequestBaseMissing(t *testing.T) {
	git, forge := pullRequestFixture()
	_, err := NewPullRequestService(git, forge).Create(context.Background(), prRepo, "origin", "feature/retry", "develop", "Title", "")
	if !errors.Is(err, domain.ErrBaseBranchNotFound) {
		t.Fatalf("err = %v, want ErrBaseBranchNotFound", err)
	}
}

func TestCreatePullRequestBranchNotPushed(t *testing.T) {
	git, forge := pullRequestFixture()
	_, err := NewPullRequestService(git, forge).Create(context.Background(), prRepo, "origin", "feature/missing", "main", "Title", "")
	if !errors.Is(err, domain.ErrBranchNotPushed) {
		t.Fatalf("err = %v, want ErrBranchNotPushed", err)
	}

	git.refs["refs/remotes/origin/feature/retry"] = "ccc"
	_, err = NewPullRequestService(git, forge).Create(context.Background(), prRepo, "origin", "feature/retry", "main", "Title", "")
	if !errors.Is(err, domain.ErrBranchNotPushed) {
		t.Fatalf("err = %v, want ErrBranchNotPushed for diverged local branch", err)
	}
}
