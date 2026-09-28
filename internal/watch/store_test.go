package watch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWatchlistRoundTrip(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	in := []WatchEntry{{Owner: "o", Repo: "r", Number: 7, URL: "u", AddedAt: "t"}}
	if err := s.SaveWatchlist(in); err != nil {
		t.Fatal(err)
	}
	got := s.ReadWatchlist()
	if len(got) != 1 || got[0].Number != 7 {
		t.Fatalf("roundtrip = %+v", got)
	}
}

func TestStateRoundTrip(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.SaveState(map[string]Snapshot{"o/r#7": {State: "OPEN", Checks: "passing"}}); err != nil {
		t.Fatal(err)
	}
	got := s.ReadState()
	if got["o/r#7"].Checks != "passing" {
		t.Fatalf("state = %+v", got)
	}
}

func TestReadMissingIsEmpty(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if len(s.ReadWatchlist()) != 0 || len(s.ReadState()) != 0 {
		t.Fatal("missing files should read empty")
	}
}

func TestReadMalformedIsEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "watchlist.json"), []byte("{ not json"), 0o644)
	if len(Store{Dir: dir}.ReadWatchlist()) != 0 {
		t.Fatal("malformed watchlist should read empty")
	}
}
