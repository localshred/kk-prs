package render

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/localshred/kk-prs/internal/board"
	"github.com/localshred/kk-prs/internal/gh"
	"github.com/localshred/koolkit/pkg/ui"
)

var boardNow = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func openCard(repo string, n int, title string) board.Card {
	return board.Card{PR: gh.BoardPR{
		Number: n, Repo: repo, State: "OPEN", Title: title,
		URL: "https://example.com/pr", Author: "alice",
		CreatedAt: boardNow.Add(-3 * 24 * time.Hour),
	}}
}

func mergedCard(repo string, n int) board.Card {
	return board.Card{PR: gh.BoardPR{
		Number: n, Repo: repo, State: "MERGED", Title: "merged change",
		URL: "https://example.com/pr", Author: "alice",
		MergedAt: boardNow.Add(-2 * 24 * time.Hour),
	}}
}

func releaseLane(name string, cards ...board.Card) board.Lane {
	return board.Lane{Kind: board.LaneRelease, Name: name, Cards: cards}
}

func plainBoard(b board.Board, width int, showAuthor bool) string {
	return ui.StripANSI(Board(b, width, showAuthor, boardNow))
}

func TestBoardLanesSideBySide(t *testing.T) {
	b := board.Board{Lanes: []board.Lane{
		{Kind: board.LaneOpen, Name: "OPEN", Cards: []board.Card{openCard("o/svc-a", 1, "one"), openCard("o/svc-a", 2, "two")}},
		{Kind: board.LanePending, Name: "NOT IN A RELEASE", Cards: []board.Card{mergedCard("o/svc-a", 3)}},
		releaseLane("release-A", mergedCard("o/svc-a", 4)),
	}}
	out := plainBoard(b, 200, false)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	h := lines[0]
	i1 := strings.Index(h, "OPEN (2)")
	i2 := strings.Index(h, "NOT IN A RELEASE (1)")
	i3 := strings.Index(h, "release-A (1)")
	if i1 < 0 || i2 < i1 || i3 < i2 {
		t.Fatalf("header order wrong: %q", h)
	}
	want := ui.VisibleLen(lines[0])
	for _, l := range lines {
		if got := ui.VisibleLen(l); got != want {
			t.Errorf("line width %d, want %d: %q", got, want, l)
		}
	}
	if want != 24*3+2*2 {
		t.Errorf("board width = %d, want %d", want, 24*3+2*2)
	}
}

func TestBoardCutHeader(t *testing.T) {
	cut := boardNow.Add(-6 * 24 * time.Hour)
	l := releaseLane("release-A", mergedCard("o/svc-a", 4))
	l.CutAt = cut
	out := plainBoard(board.Board{Lanes: []board.Lane{l}}, 200, false)
	want := "cut ~" + cut.Format("Jan 2") + " (6d)"
	if !strings.Contains(out, want) {
		t.Errorf("missing %q in:\n%s", want, out)
	}
	l.CutAt = time.Time{}
	out = plainBoard(board.Board{Lanes: []board.Lane{l}}, 200, false)
	if strings.Contains(out, "cut") {
		t.Errorf("unexpected cut header in:\n%s", out)
	}
}

func TestBoardFoldsOldest(t *testing.T) {
	b := board.Board{Lanes: []board.Lane{
		{Kind: board.LaneOpen, Name: "OPEN", Cards: []board.Card{openCard("o/svc-a", 1, "one")}},
		{Kind: board.LanePending, Name: "NOT IN A RELEASE", Cards: []board.Card{mergedCard("o/svc-a", 2)}},
		releaseLane("release-D", mergedCard("o/svc-a", 3)),
		releaseLane("release-C", mergedCard("o/svc-a", 4)),
		releaseLane("release-B", mergedCard("o/svc-a", 5), mergedCard("o/svc-a", 6), mergedCard("o/svc-a", 7)),
		releaseLane("release-A", mergedCard("o/svc-a", 8)),
	}}
	out := plainBoard(b, 4*24+3*2, false)
	for _, name := range []string{"OPEN", "NOT IN A RELEASE", "release-D", "release-C"} {
		if !strings.Contains(strings.Split(out, "\n")[0], name) {
			t.Errorf("header missing %s", name)
		}
	}
	if !strings.Contains(out, "+2 older releases: release-B (3), release-A (1)") {
		t.Errorf("summary missing:\n%s", out)
	}
	if strings.Contains(strings.Split(out, "\n")[0], "release-B") {
		t.Errorf("release-B should be folded")
	}
}

func TestBoardTooNarrow(t *testing.T) {
	b := board.Board{Lanes: []board.Lane{
		{Kind: board.LaneOpen, Name: "OPEN", Cards: []board.Card{openCard("o/svc-a", 1, "one")}},
		{Kind: board.LanePending, Name: "NOT IN A RELEASE", Cards: []board.Card{mergedCard("o/svc-a", 2)}},
		releaseLane("release-B", mergedCard("o/svc-a", 3)),
		releaseLane("release-A", mergedCard("o/svc-a", 4)),
	}}
	out := plainBoard(b, 10, false)
	head := strings.Split(out, "\n")[0]
	if !strings.Contains(head, "OPEN (1)") || !strings.Contains(head, "NOT IN A RELEASE (1)") {
		t.Errorf("open/pending missing: %q", head)
	}
	if !strings.Contains(out, "+2 older releases: release-B (1), release-A (1)") {
		t.Errorf("summary missing:\n%s", out)
	}
}

func TestBoardAlignsLaneTops(t *testing.T) {
	cut := releaseLane("release-B", mergedCard("o/svc-a", 3))
	cut.CutAt = boardNow.Add(-6 * 24 * time.Hour)
	b := board.Board{Lanes: []board.Lane{
		{Kind: board.LaneOpen, Name: "OPEN", Cards: []board.Card{openCard("o/svc-a", 1, "one")}},
		{Kind: board.LanePending, Name: "NOT IN A RELEASE", Cards: []board.Card{mergedCard("o/svc-a", 2)}},
		cut,
		releaseLane("release-A", mergedCard("o/svc-a", 4)),
	}}
	if got := boxTopRows(t, plainBoard(b, 200, false), 4); !slices.Equal(got, []int{2, 2, 2, 2}) {
		t.Errorf("box top rows = %v, want all 2", got)
	}
	b.Lanes[2].CutAt = time.Time{}
	if got := boxTopRows(t, plainBoard(b, 200, false), 4); !slices.Equal(got, []int{1, 1, 1, 1}) {
		t.Errorf("box top rows without cut lines = %v, want all 1", got)
	}
}

func TestBoardCardLines(t *testing.T) {
	c := mergedCard("o/svc-a", 9)
	c.WorkItemID = "WI-1234"
	c.WorkItemURL = "https://example.com/wi"
	c.Reason = "on main"
	c.InReleaseAt = boardNow.Add(-24 * time.Hour)
	c.Via = "#16450"
	c.PR.Title = "a title that is much longer than the inner width"
	l := releaseLane("release-A", c)
	out := plainBoard(board.Board{Lanes: []board.Lane{l}}, 200, true)
	for _, want := range []string{"WI-1234", "on main", "in release 1d", "↳ via hotfix #16450", "merged 2d", "@alice", "…│"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	c.Via = "cherry-pick"
	out = plainBoard(board.Board{Lanes: []board.Lane{releaseLane("release-A", c)}}, 200, false)
	if !strings.Contains(out, "↳ via cherry-pick") {
		t.Errorf("missing cherry-pick line:\n%s", out)
	}
	if strings.Contains(out, "@alice") {
		t.Errorf("author shown unexpectedly")
	}
}

func TestBoardOpenCardGlyph(t *testing.T) {
	c := openCard("o/svc-a", 1, "one")
	c.CheckGlyph = "✓"
	out := plainBoard(board.Board{Lanes: []board.Lane{{Kind: board.LaneOpen, Name: "OPEN", Cards: []board.Card{c}}}}, 200, false)
	if !strings.Contains(out, "✓ opened 3d") {
		t.Errorf("missing glyph line:\n%s", out)
	}
}

func TestBoardShortRepoNames(t *testing.T) {
	same := board.Board{Lanes: []board.Lane{releaseLane("release-A", mergedCard("o/svc-a", 7), mergedCard("o/svc-b", 8))}}
	out := plainBoard(same, 200, false)
	if !strings.Contains(out, "svc-a#7") || strings.Contains(out, "o/svc-a#7") {
		t.Errorf("want short names:\n%s", out)
	}
	mixed := board.Board{Lanes: []board.Lane{releaseLane("release-A", mergedCard("o/svc-a", 7), mergedCard("p/svc-b", 8))}}
	out = plainBoard(mixed, 200, false)
	if !strings.Contains(out, "o/svc-a#7") {
		t.Errorf("want full names:\n%s", out)
	}
}

func TestBoardEmpty(t *testing.T) {
	out := plainBoard(board.Board{}, 200, false)
	if !strings.Contains(out, "📭 No open or recently merged PRs.") {
		t.Errorf("got %q", out)
	}
}

// boxTopRows returns, for each of lanes columns, the first row whose cell
// starts a card box.
func boxTopRows(t *testing.T, out string, lanes int) []int {
	t.Helper()
	rows := make([]int, lanes)
	for i := range rows {
		rows[i] = -1
	}
	for row, line := range strings.Split(out, "\n") {
		runes := []rune(line)
		for i := range rows {
			col := i * (24 + 2)
			if rows[i] < 0 && col < len(runes) && runes[col] == '┌' {
				rows[i] = row
			}
		}
	}
	return rows
}
