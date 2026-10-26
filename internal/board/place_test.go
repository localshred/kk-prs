package board

import (
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/localshred/kk-prs/internal/config"
	"github.com/localshred/kk-prs/internal/gh"
)

type fakeHistory struct {
	containCalls    []string
	contains        map[string]bool
	findPickCalls   int
	introduced      map[string]time.Time
	introducedCalls int
	picks           map[string]Pick
	releases        []Release
	releasesCalls   int
}

func (f *fakeHistory) Contains(oid, release string) bool {
	f.containCalls = append(f.containCalls, oid)
	return f.contains[oid+"@"+release]
}

func (f *fakeHistory) FindPick(oid string, number int, release string) (Pick, bool) {
	f.findPickCalls++
	p, ok := f.picks[oid+"@"+release]
	return p, ok
}

func (f *fakeHistory) IntroducedAt(oid, release string) (time.Time, bool) {
	f.introducedCalls++
	t, ok := f.introduced[oid+"@"+release]
	return t, ok
}

func (f *fakeHistory) Releases() []Release {
	f.releasesCalls++
	return f.releases
}

var (
	t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 = t0.Add(24 * time.Hour)
	t2 = t0.Add(48 * time.Hour)
)

func newHistory(names ...string) *fakeHistory {
	h := &fakeHistory{
		contains:   map[string]bool{},
		introduced: map[string]time.Time{},
		picks:      map[string]Pick{},
	}
	for _, n := range names {
		h.releases = append(h.releases, Release{Name: n, CutAt: t0})
	}
	return h
}

func mergedPR(repo string, number int, oid, base string) gh.BoardPR {
	return gh.BoardPR{
		BaseRef:       base,
		DefaultBranch: "main",
		MergeOID:      oid,
		MergedAt:      t1,
		Number:        number,
		Repo:          repo,
		State:         "MERGED",
	}
}

func placeOne(t *testing.T, h *fakeHistory, pr gh.BoardPR, cache *Cache) Card {
	t.Helper()
	if cache == nil {
		cache = LoadCache(filepath.Join(t.TempDir(), "c.json"))
	}
	histories := map[string]History{}
	if h != nil {
		histories[pr.Repo] = h
	}
	b := Place(Input{Cache: cache, Histories: histories, Merged: []gh.BoardPR{pr}})
	if len(b.Lanes) != 1 || len(b.Lanes[0].Cards) != 1 {
		t.Fatalf("want one lane with one card, got %+v", b.Lanes)
	}
	return b.Lanes[0].Cards[0]
}

func placeLane(t *testing.T, h *fakeHistory, pr gh.BoardPR, cache *Cache) Lane {
	t.Helper()
	if cache == nil {
		cache = LoadCache(filepath.Join(t.TempDir(), "c.json"))
	}
	b := Place(Input{
		Cache:     cache,
		Histories: map[string]History{pr.Repo: h},
		Merged:    []gh.BoardPR{pr},
	})
	if len(b.Lanes) != 1 {
		t.Fatalf("want one lane, got %+v", b.Lanes)
	}
	return b.Lanes[0]
}

func TestPlaceOldestContaining(t *testing.T) {
	h := newHistory("release-B", "release-A")
	h.contains["abc@release-A"] = true
	h.contains["abc@release-B"] = true
	lane := placeLane(t, h, mergedPR("o/svc-a", 1, "abc", "main"), nil)
	if lane.Kind != LaneRelease || lane.Name != "release-A" {
		t.Fatalf("lane = %+v, want release-A", lane)
	}
}

func TestPlaceIntroducedAt(t *testing.T) {
	h := newHistory("release-A")
	h.contains["abc@release-A"] = true
	h.introduced["abc@release-A"] = t2
	card := placeLane(t, h, mergedPR("o/svc-a", 1, "abc", "main"), nil).Cards[0]
	if !card.InReleaseAt.Equal(t2) {
		t.Fatalf("InReleaseAt = %v, want %v", card.InReleaseAt, t2)
	}
}

func TestPlaceCherryPickBeatsAncestry(t *testing.T) {
	h := newHistory("release-B", "release-A")
	h.contains["abc@release-B"] = true
	h.picks["abc@release-A"] = Pick{At: t2, Via: 6}
	lane := placeLane(t, h, mergedPR("o/svc-a", 1, "abc", "main"), nil)
	if lane.Name != "release-A" {
		t.Fatalf("lane = %q, want release-A", lane.Name)
	}
	card := lane.Cards[0]
	if card.Via != "#6" || !card.InReleaseAt.Equal(t2) {
		t.Fatalf("card = %+v", card)
	}
}

func TestPlacePickWithoutVia(t *testing.T) {
	h := newHistory("release-A")
	h.picks["abc@release-A"] = Pick{At: t2}
	card := placeLane(t, h, mergedPR("o/svc-a", 1, "abc", "main"), nil).Cards[0]
	if card.Via != "cherry-pick" {
		t.Fatalf("Via = %q", card.Via)
	}
}

func TestPlacePickNewerThanAncestryIgnored(t *testing.T) {
	h := newHistory("release-B", "release-A")
	h.contains["abc@release-A"] = true
	h.picks["abc@release-B"] = Pick{At: t2, Via: 6}
	lane := placeLane(t, h, mergedPR("o/svc-a", 1, "abc", "main"), nil)
	if lane.Name != "release-A" || lane.Cards[0].Via != "" {
		t.Fatalf("lane = %+v", lane)
	}
}

func TestPlaceReasons(t *testing.T) {
	cases := []struct {
		name string
		h    *fakeHistory
		base string
		want string
	}{
		{"trunk", newHistory("release-A"), "main", "on main"},
		{"waiting", newHistory("release-A"), "hotfix-B", "→ hotfix-B · waiting"},
		{"unknown", nil, "main", "release status unknown"},
		{"no releases", newHistory(), "main", "no releases"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			card := placeOne(t, c.h, mergedPR("o/svc-a", 1, "abc", c.base), nil)
			if card.Reason != c.want {
				t.Fatalf("Reason = %q, want %q", card.Reason, c.want)
			}
		})
	}
}

func TestPlaceUsesCache(t *testing.T) {
	h := newHistory("release-B", "release-A")
	cache := LoadCache(filepath.Join(t.TempDir(), "c.json"))
	cache.Put("o/svc-a#1", "release-A")
	lane := placeLane(t, h, mergedPR("o/svc-a", 1, "abc", "main"), cache)
	if lane.Name != "release-A" {
		t.Fatalf("lane = %q", lane.Name)
	}
	if len(h.containCalls) != 0 {
		t.Fatalf("Contains called: %v", h.containCalls)
	}
}

func TestPlaceStaleCache(t *testing.T) {
	h := newHistory("release-B", "release-A")
	h.contains["abc@release-B"] = true
	cache := LoadCache(filepath.Join(t.TempDir(), "c.json"))
	cache.Put("o/svc-a#1", "release-Z")
	lane := placeLane(t, h, mergedPR("o/svc-a", 1, "abc", "main"), cache)
	if lane.Name != "release-B" {
		t.Fatalf("lane = %q, want release-B", lane.Name)
	}
	if got, _ := cache.Get("o/svc-a#1"); got != "release-B" {
		t.Fatalf("cache = %q, want release-B", got)
	}
}

func TestPlacePutsCache(t *testing.T) {
	h := newHistory("release-A")
	h.contains["abc@release-A"] = true
	cache := LoadCache(filepath.Join(t.TempDir(), "c.json"))
	placeLane(t, h, mergedPR("o/svc-a", 1, "abc", "main"), cache)
	if got, _ := cache.Get("o/svc-a#1"); got != "release-A" {
		t.Fatalf("cache = %q", got)
	}
}

func TestPlaceEmptyMergeOID(t *testing.T) {
	h := newHistory("release-A")
	card := placeOne(t, h, mergedPR("o/svc-a", 1, "", "main"), nil)
	if card.Reason != "on main" {
		t.Fatalf("Reason = %q", card.Reason)
	}
	calls := map[string]int{
		"Contains":     len(h.containCalls),
		"FindPick":     h.findPickCalls,
		"IntroducedAt": h.introducedCalls,
		"Releases":     h.releasesCalls,
	}
	for method, n := range calls {
		if n != 0 {
			t.Errorf("%s called %d times", method, n)
		}
	}
}

func TestPlaceMergesReposByReleaseName(t *testing.T) {
	a, b := newHistory("release-B"), newHistory("release-B")
	a.releases[0].CutAt = t1
	b.releases[0].CutAt = t2
	c := newHistory("release-B")
	c.releases[0].CutAt = t2.Add(time.Hour)
	a.contains["aaa@release-B"] = true
	b.contains["bbb@release-B"] = true
	board := Place(Input{
		Cache:     LoadCache(filepath.Join(t.TempDir(), "c.json")),
		Histories: map[string]History{"o/svc-a": a, "o/svc-b": b, "o/svc-c": c},
		Merged: []gh.BoardPR{
			mergedPR("o/svc-a", 1, "aaa", "main"),
			mergedPR("o/svc-b", 2, "bbb", "main"),
		},
	})
	if len(board.Lanes) != 1 {
		t.Fatalf("lanes = %+v", board.Lanes)
	}
	lane := board.Lanes[0]
	if len(lane.Cards) != 2 || !lane.CutAt.Equal(c.releases[0].CutAt) {
		t.Fatalf("lane = %+v", lane)
	}
}

func TestPlaceLaneOrder(t *testing.T) {
	h := newHistory("release-B", "release-A")
	h.contains["a@release-A"] = true
	h.contains["b@release-B"] = true
	open := gh.BoardPR{Number: 9, Repo: "o/svc-a", State: "OPEN", CreatedAt: t1}
	board := Place(Input{
		Cache:     LoadCache(filepath.Join(t.TempDir(), "c.json")),
		Histories: map[string]History{"o/svc-a": h},
		Merged: []gh.BoardPR{
			mergedPR("o/svc-a", 1, "a", "main"),
			mergedPR("o/svc-a", 2, "b", "main"),
			mergedPR("o/svc-a", 3, "c", "main"),
		},
		Open: []Card{{PR: open, CheckGlyph: "✓"}},
	})
	want := []string{"OPEN", "NOT IN A RELEASE", "release-B", "release-A"}
	kinds := []LaneKind{LaneOpen, LanePending, LaneRelease, LaneRelease}
	if len(board.Lanes) != len(want) {
		t.Fatalf("lanes = %+v", board.Lanes)
	}
	for i, l := range board.Lanes {
		if l.Name != want[i] || l.Kind != kinds[i] {
			t.Fatalf("lane %d = %q kind %d", i, l.Name, l.Kind)
		}
	}
}

func TestPlaceOmitsEmptyLanes(t *testing.T) {
	board := Place(Input{Cache: LoadCache(filepath.Join(t.TempDir(), "c.json"))})
	if len(board.Lanes) != 0 {
		t.Fatalf("lanes = %+v", board.Lanes)
	}
}

func TestPlaceCardOrder(t *testing.T) {
	h := newHistory("release-A")
	h.contains["a@release-A"] = true
	h.contains["b@release-A"] = true
	older := mergedPR("o/svc-a", 1, "a", "main")
	newer := mergedPR("o/svc-a", 2, "b", "main")
	newer.MergedAt = t2
	board := Place(Input{
		Cache:     LoadCache(filepath.Join(t.TempDir(), "c.json")),
		Histories: map[string]History{"o/svc-a": h},
		Merged:    []gh.BoardPR{older, newer},
	})
	cards := board.Lanes[0].Cards
	if cards[0].PR.Number != 2 || cards[1].PR.Number != 1 {
		t.Fatalf("order = %d, %d", cards[0].PR.Number, cards[1].PR.Number)
	}
}

func TestPlaceWorkItem(t *testing.T) {
	h := newHistory("release-A")
	pr := mergedPR("o/svc-a", 1, "abc", "main")
	pr.Title = "Fix thing WI-1234"
	board := Place(Input{
		Cache:     LoadCache(filepath.Join(t.TempDir(), "c.json")),
		Histories: map[string]History{"o/svc-a": h},
		Merged:    []gh.BoardPR{pr},
		Settings: config.Settings{
			WorkItemPattern: regexp.MustCompile(`WI-\d+`),
			WorkItemURL:     "https://example.com/{id}",
		},
	})
	got := board.Lanes[0].Cards[0]
	if got.WorkItemID != "WI-1234" || got.WorkItemURL != "https://example.com/WI-1234" {
		t.Fatalf("card = %+v", got)
	}
}
