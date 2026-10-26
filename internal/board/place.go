package board

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/localshred/kk-prs/internal/config"
	"github.com/localshred/kk-prs/internal/gh"
)

// Input is everything Place needs to build a board.
type Input struct {
	Cache *Cache
	// Histories is keyed by repo; a missing repo means its fetch failed.
	Histories map[string]History
	Merged    []gh.BoardPR
	// Open cards arrive with PR and CheckGlyph set.
	Open     []Card
	Settings config.Settings
}

type placement struct {
	cutAt   time.Time
	release string
}

// Place puts each open PR in the open lane and each merged PR in the earliest
// release it reached, by ancestry or by cherry-pick. Merged PRs that reached
// no release go in the pending lane with a reason.
func Place(in Input) Board {
	var open, pending []Card
	releaseCards := map[string][]Card{}
	cutAts := map[string]time.Time{}

	for _, c := range in.Open {
		c.WorkItemID, c.WorkItemURL = in.Settings.WorkItemLink(c.PR.Title, c.PR.Body)
		open = append(open, c)
	}

	for _, pr := range in.Merged {
		card := Card{PR: pr}
		card.WorkItemID, card.WorkItemURL = in.Settings.WorkItemLink(pr.Title, pr.Body)
		h, ok := in.Histories[pr.Repo]
		if !ok {
			card.Reason = ReasonUnknown
			pending = append(pending, card)
			continue
		}
		releases := h.Releases()
		if len(releases) == 0 {
			card.Reason = ReasonNoReleases
			pending = append(pending, card)
			continue
		}
		p, found := placeMerged(in.Cache, h, releases, &card)
		if !found {
			card.Reason = pendingReason(in.Settings, pr)
			pending = append(pending, card)
			continue
		}
		releaseCards[p.release] = append(releaseCards[p.release], card)
		if p.cutAt.After(cutAts[p.release]) {
			cutAts[p.release] = p.cutAt
		}
	}

	var board Board
	if len(open) > 0 {
		sortCards(open, func(c Card) time.Time { return c.PR.CreatedAt })
		board.Lanes = append(board.Lanes, Lane{Cards: open, Kind: LaneOpen})
	}
	if len(pending) > 0 {
		sortCards(pending, mergedAt)
		board.Lanes = append(board.Lanes, Lane{Cards: pending, Kind: LanePending})
	}
	names := make([]string, 0, len(releaseCards))
	for name := range releaseCards {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return VersionLess(names[j], names[i]) })
	for _, name := range names {
		cards := releaseCards[name]
		sortCards(cards, mergedAt)
		board.Lanes = append(board.Lanes, Lane{
			Cards: cards,
			CutAt: cutAts[name],
			Kind:  LaneRelease,
			Name:  name,
		})
	}
	return board
}

func ancestry(cache *Cache, h History, releases []Release, pr gh.BoardPR) (int, bool) {
	if pr.MergeOID == "" {
		return 0, false
	}
	key := fmt.Sprintf("%s#%d", pr.Repo, pr.Number)
	if cached, ok := cache.Get(key); ok {
		if i := slices.IndexFunc(releases, func(r Release) bool { return r.Name == cached }); i >= 0 {
			return i, true
		}
	}
	for i := len(releases) - 1; i >= 0; i-- {
		if h.Contains(pr.MergeOID, releases[i].Name) {
			cache.Put(key, releases[i].Name)
			return i, true
		}
	}
	return 0, false
}

func mergedAt(c Card) time.Time {
	return c.PR.MergedAt
}

func pendingReason(s config.Settings, pr gh.BoardPR) string {
	if slices.Contains(s.Trunks(pr.DefaultBranch), pr.BaseRef) {
		return ReasonOnTrunk(pr.BaseRef)
	}
	return ReasonWaiting(pr.BaseRef)
}

func placeMerged(cache *Cache, h History, releases []Release, card *Card) (placement, bool) {
	pr := card.PR
	ancestorIdx, hasAncestor := ancestry(cache, h, releases, pr)

	olderThan := -1
	if hasAncestor {
		olderThan = ancestorIdx
	}
	for i := len(releases) - 1; i > olderThan; i-- {
		pick, ok := h.FindPick(pr.MergeOID, pr.Number, releases[i].Name)
		if !ok {
			continue
		}
		card.InReleaseAt = pick.At
		card.Via = "cherry-pick"
		if pick.Via != 0 {
			card.Via = fmt.Sprintf("#%d", pick.Via)
		}
		return placement{cutAt: releases[i].CutAt, release: releases[i].Name}, true
	}
	if !hasAncestor {
		return placement{}, false
	}
	r := releases[ancestorIdx]
	if at, ok := h.IntroducedAt(pr.MergeOID, r.Name); ok {
		card.InReleaseAt = at
	}
	return placement{cutAt: r.CutAt, release: r.Name}, true
}

func sortCards(cards []Card, when func(Card) time.Time) {
	sort.SliceStable(cards, func(i, j int) bool {
		a, b := cards[i], cards[j]
		if !when(a).Equal(when(b)) {
			return when(a).After(when(b))
		}
		if a.PR.Repo != b.PR.Repo {
			return a.PR.Repo < b.PR.Repo
		}
		return a.PR.Number > b.PR.Number
	})
}
