package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/execx"
)

// fakeRunner implements execx.Runner by matching on command name/args
// prefix, returning canned output offline.
type fakeRunner struct {
	responses map[string]string // key: "name args..." joined by space
}

func (f fakeRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	if out, ok := f.responses[key]; ok {
		return out, nil
	}
	return "", &execx.RunError{Name: name, Args: args, Output: "no fake response configured", Err: os.ErrNotExist}
}

func TestRepoStatusOffline(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "myrepo")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("setting up fake repo dir: %v", err)
	}

	runner := fakeRunner{responses: map[string]string{
		"git status --porcelain=v2 --branch": "# branch.head main\n# branch.ab +1 -0\n? untracked.txt\n",
	}}
	deps := Deps{Runner: runner}

	_, out, err := deps.repoStatus(context.Background(), nil, RepoStatusInput{Root: root})
	if err != nil {
		t.Fatalf("repoStatus: unexpected error: %v", err)
	}
	if len(out.Repos) != 1 {
		t.Fatalf("repoStatus returned %d repos, want 1: %+v", len(out.Repos), out.Repos)
	}
	got := out.Repos[0]
	want := RepoStatusEntry{Name: "myrepo", Path: repoDir, Branch: "main", Ahead: 1, Behind: 0, Dirty: 0, Untracked: 1}
	if got != want {
		t.Errorf("repoStatus repo entry = %+v, want %+v", got, want)
	}
}

func TestResolveRepoRefOwnerRepoSlug(t *testing.T) {
	runner := fakeRunner{}
	dir, ownerRepo, err := resolveRepoRef(context.Background(), runner, "/root", "owner/repo")
	if err != nil {
		t.Fatalf("resolveRepoRef: unexpected error: %v", err)
	}
	if dir != "." || ownerRepo != "owner/repo" {
		t.Errorf("resolveRepoRef = (%q, %q), want (\".\", \"owner/repo\")", dir, ownerRepo)
	}
}

func TestResolveRepoRefLocalName(t *testing.T) {
	root := "/workspace"
	runner := fakeRunner{responses: map[string]string{
		"git remote get-url origin": "git@github.com:owner/myrepo.git\n",
	}}
	dir, ownerRepo, err := resolveRepoRef(context.Background(), runner, root, "myrepo")
	if err != nil {
		t.Fatalf("resolveRepoRef: unexpected error: %v", err)
	}
	wantDir := filepath.Join(root, "myrepo")
	if dir != wantDir || ownerRepo != "owner/myrepo" {
		t.Errorf("resolveRepoRef = (%q, %q), want (%q, \"owner/myrepo\")", dir, ownerRepo, wantDir)
	}
}

func TestResolveRepoRefRejectsPathEscape(t *testing.T) {
	runner := fakeRunner{}
	if _, _, err := resolveRepoRef(context.Background(), runner, "/root", "../escape"); err == nil {
		t.Fatal("resolveRepoRef(\"../escape\"): got nil error, want error")
	}
}

func TestResolveAndValidateRootRejectsTraversal(t *testing.T) {
	if _, err := resolveAndValidateRoot("../outside"); err == nil {
		t.Fatal("resolveAndValidateRoot(\"../outside\"): got nil error, want error")
	}
}
