// Package watch maintains a PR watchlist, diffs snapshots, and drives
// notifications. This file holds the pure snapshot/diff logic ported from
// the mx prs_watch namespace.
package watch

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	reShort = regexp.MustCompile(`^([^/\s]+)/([^/\s#]+)#(\d+)$`)
	reURL   = regexp.MustCompile(`github\.com[:/]([^/\s]+)/([^/\s]+)/pull/(\d+)`)
)

// PRView represents the gh pr view --json output structure.
type PRView struct {
	Number            int           `json:"number"`
	Title             string        `json:"title"`
	URL               string        `json:"url"`
	State             string        `json:"state"`
	Mergeable         string        `json:"mergeable"`
	ReviewDecision    string        `json:"reviewDecision"`
	Comments          []comment     `json:"comments"`
	Reviews           []struct{}    `json:"reviews"`
	StatusCheckRollup []RollupEntry `json:"statusCheckRollup"`
}

// Ref identifies a GitHub PR by owner, repo, and number.
type Ref struct {
	Owner  string
	Repo   string
	Number int
	URL    string
}

// RollupEntry represents a single status check entry from gh pr view.
type RollupEntry struct {
	State      string `json:"state"`      // StatusContext
	Status     string `json:"status"`     // CheckRun (e.g. COMPLETED)
	Conclusion string `json:"conclusion"` // CheckRun conclusion
}

// Snapshot holds the diffable state of a PR.
type Snapshot struct {
	Checks         string `json:"checks"`
	Comments       int    `json:"comments"`
	Mergeable      string `json:"mergeable"`
	ReviewDecision string `json:"review-decision"`
	Reviews        int    `json:"reviews"`
	State          string `json:"state"`
	Title          string `json:"title"`
	URL            string `json:"url"`
	CheckedAt      string `json:"checked-at"`
}

type comment struct {
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	Author    struct {
		Login string `json:"login"`
	} `json:"author"`
}

var (
	contextKind = map[string]string{
		"ERROR": "fail", "EXPECTED": "ok", "FAILURE": "fail", "PENDING": "pending", "SUCCESS": "ok",
	}
	conclusionKind = map[string]string{
		"ACTION_REQUIRED": "fail", "CANCELLED": "fail", "FAILURE": "fail", "NEUTRAL": "ok",
		"SKIPPED": "ok", "STALE": "fail", "STARTUP_FAILURE": "fail", "SUCCESS": "ok", "TIMED_OUT": "fail",
	}
)

// ChecksSummary returns "passing", "failing", "running", or "none" based on rollup state.
func ChecksSummary(rollup []RollupEntry) string {
	if len(rollup) == 0 {
		return "none"
	}
	var anyFail, anyPending bool
	for _, e := range rollup {
		switch checkKind(e) {
		case "fail":
			anyFail = true
		case "pending":
			anyPending = true
		}
	}
	switch {
	case anyFail:
		return "failing"
	case anyPending:
		return "running"
	default:
		return "passing"
	}
}

// ComputeChanges returns notification lines for notable diffs, mirroring the
// mx compute-changes cond.
func ComputeChanges(old, new Snapshot) []string {
	var out []string
	if old.State == "OPEN" && new.State == "MERGED" {
		out = append(out, "🎉 Merged")
	}
	if old.State == "OPEN" && new.State == "CLOSED" {
		out = append(out, "🚫 Closed")
	}
	if d := new.Comments - old.Comments; d > 0 {
		out = append(out, fmt.Sprintf("💬 %d new comment%s", d, plural(d)))
	}
	decisionChanged := old.ReviewDecision != new.ReviewDecision
	if decisionChanged && new.ReviewDecision == "APPROVED" {
		out = append(out, "✅ Approved")
	}
	if decisionChanged && new.ReviewDecision == "CHANGES_REQUESTED" {
		out = append(out, "🔴 Changes requested")
	}
	if !decisionChanged {
		if d := new.Reviews - old.Reviews; d > 0 {
			out = append(out, fmt.Sprintf("👀 %d new review%s", d, plural(d)))
		}
	}
	if old.Checks != new.Checks {
		switch new.Checks {
		case "failing":
			out = append(out, "❌ CI failing")
		case "passing":
			out = append(out, "✅ CI passing")
		case "running":
			out = append(out, "🔄 CI running")
		}
	}
	if old.Mergeable != new.Mergeable && new.Mergeable == "CONFLICTING" {
		out = append(out, "⚠️ Merge conflicts")
	}
	if old.Mergeable == "CONFLICTING" && new.Mergeable == "MERGEABLE" {
		out = append(out, "✅ Conflicts resolved")
	}
	return out
}

// FieldTransitions returns detailed X → Y transition lines for all changed fields.
func FieldTransitions(old, new Snapshot) []string {
	var out []string
	if old.Checks != new.Checks {
		out = append(out, fmt.Sprintf("🔧 CI: %s → %s", old.Checks, new.Checks))
	}
	if old.ReviewDecision != new.ReviewDecision {
		out = append(out, fmt.Sprintf("👀 Review: %s → %s", labelReview(old.ReviewDecision), labelReview(new.ReviewDecision)))
	}
	if old.Mergeable != new.Mergeable {
		out = append(out, fmt.Sprintf("🔀 Merge: %s → %s", old.Mergeable, new.Mergeable))
	}
	if old.State != new.State {
		out = append(out, fmt.Sprintf("📦 State: %s → %s", old.State, new.State))
	}
	return out
}

// Key returns the canonical "owner/repo#number" string for this ref.
func (r Ref) Key() string { return fmt.Sprintf("%s/%s#%d", r.Owner, r.Repo, r.Number) }

// ParseRef accepts a github PR URL or owner/repo#number format.
func ParseRef(s string) (Ref, bool) {
	if m := reURL.FindStringSubmatch(s); m != nil {
		n, err := strconv.Atoi(m[3])
		if err != nil {
			return Ref{}, false
		}
		return Ref{Owner: m[1], Repo: m[2], Number: n,
			URL: fmt.Sprintf("https://github.com/%s/%s/pull/%s", m[1], m[2], m[3])}, true
	}
	if m := reShort.FindStringSubmatch(s); m != nil {
		n, err := strconv.Atoi(m[3])
		if err != nil {
			return Ref{}, false
		}
		return Ref{Owner: m[1], Repo: m[2], Number: n,
			URL: fmt.Sprintf("https://github.com/%s/%s/pull/%s", m[1], m[2], m[3])}, true
	}
	return Ref{}, false
}

// ToSnapshot converts a PRView into a diffable Snapshot.
func ToSnapshot(v PRView) Snapshot {
	return Snapshot{
		Checks:         ChecksSummary(v.StatusCheckRollup),
		Comments:       len(v.Comments),
		Mergeable:      v.Mergeable,
		ReviewDecision: v.ReviewDecision,
		Reviews:        len(v.Reviews),
		State:          v.State,
		Title:          v.Title,
		URL:            v.URL,
	}
}

func checkKind(e RollupEntry) string {
	if e.State != "" {
		if k, ok := contextKind[e.State]; ok {
			return k
		}
		return "pending"
	}
	if e.Status == "COMPLETED" {
		if k, ok := conclusionKind[e.Conclusion]; ok {
			return k
		}
		return "fail"
	}
	return "pending"
}

func labelReview(d string) string {
	switch d {
	case "":
		return "none"
	case "APPROVED":
		return "approved"
	case "CHANGES_REQUESTED":
		return "changes requested"
	case "REVIEW_REQUIRED":
		return "review required"
	default:
		return d
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
