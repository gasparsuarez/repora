package github

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gasparsuarez/repora/internal/domain"
)

type fakeRunner struct {
	stdout []byte
	stderr []byte
	err    error

	args  []string
	dir   string
	stdin []byte
}

func (f *fakeRunner) Run(_ context.Context, dir string, stdin []byte, args ...string) ([]byte, []byte, error) {
	f.dir = dir
	f.stdin = append([]byte(nil), stdin...)
	f.args = append([]string(nil), args...)
	return f.stdout, f.stderr, f.err
}

func testRequest() domain.PullRequestRequest {
	return domain.PullRequestRequest{
		Repository: domain.Repository{ID: "api", Name: "API", Path: "/tmp/api"},
		Remote:     domain.Remote{Name: "origin", FetchURL: "git@github.com:acme/api.git"},
		Head:       "feature/retry",
		Base:       "main",
		Title:      "Add retry logic",
		Body:       "Retries failed calls.",
	}
}

func TestCreatePullRequest(t *testing.T) {
	runner := &fakeRunner{stdout: []byte(`{"number":42,"html_url":"https://github.com/acme/api/pull/42","title":"Add retry logic","head":{"ref":"feature/retry"},"base":{"ref":"main"}}`)}
	client := NewClient(runner)

	pr, err := client.CreatePullRequest(context.Background(), testRequest())
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if pr.Number != 42 || pr.URL != "https://github.com/acme/api/pull/42" {
		t.Fatalf("pr = %+v", pr)
	}
	if runner.dir != "/tmp/api" {
		t.Errorf("dir = %q", runner.dir)
	}
	wantArgs := []string{"api", "--method", "POST", "--hostname", "github.com", "repos/acme/api/pulls", "--input", "-"}
	if strings.Join(runner.args, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("args = %v, want %v", runner.args, wantArgs)
	}

	var payload map[string]string
	if err := json.Unmarshal(runner.stdin, &payload); err != nil {
		t.Fatalf("stdin is not JSON: %v\n%s", err, runner.stdin)
	}
	if payload["title"] != "Add retry logic" || payload["head"] != "feature/retry" ||
		payload["base"] != "main" || payload["body"] != "Retries failed calls." {
		t.Errorf("payload = %+v", payload)
	}
}

func TestCreatePullRequestAuthenticationError(t *testing.T) {
	runner := &fakeRunner{
		stderr: []byte("To get started with GitHub CLI, please run: gh auth login"),
		err:    errors.New("exit status 1"),
	}
	_, err := NewClient(runner).CreatePullRequest(context.Background(), testRequest())
	if !errors.Is(err, domain.ErrForgeAuth) {
		t.Fatalf("err = %v, want ErrForgeAuth", err)
	}
}

func TestCreatePullRequestUnavailable(t *testing.T) {
	runner := &fakeRunner{err: &exec.Error{Name: "gh", Err: exec.ErrNotFound}}
	_, err := NewClient(runner).CreatePullRequest(context.Background(), testRequest())
	if !errors.Is(err, domain.ErrForgeUnavailable) {
		t.Fatalf("err = %v, want ErrForgeUnavailable", err)
	}
}

func TestCreatePullRequestRejected(t *testing.T) {
	runner := &fakeRunner{
		stderr: []byte(`{"message":"Validation Failed","errors":[{"message":"A pull request already exists"}]}`),
		err:    errors.New("exit status 1"),
	}
	_, err := NewClient(runner).CreatePullRequest(context.Background(), testRequest())
	if !errors.Is(err, domain.ErrForgeRejected) {
		t.Fatalf("err = %v, want ErrForgeRejected", err)
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error does not preserve gh output: %v", err)
	}
}

func TestCreatePullRequestUnsupportedRemote(t *testing.T) {
	req := testRequest()
	req.Remote.FetchURL = "/tmp/bare.git"
	_, err := NewClient(&fakeRunner{}).CreatePullRequest(context.Background(), req)
	if !errors.Is(err, domain.ErrUnsupportedRemote) {
		t.Fatalf("err = %v, want ErrUnsupportedRemote", err)
	}
}

func TestCLIRunnerExecutesGH(t *testing.T) {
	binDir := t.TempDir()
	script := filepath.Join(binDir, "gh")
	if err := os.WriteFile(script, []byte(`#!/bin/sh
cat >/dev/null
printf '{"number":9,"html_url":"https://github.com/acme/api/pull/9","title":"Add retry","head":{"ref":"feature/retry"},"base":{"ref":"main"}}'
`), 0o755); err != nil {
		t.Fatalf("write gh stub: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	req := testRequest()
	req.Repository.Path = t.TempDir()
	pr, err := NewClient(nil).CreatePullRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("CreatePullRequest: %v", err)
	}
	if pr.Number != 9 || pr.URL != "https://github.com/acme/api/pull/9" {
		t.Fatalf("pr = %+v", pr)
	}
}
