// Package ghx wraps the GitHub CLI (gh) to list workflow runs and pull
// requests, parsing its JSON output.
package ghx

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/execx"
)

// ErrGHNotFound is returned when the gh CLI is not on PATH.
var ErrGHNotFound = fmt.Errorf("gh: command not found on PATH")

// CheckAvailable returns ErrGHNotFound if gh is not on PATH.
func CheckAvailable() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return ErrGHNotFound
	}
	return nil
}

// Run describes one CI workflow run, as reported by `gh run list`.
type Run struct {
	Workflow   string    `json:"workflowName"`
	Conclusion string    `json:"conclusion"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	URL        string    `json:"url"`
}

// rawRun mirrors the subset of `gh run list --json` fields we consume.
type rawRun struct {
	WorkflowName string `json:"workflowName"`
	Conclusion   string `json:"conclusion"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt"`
	URL          string `json:"url"`
}

// ListRuns runs `gh run list -R owner/repo --limit N --json ...` and parses
// the result.
func ListRuns(ctx context.Context, runner execx.Runner, dir, ownerRepo string, limit int) ([]Run, error) {
	out, err := runner.Run(ctx, dir, "gh", "run", "list",
		"-R", ownerRepo,
		"--limit", strconv.Itoa(limit),
		"--json", "workflowName,conclusion,status,createdAt,url",
	)
	if err != nil {
		return nil, fmt.Errorf("gh run list for %s: %w", ownerRepo, err)
	}
	return parseRuns(out)
}

func parseRuns(out string) ([]Run, error) {
	var raw []rawRun
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parsing gh run list output: %w", err)
	}
	runs := make([]Run, 0, len(raw))
	for _, r := range raw {
		run := Run{
			Workflow:   r.WorkflowName,
			Conclusion: r.Conclusion,
			Status:     r.Status,
			URL:        r.URL,
		}
		if r.CreatedAt != "" {
			t, err := time.Parse(time.RFC3339, r.CreatedAt)
			if err != nil {
				return nil, fmt.Errorf("parsing run createdAt %q: %w", r.CreatedAt, err)
			}
			run.CreatedAt = t
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// PullRequest describes one open PR, as reported by `gh pr list`.
type PullRequest struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	HeadRefName    string `json:"headRefName"`
	ReviewDecision string `json:"reviewDecision"`
	URL            string `json:"url"`
	StatusCheck    string // summarized rollup across statusCheckRollup
}

type rawPR struct {
	Number            int        `json:"number"`
	Title             string     `json:"title"`
	HeadRefName       string     `json:"headRefName"`
	ReviewDecision    string     `json:"reviewDecision"`
	URL               string     `json:"url"`
	StatusCheckRollup []rawCheck `json:"statusCheckRollup"`
}

type rawCheck struct {
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
}

// ListPRs runs `gh pr list -R owner/repo --json ...` and parses the result.
func ListPRs(ctx context.Context, runner execx.Runner, dir, ownerRepo string) ([]PullRequest, error) {
	out, err := runner.Run(ctx, dir, "gh", "pr", "list",
		"-R", ownerRepo,
		"--json", "number,title,headRefName,reviewDecision,url,statusCheckRollup",
	)
	if err != nil {
		return nil, fmt.Errorf("gh pr list for %s: %w", ownerRepo, err)
	}
	return parsePRs(out)
}

func parsePRs(out string) ([]PullRequest, error) {
	var raw []rawPR
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parsing gh pr list output: %w", err)
	}
	prs := make([]PullRequest, 0, len(raw))
	for _, r := range raw {
		prs = append(prs, PullRequest{
			Number:         r.Number,
			Title:          r.Title,
			HeadRefName:    r.HeadRefName,
			ReviewDecision: r.ReviewDecision,
			URL:            r.URL,
			StatusCheck:    summarizeChecks(r.StatusCheckRollup),
		})
	}
	return prs, nil
}

func summarizeChecks(checks []rawCheck) string {
	if len(checks) == 0 {
		return "none"
	}
	failing, pending, passing := 0, 0, 0
	for _, c := range checks {
		switch {
		case c.Status != "" && c.Status != "COMPLETED":
			pending++
		case c.Conclusion == "SUCCESS" || c.Conclusion == "NEUTRAL" || c.Conclusion == "SKIPPED":
			passing++
		case c.Conclusion == "":
			pending++
		default:
			failing++
		}
	}
	switch {
	case failing > 0:
		return "failing"
	case pending > 0:
		return "pending"
	case passing > 0:
		return "passing"
	default:
		return "none"
	}
}
