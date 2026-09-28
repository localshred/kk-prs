package watch

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type WatchEntry struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Number  int    `json:"number"`
	URL     string `json:"url"`
	AddedAt string `json:"added-at"`
}

func (w WatchEntry) Ref() Ref { return Ref{Owner: w.Owner, Repo: w.Repo, Number: w.Number, URL: w.URL} }

type Store struct {
	Dir string
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

func (s Store) watchlistPath() string { return filepath.Join(s.Dir, "watchlist.json") }
func (s Store) statePath() string     { return filepath.Join(s.Dir, "state.json") }

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

func (s Store) ReadWatchlist() []WatchEntry {
	return readJSON(s.watchlistPath(), []WatchEntry{})
}
func (s Store) SaveWatchlist(wl []WatchEntry) error { return writeJSON(s.watchlistPath(), wl) }

func (s Store) ReadState() map[string]Snapshot {
	return readJSON(s.statePath(), map[string]Snapshot{})
}
func (s Store) SaveState(state map[string]Snapshot) error { return writeJSON(s.statePath(), state) }
