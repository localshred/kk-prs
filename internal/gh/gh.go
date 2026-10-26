// Package gh is a thin gh-CLI JSON client for kk-prs.
package gh

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/localshred/koolkit/pkg/run"
)

// BoardPR is a pull request, open or merged, fetched for the release board.
type BoardPR struct {
	Author        string
	BaseRef       string
	Body          string
	CreatedAt     time.Time
	DefaultBranch string
	MergeOID      string
	MergedAt      time.Time
	Number        int
	Repo          string
	State         string
	Title         string
	URL           string
}

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

// RecentPRs returns the author's open PRs and the PRs they merged on or after
// since, using one paginated gh api graphql call per page. A search stops
// paginating when a page has no next page or no end cursor. author may be
// "@me". Bodies are fetched only when withBody is true.
func (c Client) RecentPRs(author string, since time.Time, withBody bool) ([]BoardPR, error) {
	openQuery := fmt.Sprintf("is:pr is:open author:%s", author)
	mergedQuery := fmt.Sprintf("is:pr is:merged author:%s merged:>=%s",
		author, since.UTC().Format("2006-01-02"))
	document := recentPRsDocument(withBody)

	var prs []BoardPR
	var openCursor, mergedCursor string
	openDone, mergedDone := false, false
	for !openDone || !mergedDone {
		args := []string{
			"api", "graphql",
			"-f", "query=" + document,
			"-f", "openQuery=" + openQuery,
			"-f", "mergedQuery=" + mergedQuery,
		}
		if openCursor != "" {
			args = append(args, "-f", "openCursor="+openCursor)
		}
		if mergedCursor != "" {
			args = append(args, "-f", "mergedCursor="+mergedCursor)
		}
		var resp recentPRsResponse
		if err := run.JSON(c.Run, &resp, "gh", args...); err != nil {
			return nil, err
		}
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("graphql: %s", resp.Errors[0].Message)
		}
		var err error
		if prs, err = appendBoardPRs(prs, resp.Data.Open, openDone); err != nil {
			return nil, err
		}
		if prs, err = appendBoardPRs(prs, resp.Data.Merged, mergedDone); err != nil {
			return nil, err
		}
		openDone = openDone || !resp.Data.Open.PageInfo.HasNextPage || resp.Data.Open.PageInfo.EndCursor == ""
		mergedDone = mergedDone || !resp.Data.Merged.PageInfo.HasNextPage || resp.Data.Merged.PageInfo.EndCursor == ""
		if !openDone {
			openCursor = resp.Data.Open.PageInfo.EndCursor
		}
		if !mergedDone {
			mergedCursor = resp.Data.Merged.PageInfo.EndCursor
		}
	}
	return prs, nil
}

// ReviewRequests returns PRs requesting review from the current user.
func (c Client) ReviewRequests() ([]PR, error) { return c.search("--review-requested=@me") }

// Runner returns the underlying runner for the client.
func (c Client) Runner() run.Runner { return c.Run }

// User returns the current user's login.
func (c Client) User() (string, error) {
	res, err := c.Run.Run("gh", "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Stdout), nil
}

type boardNode struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Body        string `json:"body"`
	CreatedAt   string `json:"createdAt"`
	MergedAt    string `json:"mergedAt"`
	State       string `json:"state"`
	BaseRefName string `json:"baseRefName"`
	MergeCommit *struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	Repository struct {
		NameWithOwner    string `json:"nameWithOwner"`
		DefaultBranchRef *struct {
			Name string `json:"name"`
		} `json:"defaultBranchRef"`
	} `json:"repository"`
}

type recentPRsConnection struct {
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
	Nodes []boardNode `json:"nodes"`
}

type recentPRsResponse struct {
	Data struct {
		Merged recentPRsConnection `json:"merged"`
		Open   recentPRsConnection `json:"open"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func appendBoardPRs(prs []BoardPR, conn recentPRsConnection, done bool) ([]BoardPR, error) {
	if done {
		return prs, nil
	}
	for _, n := range conn.Nodes {
		if n.Number == 0 {
			continue
		}
		pr, err := n.toBoardPR()
		if err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}

func recentPRsDocument(withBody bool) string {
	body := ""
	if withBody {
		body = " body"
	}
	fields := "number title url" + body + " createdAt mergedAt state baseRefName " +
		"mergeCommit { oid } author { login } " +
		"repository { nameWithOwner defaultBranchRef { name } }"
	selection := "pageInfo { hasNextPage endCursor } nodes { ... on PullRequest { " + fields + " } }"
	return "query($openQuery: String!, $mergedQuery: String!, $openCursor: String, $mergedCursor: String) { " +
		"open: search(type: ISSUE, query: $openQuery, first: 100, after: $openCursor) { " + selection + " } " +
		"merged: search(type: ISSUE, query: $mergedQuery, first: 100, after: $mergedCursor) { " + selection + " } }"
}

func (n boardNode) toBoardPR() (BoardPR, error) {
	created, err := parseTime(n.CreatedAt)
	if err != nil {
		return BoardPR{}, fmt.Errorf("parse createdAt of #%d: %w", n.Number, err)
	}
	merged, err := parseTime(n.MergedAt)
	if err != nil {
		return BoardPR{}, fmt.Errorf("parse mergedAt of #%d: %w", n.Number, err)
	}
	pr := BoardPR{
		BaseRef:   n.BaseRefName,
		Body:      n.Body,
		CreatedAt: created,
		MergedAt:  merged,
		Number:    n.Number,
		Repo:      n.Repository.NameWithOwner,
		State:     n.State,
		Title:     n.Title,
		URL:       n.URL,
	}
	if n.Author != nil {
		pr.Author = n.Author.Login
	}
	if n.MergeCommit != nil {
		pr.MergeOID = n.MergeCommit.OID
	}
	if n.Repository.DefaultBranchRef != nil {
		pr.DefaultBranch = n.Repository.DefaultBranchRef.Name
	}
	return pr, nil
}
