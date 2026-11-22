package gitcli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gasparsuarez/repora/internal/domain"
)

type refExec struct {
	stdout []byte
	stderr []byte
	err    error
	args   []string
}

func (e *refExec) Execute(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
	e.args = append([]string(nil), args...)
	return e.stdout, e.stderr, e.err
}

func TestRefOID(t *testing.T) {
	ex := &refExec{stdout: []byte("abcdef1234567890\n")}
	oid, err := NewRepository(ex).RefOID(context.Background(), domain.Repository{Path: "/tmp/api"}, "refs/heads/main")
	if err != nil {
		t.Fatalf("RefOID: %v", err)
	}
	if oid != "abcdef1234567890" {
		t.Fatalf("oid = %q", oid)
	}
	if got := strings.Join(ex.args, " "); got != "rev-parse --verify --quiet refs/heads/main" {
		t.Errorf("args = %q", got)
	}
}

func TestRefOIDErrorPreservesOutput(t *testing.T) {
	ex := &refExec{stderr: []byte("fatal: bad revision"), err: errors.New("exit status 128")}
	_, err := NewRepository(ex).RefOID(context.Background(), domain.Repository{Path: "/tmp/api"}, "refs/heads/missing")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "bad revision") {
		t.Errorf("error does not preserve stderr: %v", err)
	}
}
