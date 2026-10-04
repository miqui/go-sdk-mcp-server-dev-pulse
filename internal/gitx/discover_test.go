package gitx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRepos(t *testing.T) {
	root := t.TempDir()
	mustMkGitDir(t, filepath.Join(root, "alpha"))
	mustMkGitDir(t, filepath.Join(root, "beta-service"))
	if err := os.MkdirAll(filepath.Join(root, "not-a-repo"), 0o755); err != nil {
		t.Fatalf("mkdir not-a-repo: %v", err)
	}

	tests := []struct {
		name       string
		nameFilter string
		want       []string
	}{
		{name: "no filter", nameFilter: "", want: []string{"alpha", "beta-service"}},
		{name: "filter matches one", nameFilter: "beta", want: []string{"beta-service"}},
		{name: "filter matches none", nameFilter: "zzz", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repos, err := DiscoverRepos(root, tt.nameFilter)
			if err != nil {
				t.Fatalf("DiscoverRepos: unexpected error: %v", err)
			}
			var got []string
			for _, r := range repos {
				got = append(got, r.Name)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("DiscoverRepos(filter=%q) = %v, want %v", tt.nameFilter, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("DiscoverRepos(filter=%q)[%d] = %q, want %q", tt.nameFilter, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func mustMkGitDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		t.Fatalf("mustMkGitDir(%q): %v", path, err)
	}
}
