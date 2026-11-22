package app

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gasparsuarez/repora/internal/domain"
)

type fakeGit struct {
	delay   time.Duration
	mu      sync.Mutex
	running int
	maxRun  int
	calls   []domain.RepositoryID
}

func (f *fakeGit) Status(_ context.Context, repo domain.Repository) domain.RepositoryState {
	f.mu.Lock()
	f.running++
	if f.running > f.maxRun {
		f.maxRun = f.running
	}
	f.calls = append(f.calls, repo.ID)
	f.mu.Unlock()

	time.Sleep(f.delay)

	f.mu.Lock()
	f.running--
	f.mu.Unlock()

	return domain.RepositoryState{
		RepositoryID: repo.ID,
		Branch:       domain.BranchState{Head: "main"},
	}
}

func (f *fakeGit) Root(_ context.Context, path string) (string, error) { return path, nil }

func (f *fakeGit) Branches(_ context.Context, repo domain.Repository) ([]domain.Branch, error) {
	return nil, nil
}

func (f *fakeGit) Log(_ context.Context, repo domain.Repository, limit int) ([]domain.Commit, error) {
	return nil, nil
}

func (f *fakeGit) Remotes(_ context.Context, repo domain.Repository) ([]domain.Remote, error) {
	return nil, nil
}

func (f *fakeGit) RefOID(_ context.Context, repo domain.Repository, ref string) (string, error) {
	return ref, nil
}

func (f *fakeGit) Diff(_ context.Context, repo domain.Repository, target domain.DiffTarget, path string) (string, error) {
	return "", nil
}

func (f *fakeGit) ShowCommit(_ context.Context, repo domain.Repository, hash string) (string, error) {
	return "", nil
}

func repos(n int) []domain.Repository {
	out := make([]domain.Repository, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, domain.Repository{
			ID:   domain.RepositoryID(string(rune('a' + i))),
			Name: string(rune('A' + i)),
			Path: "/tmp/x",
		})
	}
	return out
}

func TestRefreshRespectsWorkerLimitAndDeliversAll(t *testing.T) {
	git := &fakeGit{delay: 10 * time.Millisecond}
	svc := NewStatusService(git, 4)

	var mu sync.Mutex
	seen := make(map[domain.RepositoryID]int)
	list := repos(12)

	svc.Refresh(context.Background(), list, func(state domain.RepositoryState) {
		mu.Lock()
		seen[state.RepositoryID]++
		mu.Unlock()
	})

	if len(seen) != len(list) {
		t.Fatalf("delivered %d of %d", len(seen), len(list))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("%s delivered %d times", id, n)
		}
	}

	git.mu.Lock()
	maxRun := git.maxRun
	git.mu.Unlock()
	if maxRun > 4 {
		t.Errorf("max concurrency = %d, want <= 4", maxRun)
	}
	if maxRun < 2 {
		t.Errorf("max concurrency = %d, expected parallelism", maxRun)
	}
}

func TestRefreshCancellationStopsWork(t *testing.T) {
	git := &fakeGit{delay: 50 * time.Millisecond}
	svc := NewStatusService(git, 2)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var mu sync.Mutex
	count := 0
	svc.Refresh(ctx, repos(8), func(domain.RepositoryState) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	if count != 0 {
		t.Errorf("emitted %d states after cancellation, want 0", count)
	}
}

func TestRefreshEmptyWorkspace(t *testing.T) {
	svc := NewStatusService(&fakeGit{}, 4)
	called := false
	svc.Refresh(context.Background(), nil, func(domain.RepositoryState) { called = true })
	if called {
		t.Error("emit called for empty repository list")
	}
}

func TestWorkersClamped(t *testing.T) {
	if got := NewStatusService(&fakeGit{}, 0).Workers(); got != 1 {
		t.Errorf("workers = %d, want 1", got)
	}
	if got := NewStatusService(&fakeGit{}, 500).Workers(); got != 16 {
		t.Errorf("workers = %d, want 16", got)
	}
}

func TestRefreshOrderIndependence(t *testing.T) {
	// Completion order is not guaranteed; ensure every repository is reported
	// exactly once regardless of order.
	git := &fakeGit{}
	svc := NewStatusService(git, 4)

	var mu sync.Mutex
	var got []string
	svc.Refresh(context.Background(), repos(5), func(state domain.RepositoryState) {
		mu.Lock()
		got = append(got, string(state.RepositoryID))
		mu.Unlock()
	})

	sort.Strings(got)
	want := []string{"a", "b", "c", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
