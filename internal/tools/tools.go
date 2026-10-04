// Package tools implements the dev-pulse MCP tools: repo_status, ci_status,
// pr_queue, and stale_branches.
package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/execx"
	"github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/getorigin"
	"github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/ghx"
	"github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/gitx"
)

// Deps bundles the dependencies every tool handler needs, so tests can
// inject a fake execx.Runner and run fully offline.
type Deps struct {
	Runner execx.Runner
}

// RegisterAll registers all dev-pulse tools on server.
func RegisterAll(server *mcp.Server, deps Deps) {
	readOnly := &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: boolPtr(false),
		IdempotentHint:  true,
		OpenWorldHint:   boolPtr(true),
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "repo_status",
		Description: "List git repos under DEV_PULSE_ROOT with branch, ahead/behind, and dirty/untracked counts.",
		Annotations: readOnly,
	}, deps.repoStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ci_status",
		Description: "List recent GitHub Actions CI runs for a repo via `gh run list`.",
		Annotations: readOnly,
	}, deps.ciStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "pr_queue",
		Description: "List open pull requests with review decision and CI state via `gh pr list`.",
		Annotations: readOnly,
	}, deps.prQueue)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stale_branches",
		Description: "List local git branches whose last commit is older than N days.",
		Annotations: readOnly,
	}, deps.staleBranches)
}

func boolPtr(b bool) *bool { return &b }

// --- repo_status ---

// RepoStatusInput is the input for the repo_status tool.
type RepoStatusInput struct {
	Root string `json:"root,omitempty" jsonschema:"optional workspace root; defaults to DEV_PULSE_ROOT or $HOME/development"`
	Name string `json:"name,omitempty" jsonschema:"optional substring filter on repo name"`
}

// RepoStatusEntry is one repo's status in the repo_status output.
type RepoStatusEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Branch    string `json:"branch"`
	Ahead     int    `json:"ahead"`
	Behind    int    `json:"behind"`
	Dirty     int    `json:"dirty"`
	Untracked int    `json:"untracked"`
}

// RepoStatusOutput is the output for the repo_status tool.
type RepoStatusOutput struct {
	Repos []RepoStatusEntry `json:"repos"`
}

func (d Deps) repoStatus(ctx context.Context, _ *mcp.CallToolRequest, in RepoStatusInput) (*mcp.CallToolResult, RepoStatusOutput, error) {
	root, err := resolveAndValidateRoot(in.Root)
	if err != nil {
		return toolError(err), RepoStatusOutput{}, nil
	}

	repos, err := gitx.DiscoverRepos(root, in.Name)
	if err != nil {
		return toolError(fmt.Errorf("discovering repos: %w", err)), RepoStatusOutput{}, nil
	}

	out := RepoStatusOutput{}
	for _, repo := range repos {
		st, err := gitx.GitStatus(ctx, d.Runner, repo)
		if err != nil {
			return toolError(fmt.Errorf("repo %s: %w", repo.Name, err)), RepoStatusOutput{}, nil
		}
		out.Repos = append(out.Repos, RepoStatusEntry{
			Name:      repo.Name,
			Path:      repo.Path,
			Branch:    st.Branch,
			Ahead:     st.Ahead,
			Behind:    st.Behind,
			Dirty:     st.Dirty,
			Untracked: st.Untracked,
		})
	}
	return nil, out, nil
}

// --- ci_status ---

// CIStatusInput is the input for the ci_status tool.
type CIStatusInput struct {
	Repo  string `json:"repo" jsonschema:"repo name under DEV_PULSE_ROOT, or owner/name"`
	Limit int    `json:"limit,omitempty" jsonschema:"max number of runs to return; default 10"`
}

// CIStatusOutput is the output for the ci_status tool.
type CIStatusOutput struct {
	Runs []CIRunEntry `json:"runs"`
}

// CIRunEntry is one CI run in the ci_status output.
type CIRunEntry struct {
	Workflow   string `json:"workflow"`
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
	AgeMinutes int64  `json:"age_minutes"`
	URL        string `json:"url"`
}

func (d Deps) ciStatus(ctx context.Context, _ *mcp.CallToolRequest, in CIStatusInput) (*mcp.CallToolResult, CIStatusOutput, error) {
	if err := ghx.CheckAvailable(); err != nil {
		return toolError(err), CIStatusOutput{}, nil
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}

	root, err := resolveAndValidateRoot("")
	if err != nil {
		return toolError(err), CIStatusOutput{}, nil
	}
	dir, ownerRepo, err := resolveRepoRef(ctx, d.Runner, root, in.Repo)
	if err != nil {
		return toolError(err), CIStatusOutput{}, nil
	}

	runs, err := ghx.ListRuns(ctx, d.Runner, dir, ownerRepo, limit)
	if err != nil {
		return toolError(err), CIStatusOutput{}, nil
	}

	out := CIStatusOutput{}
	now := time.Now()
	for _, r := range runs {
		age := int64(0)
		if !r.CreatedAt.IsZero() {
			age = int64(now.Sub(r.CreatedAt).Minutes())
		}
		out.Runs = append(out.Runs, CIRunEntry{
			Workflow:   r.Workflow,
			Conclusion: r.Conclusion,
			Status:     r.Status,
			AgeMinutes: age,
			URL:        r.URL,
		})
	}
	return nil, out, nil
}

// --- pr_queue ---

// PRQueueInput is the input for the pr_queue tool.
type PRQueueInput struct {
	Repo string `json:"repo,omitempty" jsonschema:"optional repo name under DEV_PULSE_ROOT; if omitted, all repos are covered"`
}

// PRQueueOutput is the output for the pr_queue tool.
type PRQueueOutput struct {
	PullRequests []PREntry `json:"pull_requests"`
}

// PREntry is one PR in the pr_queue output.
type PREntry struct {
	Repo           string `json:"repo"`
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Branch         string `json:"branch"`
	ReviewDecision string `json:"review_decision"`
	CIState        string `json:"ci_state"`
	URL            string `json:"url"`
}

func (d Deps) prQueue(ctx context.Context, _ *mcp.CallToolRequest, in PRQueueInput) (*mcp.CallToolResult, PRQueueOutput, error) {
	if err := ghx.CheckAvailable(); err != nil {
		return toolError(err), PRQueueOutput{}, nil
	}

	root, err := resolveAndValidateRoot("")
	if err != nil {
		return toolError(err), PRQueueOutput{}, nil
	}

	repos, err := gitx.DiscoverRepos(root, in.Repo)
	if err != nil {
		return toolError(fmt.Errorf("discovering repos: %w", err)), PRQueueOutput{}, nil
	}

	out := PRQueueOutput{}
	for _, repo := range repos {
		ownerRepo, err := originOwnerRepo(ctx, d.Runner, repo.Path)
		if err != nil {
			continue // skip repos without a resolvable GitHub origin
		}
		prs, err := ghx.ListPRs(ctx, d.Runner, repo.Path, ownerRepo)
		if err != nil {
			return toolError(fmt.Errorf("repo %s: %w", repo.Name, err)), PRQueueOutput{}, nil
		}
		for _, pr := range prs {
			out.PullRequests = append(out.PullRequests, PREntry{
				Repo:           repo.Name,
				Number:         pr.Number,
				Title:          pr.Title,
				Branch:         pr.HeadRefName,
				ReviewDecision: pr.ReviewDecision,
				CIState:        pr.StatusCheck,
				URL:            pr.URL,
			})
		}
	}
	return nil, out, nil
}

// --- stale_branches ---

// StaleBranchesInput is the input for the stale_branches tool.
type StaleBranchesInput struct {
	Days int    `json:"days,omitempty" jsonschema:"branches with no commit in this many days are stale; default 30"`
	Repo string `json:"repo,omitempty" jsonschema:"optional substring filter on repo name; if omitted, all repos are covered"`
}

// StaleBranchesOutput is the output for the stale_branches tool.
type StaleBranchesOutput struct {
	Branches []StaleBranchEntry `json:"branches"`
}

// StaleBranchEntry is one stale branch in the stale_branches output.
type StaleBranchEntry struct {
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	AgeDays    int    `json:"age_days"`
	LastCommit string `json:"last_commit"`
}

func (d Deps) staleBranches(ctx context.Context, _ *mcp.CallToolRequest, in StaleBranchesInput) (*mcp.CallToolResult, StaleBranchesOutput, error) {
	days := in.Days
	if days <= 0 {
		days = 30
	}

	root, err := resolveAndValidateRoot("")
	if err != nil {
		return toolError(err), StaleBranchesOutput{}, nil
	}

	repos, err := gitx.DiscoverRepos(root, in.Repo)
	if err != nil {
		return toolError(fmt.Errorf("discovering repos: %w", err)), StaleBranchesOutput{}, nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	out := StaleBranchesOutput{}
	for _, repo := range repos {
		branches, err := gitx.LocalBranches(ctx, d.Runner, repo)
		if err != nil {
			return toolError(fmt.Errorf("repo %s: %w", repo.Name, err)), StaleBranchesOutput{}, nil
		}
		for _, b := range branches {
			if b.LastCommit.Before(cutoff) {
				out.Branches = append(out.Branches, StaleBranchEntry{
					Repo:       repo.Name,
					Branch:     b.Name,
					AgeDays:    int(time.Since(b.LastCommit).Hours() / 24),
					LastCommit: b.LastCommit.Format(time.RFC3339),
				})
			}
		}
	}
	return nil, out, nil
}

// --- shared helpers ---

func toolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}

// resolveAndValidateRoot resolves root to an absolute path. When a caller
// supplies a root, it must not escape DEV_PULSE_ROOT's default resolution in
// a way that relies on symlink or ".." traversal.
func resolveAndValidateRoot(root string) (string, error) {
	resolved, err := gitx.ResolveRoot(root)
	if err != nil {
		return "", err
	}
	if strings.Contains(root, "..") {
		return "", fmt.Errorf("root %q must not contain path traversal segments", root)
	}
	return resolved, nil
}

// resolveRepoRef resolves a user-supplied repo reference (a bare "owner/name"
// or a local directory name under root) to a working directory and an
// "owner/name" slug for gh -R.
func resolveRepoRef(ctx context.Context, runner execx.Runner, root, repo string) (dir string, ownerRepo string, err error) {
	if repo == "" {
		return "", "", fmt.Errorf("repo is required")
	}
	if strings.Count(repo, "/") == 1 && !strings.ContainsAny(repo, `\`) && !strings.Contains(repo, "..") {
		// Looks like an owner/name slug already; use the current directory.
		owner, name, _ := strings.Cut(repo, "/")
		if owner != "" && name != "" {
			return ".", repo, nil
		}
	}
	if strings.ContainsAny(repo, `/\`) || repo == ".." || repo == "." {
		return "", "", fmt.Errorf("repo %q must be a plain name or owner/name slug", repo)
	}
	name := filepath.Base(repo)
	if name != repo {
		return "", "", fmt.Errorf("repo %q must be a plain name or owner/name slug", repo)
	}
	candidate := filepath.Join(root, name)
	ownerRepo, err = originOwnerRepo(ctx, runner, candidate)
	if err != nil {
		return "", "", fmt.Errorf("resolving GitHub origin for %s: %w", name, err)
	}
	return candidate, ownerRepo, nil
}

func originOwnerRepo(ctx context.Context, runner execx.Runner, dir string) (string, error) {
	out, err := runner.Run(ctx, dir, "git", "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin: %w", err)
	}
	return getorigin.ParseOwnerRepo(out)
}
