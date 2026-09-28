package render

import (
	"testing"

	"github.com/localshred/kk-prs/internal/gh"
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

func TestCompactRowShape(t *testing.T) {
	pr := gh.PR{Number: 5, Title: "T", URL: "u", CreatedAt: "2026-01-01T00:00:00Z"}
	pr.Repository.NameWithOwner = "o/r"
	row := CompactRow(pr, "✅")
	if len(row) != 4 || row[2] != "✅" {
		t.Fatalf("row = %v", row)
	}
}

func TestAgeCellBadTimestamp(t *testing.T) {
	if got := AgeCell("not-a-time"); got != "?" {
		t.Fatalf("AgeCell(bad) = %q, want ?", got)
	}
}
