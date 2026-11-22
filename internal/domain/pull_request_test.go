package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidatePullRequest(t *testing.T) {
	if err := ValidatePullRequest("Add retry", "feature/retry", "main"); err != nil {
		t.Fatalf("valid request: %v", err)
	}

	for name, tt := range map[string]struct {
		title, head, base string
	}{
		"empty title": {" ", "feature/x", "main"},
		"same branch": {"Title", "main", "main"},
		"bad head":    {"Title", "bad..head", "main"},
		"bad base":    {"Title", "feature/x", "bad..base"},
		"long title":  {strings.Repeat("x", 257), "feature/x", "main"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidatePullRequest(tt.title, tt.head, tt.base); !errors.Is(err, ErrInvalidPullRequest) {
				t.Fatalf("err = %v, want ErrInvalidPullRequest", err)
			}
		})
	}
}
