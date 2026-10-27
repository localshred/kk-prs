package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/localshred/kk-prs/internal/gh"
	"github.com/localshred/koolkit/pkg/ui"
)

func TestOverallStatus(t *testing.T) {
	cases := []struct {
		in   []gh.Check
		want string
	}{
		{nil, ""},
		{[]gh.Check{{State: "SUCCESS"}}, "✅"},
		{[]gh.Check{{State: "SUCCESS"}, {State: "FAILURE"}}, "❌"},
		{[]gh.Check{{State: "SUCCESS"}, {State: "PENDING"}}, "🔄"},
	}
	for _, c := range cases {
		if got := OverallStatus(c.in); got != c.want {
			t.Errorf("OverallStatus(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSortChecksPendingFirst(t *testing.T) {
	got := SortChecks([]gh.Check{{Name: "b", State: "SUCCESS"}, {Name: "a", State: "PENDING"}})
	if got[0].State != "PENDING" {
		t.Fatalf("pending should sort first, got %+v", got)
	}
}

func TestBaseCell(t *testing.T) {
	if got := ui.StripANSI(BaseCell(gh.PR{BaseRefName: "release-B"})); got != "→ release-B" {
		t.Errorf("BaseCell = %q, want %q", got, "→ release-B")
	}
	if got := BaseCell(gh.PR{}); got != "" {
		t.Errorf("BaseCell(no base) = %q, want empty", got)
	}
	long := gh.PR{BaseRefName: "alice/feature-login-session-refresh-cache-port"}
	if got := ui.StripANSI(BaseCell(long)); got != "→ alice/featur…-cache-port" {
		t.Errorf("BaseCell(long) = %q, want %q", got, "→ alice/featur…-cache-port")
	}
	exact := gh.PR{BaseRefName: "release/2026.10.1-hotfix"}
	if got := ui.StripANSI(BaseCell(exact)); got != "→ release/2026.10.1-hotfix" {
		t.Errorf("BaseCell(24 runes) = %q, want it untouched", got)
	}
}

func TestCompactRowShape(t *testing.T) {
	pr := gh.PR{Number: 5, Title: "T", URL: "u", CreatedAt: "2026-01-01T00:00:00Z", BaseRefName: "main"}
	pr.Repository.NameWithOwner = "o/r"
	row := CompactRow(pr, "✅")
	if len(row) != 5 || row[2] != "✅" || ui.StripANSI(row[3]) != "→ main" || ui.StripANSI(row[4]) != "T" {
		t.Fatalf("row = %q", row)
	}
}

func TestReviewRowShowsBase(t *testing.T) {
	pr := gh.PR{Number: 5, Title: "T", URL: "u", CreatedAt: "2026-01-01T00:00:00Z", BaseRefName: "release-B"}
	pr.Author.Login = "alice"
	row := ReviewRow(pr)
	if len(row) != 5 || ui.StripANSI(row[2]) != "@alice" || ui.StripANSI(row[3]) != "→ release-B" {
		t.Fatalf("row = %q", row)
	}
}

func TestAgeCellBadTimestamp(t *testing.T) {
	if got := AgeCell("not-a-time"); got != "?" {
		t.Fatalf("AgeCell(bad) = %q, want ?", got)
	}
}

func TestFitTitles(t *testing.T) {
	long := "Refactor snapshot diffing so watch ticks stop re-reading every PR"
	rows := [][]string{
		{"o/r#7", "3d", "→ main", ui.Green(long)},
		{"o/r#12", "1w", "→ release-B", ui.Green("Short one")},
	}
	FitTitles(rows, 40)
	var out bytes.Buffer
	ui.PrintTable(&out, rows)
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	for _, l := range lines {
		if n := ui.VisibleLen(l); n > 40 {
			t.Errorf("row is %d wide, want <= 40: %q", n, ui.StripANSI(l))
		}
	}
	if got := ui.StripANSI(rows[0][3]); !strings.HasSuffix(got, "…") || !strings.HasPrefix(got, "Refactor") {
		t.Errorf("long title = %q, want a prefix ending in …", got)
	}
	if got := ui.StripANSI(rows[1][3]); got != "Short one" {
		t.Errorf("short title = %q, want it untouched", got)
	}
	if rows[0][3] != ui.Green(ui.StripANSI(rows[0][3])) {
		t.Errorf("fitted title lost its color: %q", rows[0][3])
	}
}

func TestFitTitlesKeepsMinimum(t *testing.T) {
	rows := [][]string{{"o/a-very-long-repository-name#7", "3d", "→ main", ui.Green("A title that cannot fit")}}
	FitTitles(rows, 20)
	if got := []rune(ui.StripANSI(rows[0][3])); len(got) != minTitleWidth {
		t.Errorf("title = %q (%d runes), want %d", string(got), len(got), minTitleWidth)
	}
}

func TestReviewRowTruncatesAuthor(t *testing.T) {
	pr := gh.PR{Number: 5, Title: "T", URL: "u", CreatedAt: "2026-01-01T00:00:00Z"}
	pr.Author.Login = "renovate-enterprise-github-prod"
	if got := ui.StripANSI(ReviewRow(pr)[2]); got != "@renovate-e…thub-prod" {
		t.Errorf("author = %q, want %q", got, "@renovate-e…thub-prod")
	}
}
