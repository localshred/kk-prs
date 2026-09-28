// Package gh is a thin gh-CLI JSON client for kk-prs.
package gh

import (
	"strconv"
	"strings"

	"github.com/localshred/koolkit/pkg/run"
)

// PR represents a GitHub pull request.
type PR struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	CreatedAt  string `json:"createdAt"`
	Author     struct {
		Login string `json:"login"`
	} `json:"author"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

// Check represents a GitHub PR check.
type Check struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	Workflow string `json:"workflow"`
	Link     string `json:"link"`
}

// Client is a gh-CLI JSON client.
type Client struct {
	Run run.Runner
}

const prFields = "number,title,author,repository,url,createdAt"

func (c Client) search(filter string) ([]PR, error) {
	var prs []PR
	err := run.JSON(c.Run, &prs, "gh", "search", "prs", filter, "--state=open",
		"--json", prFields, "--limit", "50")
	return prs, err
}

// Authored returns PRs authored by the current user.
func (c Client) Authored() ([]PR, error) { return c.search("--author=@me") }

// Checks returns checks for a PR. Returns nil on error.
func (c Client) Checks(repo string, number int) []Check {
	var checks []Check
	if err := run.JSON(c.Run, &checks, "gh", "pr", "checks", strconv.Itoa(number),
		"--repo", repo, "--json", "name,state,link,workflow"); err != nil {
		return nil
	}
	return checks
}

// ReviewRequests returns PRs requesting review from the current user.
func (c Client) ReviewRequests() ([]PR, error) { return c.search("--review-requested=@me") }

// User returns the current user's login.
func (c Client) User() (string, error) {
	res, err := c.Run.Run("gh", "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}
