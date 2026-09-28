package watch

import "os"

type Fetcher interface {
	Fetch(url string) (PRView, error)
}

type Notifier interface {
	Notify(key, title, message, url string) error
}

type Watcher struct {
	Store  Store
	Fetch  Fetcher
	Notify Notifier
	Now    func() string
}

func isClosed(state string) bool { return state == "MERGED" || state == "CLOSED" }

// Tick checks every watched PR once: notify on change, persist fresh
// snapshots, drop merged/closed. A per-PR fetch error is logged-over: the
// PR keeps its old snapshot and stays on the list.
func (w Watcher) Tick() error {
	wl := w.Store.ReadWatchlist()
	if len(wl) == 0 {
		return nil
	}
	state := w.Store.ReadState()
	var kept []WatchEntry
	for _, entry := range wl {
		key := entry.Ref().Key()
		view, err := w.Fetch.Fetch(entry.URL)
		if err != nil {
			kept = append(kept, entry)
			continue
		}
		snap := ToSnapshot(view)
		snap.CheckedAt = w.Now()
		if old, ok := state[key]; ok {
			if changes := ComputeChanges(old, snap); len(changes) > 0 {
				w.Notify.Notify(key, snap.Title, join(changes, " · "), snap.URL)
			}
		}
		if isClosed(snap.State) {
			delete(state, key)
			continue
		}
		state[key] = snap
		kept = append(kept, entry)
	}
	if err := w.Store.SaveState(state); err != nil {
		return err
	}
	return w.Store.SaveWatchlist(kept)
}

// Add seeds a PR's snapshot without notifying. Returns a status message.
func (w Watcher) Add(ref Ref) (string, error) {
	wl := w.Store.ReadWatchlist()
	key := ref.Key()
	for _, e := range wl {
		if e.Ref().Key() == key {
			return "already watching " + key, nil
		}
	}
	view, err := w.Fetch.Fetch(ref.URL)
	if err != nil {
		return "", err
	}
	if isClosed(view.State) {
		return key + " is already closed — not adding", nil
	}
	snap := ToSnapshot(view)
	snap.CheckedAt = w.Now()
	state := w.Store.ReadState()
	state[key] = snap
	if err := w.Store.SaveState(state); err != nil {
		return "", err
	}
	wl = append(wl, WatchEntry{Owner: ref.Owner, Repo: ref.Repo, Number: ref.Number, URL: ref.URL, AddedAt: w.Now()})
	if err := w.Store.SaveWatchlist(wl); err != nil {
		return "", err
	}
	home, _ := os.UserHomeDir()
	_ = EnsureAgent(home)
	return "now watching " + key + " — " + view.Title, nil
}

func (w Watcher) Remove(ref Ref) bool {
	key := ref.Key()
	wl := w.Store.ReadWatchlist()
	var kept []WatchEntry
	for _, e := range wl {
		if e.Ref().Key() != key {
			kept = append(kept, e)
		}
	}
	if len(kept) == len(wl) {
		return false
	}
	w.Store.SaveWatchlist(kept)
	state := w.Store.ReadState()
	delete(state, key)
	w.Store.SaveState(state)
	return true
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
