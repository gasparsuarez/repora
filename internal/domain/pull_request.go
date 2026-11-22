package domain

import (
	"context"
	"fmt"
	"strings"
)

// PullRequest is the result of creating a pull request.
type PullRequest struct {
	Number int
	URL    string
	Title  string
	Head   string
	Base   string
}

// PullRequestRequest describes the pull request to create.
type PullRequestRequest struct {
	Repository Repository
	Remote     Remote
	Head       string
	Base       string
	Title      string
	Body       string
}

// ForgePort creates pull requests on a hosting provider.
type ForgePort interface {
	CreatePullRequest(ctx context.Context, req PullRequestRequest) (PullRequest, error)
}

// ValidatePullRequest checks the provider-independent fields.
func ValidatePullRequest(title, head, base string) error {
	title = strings.TrimSpace(title)
	head = strings.TrimSpace(head)
	base = strings.TrimSpace(base)

	if title == "" {
		return fmt.Errorf("%w: title is empty", ErrInvalidPullRequest)
	}
	if len([]rune(title)) > 256 {
		return fmt.Errorf("%w: title is longer than 256 characters", ErrInvalidPullRequest)
	}
	if err := ValidateBranchName(head); err != nil {
		return fmt.Errorf("%w: head: %v", ErrInvalidPullRequest, err)
	}
	if err := ValidateBranchName(base); err != nil {
		return fmt.Errorf("%w: base: %v", ErrInvalidPullRequest, err)
	}
	if head == base {
		return fmt.Errorf("%w: head and base are the same branch", ErrInvalidPullRequest)
	}
	return nil
}
