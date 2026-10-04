// Package gitx discovers local git repositories under a workspace root and
// extracts status information from git porcelain output.
package gitx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/execx"
)

// Repo identifies a discovered git repository.
type Repo struct {
	Name string // directory base name
	Path string // absolute path
}

// Status describes the working-tree and branch status of a repo.
type Status struct {
	Repo      Repo
	Branch    string
	Ahead     int
	Behind    int
	Dirty     int
	Untracked int
}

// Branch describes a local branch and its last commit time.
type Branch struct {
	Name       string
	LastCommit time.Time
}

// ResolveRoot resolves root (or the DEV_PULSE_ROOT env var, defaulting to
// $HOME/development) to an absolute path.
func ResolveRoot(root string) (string, error) {
	if root == "" {
		root = os.Getenv("DEV_PULSE_ROOT")
	}
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		root = filepath.Join(home, "development")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolving root %q: %w", root, err)
	}
	return abs, nil
}

// DiscoverRepos lists immediate subdirectories of root that are git
// repositories (contain a .git entry), optionally filtered by a case
// insensitive substring match on the repo name.
func DiscoverRepos(root string, nameFilter string) ([]Repo, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading root %q: %w", root, err)
	}

	var repos []Repo
	nameFilter = strings.ToLower(nameFilter)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := filepath.Base(e.Name())
		if name != e.Name() || strings.Contains(name, string(os.PathSeparator)) {
			// Defensive: directory entries should never contain separators or
			// escape the root, but validate anyway before trusting the name.
			continue
		}
		if nameFilter != "" && !strings.Contains(strings.ToLower(name), nameFilter) {
			continue
		}
		candidate := filepath.Join(root, name)
		if _, err := os.Stat(filepath.Join(candidate, ".git")); err != nil {
			continue
		}
		repos = append(repos, Repo{Name: name, Path: candidate})
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].Name < repos[j].Name })
	return repos, nil
}

// GitStatus runs `git status --porcelain=v2 --branch` in repo.Path and parses
// the result into a Status.
func GitStatus(ctx context.Context, runner execx.Runner, repo Repo) (Status, error) {
	out, err := runner.Run(ctx, repo.Path, "git", "status", "--porcelain=v2", "--branch")
	if err != nil {
		return Status{}, fmt.Errorf("git status in %s: %w", repo.Path, err)
	}
	return parsePorcelainV2(repo, out), nil
}

// parsePorcelainV2 parses `git status --porcelain=v2 --branch` output.
func parsePorcelainV2(repo Repo, out string) Status {
	st := Status{Repo: repo}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			st.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.ab "):
			fields := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			for _, f := range fields {
				if n, ok := strings.CutPrefix(f, "+"); ok {
					st.Ahead, _ = strconv.Atoi(n)
				} else if n, ok := strings.CutPrefix(f, "-"); ok {
					st.Behind, _ = strconv.Atoi(n)
				}
			}
		case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "2 "):
			st.Dirty++
		case strings.HasPrefix(line, "? "):
			st.Untracked++
		}
	}
	return st
}

// LocalBranches runs `git for-each-ref` to list local branches with their
// last commit time.
func LocalBranches(ctx context.Context, runner execx.Runner, repo Repo) ([]Branch, error) {
	out, err := runner.Run(ctx, repo.Path, "git", "for-each-ref",
		"--format=%(refname:short)\t%(committerdate:iso-strict)", "refs/heads/")
	if err != nil {
		return nil, fmt.Errorf("listing branches in %s: %w", repo.Path, err)
	}
	return parseForEachRef(out)
}

func parseForEachRef(out string) ([]Branch, error) {
	var branches []Branch
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("parsing for-each-ref line %q: want 2 tab-separated fields", line)
		}
		t, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			return nil, fmt.Errorf("parsing commit date %q: %w", parts[1], err)
		}
		branches = append(branches, Branch{Name: parts[0], LastCommit: t})
	}
	return branches, nil
}
