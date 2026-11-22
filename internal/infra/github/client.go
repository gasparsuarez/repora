package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gasparsuarez/repora/internal/domain"
)

// Runner executes the gh binary. It is replaceable in tests.
type Runner interface {
	Run(ctx context.Context, dir string, stdin []byte, args ...string) (stdout, stderr []byte, err error)
}

// CLIRunner runs the gh executable found on PATH.
type CLIRunner struct{}

// CommandError preserves the failing command, exit code, and output.
type CommandError struct {
	Args     []string
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
}

func (e *CommandError) Error() string {
	detail := strings.TrimSpace(e.Stderr)
	if detail == "" {
		detail = strings.TrimSpace(e.Stdout)
	}
	if detail == "" {
		detail = e.Err.Error()
	}
	return fmt.Sprintf("gh %s: exit code %d: %s", strings.Join(e.Args, " "), e.ExitCode, detail)
}

func (e *CommandError) Unwrap() error { return e.Err }

// Run implements Runner.
func (CLIRunner) Run(ctx context.Context, dir string, stdin []byte, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = ghEnvironment()
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return stdout.Bytes(), stderr.Bytes(), ctxErr
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout.Bytes(), stderr.Bytes(), &CommandError{
				Args:     args,
				ExitCode: exitErr.ExitCode(),
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				Err:      err,
			}
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("%w: %v", domain.ErrForgeUnavailable, err)
		}
		return stdout.Bytes(), stderr.Bytes(), err
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func ghEnvironment() []string {
	overrides := map[string]string{
		"GH_PROMPT_DISABLED":    "1",
		"GH_NO_UPDATE_NOTIFIER": "1",
		"NO_COLOR":              "1",
		"CLICOLOR":              "0",
		"GH_PAGER":              "cat",
		"PAGER":                 "cat",
	}

	base := os.Environ()
	out := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, overridden := overrides[key]; overridden {
				continue
			}
		}
		out = append(out, entry)
	}
	for key, value := range overrides {
		out = append(out, key+"="+value)
	}
	return out
}

// Client creates pull requests with `gh api`.
type Client struct {
	runner Runner
}

// NewClient returns a client. A nil runner uses the real gh binary.
func NewClient(runner Runner) *Client {
	if runner == nil {
		runner = CLIRunner{}
	}
	return &Client{runner: runner}
}

// CreatePullRequest implements domain.ForgePort.
func (c *Client) CreatePullRequest(ctx context.Context, req domain.PullRequestRequest) (domain.PullRequest, error) {
	remoteURL := req.Remote.PushURL
	if remoteURL == "" {
		remoteURL = req.Remote.FetchURL
	}
	remote, err := ParseRemoteURL(remoteURL)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("%w: %v", domain.ErrUnsupportedRemote, err)
	}

	payload := struct {
		Title string `json:"title"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Body  string `json:"body"`
	}{
		Title: req.Title,
		Head:  req.Head,
		Base:  req.Base,
		Body:  req.Body,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("encoding pull request: %w", err)
	}

	args := []string{
		"api",
		"--method", "POST",
		"--hostname", remote.Host,
		fmt.Sprintf("repos/%s/%s/pulls", remote.Owner, remote.Name),
		"--input", "-",
	}
	stdout, stderr, err := c.runner.Run(ctx, req.Repository.Path, body, args...)
	if err != nil {
		return domain.PullRequest{}, mapCommandError(ctx, stdout, stderr, err)
	}

	var response struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		Title   string `json:"title"`
		Head    struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}
	if err := json.Unmarshal(stdout, &response); err != nil {
		return domain.PullRequest{}, fmt.Errorf("%w: could not parse gh output: %v", domain.ErrForgeRejected, err)
	}
	if response.Number == 0 || response.HTMLURL == "" {
		return domain.PullRequest{}, fmt.Errorf("%w: gh did not return a pull request", domain.ErrForgeRejected)
	}

	return domain.PullRequest{
		Number: response.Number,
		URL:    response.HTMLURL,
		Title:  response.Title,
		Head:   response.Head.Ref,
		Base:   response.Base.Ref,
	}, nil
}

func mapCommandError(ctx context.Context, stdout, stderr []byte, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	msg := strings.ToLower(string(stderr) + "\n" + string(stdout) + "\n" + err.Error())
	switch {
	case errors.Is(err, domain.ErrForgeUnavailable),
		strings.Contains(msg, "executable file not found"),
		strings.Contains(msg, "command not found"):
		return fmt.Errorf("%w: install the GitHub CLI and run `gh auth login`", domain.ErrForgeUnavailable)
	case strings.Contains(msg, "gh auth login"),
		strings.Contains(msg, "not logged in"),
		strings.Contains(msg, "authentication required"),
		strings.Contains(msg, "bad credentials"),
		strings.Contains(msg, "http 401"),
		strings.Contains(msg, "http 403"):
		return fmt.Errorf("%w: run `gh auth login`", domain.ErrForgeAuth)
	default:
		detail := strings.TrimSpace(string(stderr))
		if detail == "" {
			detail = strings.TrimSpace(string(stdout))
		}
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("%w: %s", domain.ErrForgeRejected, detail)
	}
}
