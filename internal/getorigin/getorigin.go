// Package getorigin parses a git remote URL (SSH or HTTPS GitHub form) into
// an "owner/repo" slug suitable for `gh -R`.
package getorigin

import (
	"fmt"
	"regexp"
	"strings"
)

var githubURLPattern = regexp.MustCompile(`github\.com[:/]([^/]+)/([^/]+?)(\.git)?/?$`)

// ParseOwnerRepo extracts "owner/repo" from a GitHub remote URL, accepting
// both SSH (git@github.com:owner/repo.git) and HTTPS
// (https://github.com/owner/repo.git) forms.
func ParseOwnerRepo(remoteURL string) (string, error) {
	remoteURL = strings.TrimSpace(remoteURL)
	m := githubURLPattern.FindStringSubmatch(remoteURL)
	if m == nil {
		return "", fmt.Errorf("remote url %q is not a recognizable github.com url", remoteURL)
	}
	return m[1] + "/" + m[2], nil
}
