package watch

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Store persists the watchlist and per-PR snapshots as JSON files under Dir.
type Store struct {
	Dir string
}

// WatchEntry is one PR on the watchlist as persisted to disk.
type WatchEntry struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Number  int    `json:"number"`
	URL     string `json:"url"`
	AddedAt string `json:"added-at"`
}

// NewStore places watch state under the koolkit data dir.
func NewStore() Store {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		home, _ := os.UserHomeDir()
		data = filepath.Join(home, ".local", "share")
	}
	return Store{Dir: filepath.Join(data, "koolkit", "prs-watch")}
}

// Ref returns the PR reference identifying this entry.
func (w WatchEntry) Ref() Ref {
	return Ref{Owner: w.Owner, Repo: w.Repo, Number: w.Number, URL: w.URL}
}

// ReadState returns the persisted per-PR snapshots, or an empty map when the
// file is missing or malformed.
func (s Store) ReadState() map[string]Snapshot {
	return readJSON(s.statePath(), map[string]Snapshot{})
}

// ReadWatchlist returns the persisted watchlist, or an empty slice when the
// file is missing or malformed.
func (s Store) ReadWatchlist() []WatchEntry {
	return readJSON(s.watchlistPath(), []WatchEntry{})
}

// SaveState writes the per-PR snapshots to disk.
func (s Store) SaveState(state map[string]Snapshot) error {
	return writeJSON(s.statePath(), state)
}

// SaveWatchlist writes the watchlist to disk.
func (s Store) SaveWatchlist(wl []WatchEntry) error {
	return writeJSON(s.watchlistPath(), wl)
}

func readJSON[T any](path string, zero T) T {
	data, err := os.ReadFile(path)
	if err != nil {
		return zero
	}
	var v T
	if json.Unmarshal(data, &v) != nil {
		return zero
	}
	return v
}

func (s Store) statePath() string { return filepath.Join(s.Dir, "state.json") }

func (s Store) watchlistPath() string { return filepath.Join(s.Dir, "watchlist.json") }

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
