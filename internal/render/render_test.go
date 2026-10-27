package render

import (
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
