// kk-prs/internal/watch/watch_test.go
package watch

import (
	"errors"
	"testing"
)

type mapFetcher map[string]PRView

func (m mapFetcher) Fetch(url string) (PRView, error) {
	if v, ok := m[url]; ok {
		return v, nil
	}
	return PRView{}, errors.New("fetch failed")
}

type recordNotifier struct{ keys []string }

func (r *recordNotifier) Notify(key, title, message, url string) error {
	r.keys = append(r.keys, key)
	return nil
}

func fixedNow() string { return "2026-01-01T00:00:00Z" }

func TestTickNotifiesOnChangeAndDropsMerged(t *testing.T) {
	dir := t.TempDir()
	s := Store{Dir: dir}
	s.SaveWatchlist([]WatchEntry{{Owner: "o", Repo: "r", Number: 7, URL: "u7"}})
	s.SaveState(map[string]Snapshot{"o/r#7": {State: "OPEN", Comments: 0}})
	fetch := mapFetcher{"u7": {State: "MERGED", Title: "T", URL: "u7"}}
	n := &recordNotifier{}
	w := Watcher{Store: s, Fetch: fetch, Notify: n, Now: fixedNow}
	if err := w.Tick(); err != nil {
		t.Fatal(err)
	}
	if len(n.keys) != 1 || n.keys[0] != "o/r#7" {
		t.Fatalf("notified keys = %v", n.keys)
	}
	if len(s.ReadWatchlist()) != 0 {
		t.Fatal("merged PR should be dropped from the watchlist")
	}
}

func TestTickContinuesPastFetchError(t *testing.T) {
	dir := t.TempDir()
	s := Store{Dir: dir}
	s.SaveWatchlist([]WatchEntry{
		{Owner: "o", Repo: "r", Number: 1, URL: "bad"},
		{Owner: "o", Repo: "r", Number: 2, URL: "u2"},
	})
	s.SaveState(map[string]Snapshot{"o/r#2": {State: "OPEN", Comments: 0}})
	fetch := mapFetcher{"u2": {State: "OPEN", Comments: []comment{{}, {}}, Title: "T", URL: "u2"}}
	n := &recordNotifier{}
	w := Watcher{Store: s, Fetch: fetch, Notify: n, Now: fixedNow}
	if err := w.Tick(); err != nil {
		t.Fatal(err)
	}
	if len(n.keys) != 1 || n.keys[0] != "o/r#2" {
		t.Fatalf("healthy PR should still notify; keys = %v", n.keys)
	}
	if len(s.ReadWatchlist()) != 2 {
		t.Fatal("failed-fetch PR should remain on the watchlist")
	}
}
