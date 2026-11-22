package github

import "testing"

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want RemoteRepository
	}{
		{"scp", "git@github.com:acme/api.git", RemoteRepository{Host: "github.com", Owner: "acme", Name: "api"}},
		{"https", "https://github.com/acme/api.git", RemoteRepository{Host: "github.com", Owner: "acme", Name: "api"}},
		{"https no git", "https://github.com/acme/api", RemoteRepository{Host: "github.com", Owner: "acme", Name: "api"}},
		{"ssh", "ssh://git@github.com/acme/api.git", RemoteRepository{Host: "github.com", Owner: "acme", Name: "api"}},
		{"ssh port", "ssh://git@github.com:22/acme/api.git", RemoteRepository{Host: "github.com", Owner: "acme", Name: "api"}},
		{"enterprise", "git@github.example.com:team/api.git", RemoteRepository{Host: "github.example.com", Owner: "team", Name: "api"}},
		{"trailing slash", "https://github.com/acme/api/", RemoteRepository{Host: "github.com", Owner: "acme", Name: "api"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRemoteURL(tt.raw)
			if err != nil {
				t.Fatalf("ParseRemoteURL(%q): %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseRemoteURLRejectsUnsupported(t *testing.T) {
	for _, raw := range []string{
		"",
		"/tmp/repo.git",
		"../repo",
		"https://github.com/acme",
		"https://github.com/acme/api/extra",
	} {
		if _, err := ParseRemoteURL(raw); err == nil {
			t.Errorf("ParseRemoteURL(%q) succeeded, want error", raw)
		}
	}
}
