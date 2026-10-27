// Package render formats PRs and checks for terminal display. Ported from
// the mx prs command.
package render

import (
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/localshred/kk-prs/internal/gh"
	"github.com/localshred/koolkit/pkg/ui"
)

const (
	authorWidth   = 20
	baseWidth     = 24
	minTitleWidth = 10
	tableGap      = 2
)

var IgnoredCheck = regexp.MustCompile("(?i)snyk")

var statusPriority = map[string]int{
	"PENDING": 1, "IN_PROGRESS": 1, "QUEUED": 1,
	"ERROR": 2, "FAILURE": 2,
	"SUCCESS": 3,
	"SKIPPED": 4, "CANCELLED": 4,
}

// AgeCell renders a colored humanized age from an ISO-8601 timestamp.
func AgeCell(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return "?"
	}
	return ui.AgeCell(time.Since(t))
}

// BaseCell renders the branch a PR merges into, dimmed, or "" when unknown.
func BaseCell(pr gh.PR) string {
	if pr.BaseRefName == "" {
		return ""
	}
	return ui.Dim("→ " + truncateMiddle(pr.BaseRefName, baseWidth))
}

// CompactRow returns a compact PR row: linked repo#num, age, status, base
// branch, title.
func CompactRow(pr gh.PR, status string) []string {
	return []string{
		ui.Hyperlink(pr.URL, ui.Cyan(repoID(pr))),
		AgeCell(pr.CreatedAt),
		status,
		BaseCell(pr),
		ui.Green(pr.Title),
	}
}

// FitTitles shortens each row's last cell, a green PR title, with a trailing
// … so the row fits width columns once ui.PrintTable aligns it, leaving one
// column of slack for wide status emoji. Titles never shrink below
// minTitleWidth runes.
func FitTitles(rows [][]string, width int) {
	widths := map[int]int{}
	for _, r := range rows {
		for i := 0; i < len(r)-1; i++ {
			widths[i] = max(widths[i], ui.VisibleLen(r[i]))
		}
	}
	used := 1
	for _, w := range widths {
		used += w + tableGap
	}
	room := max(minTitleWidth, width-used)
	for _, r := range rows {
		if last := len(r) - 1; last >= 0 {
			r[last] = ui.Green(truncateEnd(ui.StripANSI(r[last]), room))
		}
	}
}

// OverallStatus returns the overall status emoji for a slice of checks.
func OverallStatus(checks []gh.Check) string {
	if len(checks) == 0 {
		return ""
	}
	var anyFail, anyPending, allSuccess bool = false, false, true
	for _, c := range checks {
		switch c.State {
		case "ERROR", "FAILURE":
			anyFail = true
		case "PENDING", "IN_PROGRESS", "QUEUED":
			anyPending = true
		}
		if c.State != "SUCCESS" {
			allSuccess = false
		}
	}
	switch {
	case anyFail:
		return "❌"
	case anyPending:
		return "🔄"
	case allSuccess:
		return "✅"
	default:
		return ""
	}
}

// ReviewRow returns a review-request PR row: linked repo#num, age, @author,
// base branch, title.
func ReviewRow(pr gh.PR) []string {
	return []string{
		ui.Hyperlink(pr.URL, ui.Cyan(repoID(pr))),
		AgeCell(pr.CreatedAt),
		ui.Yellow("@" + truncateMiddle(pr.Author.Login, authorWidth)),
		BaseCell(pr),
		ui.Green(pr.Title),
	}
}

// SortChecks returns a stable-sorted copy of checks by status priority then name.
func SortChecks(checks []gh.Check) []gh.Check {
	out := append([]gh.Check(nil), checks...)
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := priority(out[i].State), priority(out[j].State)
		if pi != pj {
			return pi < pj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func priority(state string) int {
	if p, ok := statusPriority[state]; ok {
		return p
	}
	return 5
}

func repoID(pr gh.PR) string {
	return fmt.Sprintf("%s#%d", pr.Repository.NameWithOwner, pr.Number)
}

func truncateMiddle(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	keep := width - 1
	head := keep - keep/2
	return string(r[:head]) + "…" + string(r[len(r)-(keep-head):])
}

func truncateEnd(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "…"
}
