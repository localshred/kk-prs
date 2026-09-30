package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/localshred/kk-prs/internal/watch"
	"github.com/localshred/koolkit/pkg/ui"
	"github.com/spf13/cobra"
)

func TestListWatchedEmpty(t *testing.T) {
	w := watch.Watcher{Store: watch.Store{Dir: t.TempDir()}}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := listWatched(w, cmd); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Not watching any PRs") {
		t.Errorf("out = %q", out.String())
	}
}

func TestListWatchedRendersLinkedSortedTable(t *testing.T) {
	store := watch.Store{Dir: t.TempDir()}
	store.SaveWatchlist([]watch.WatchEntry{
		{Owner: "zed", Repo: "b", Number: 2, URL: "https://github.com/zed/b/pull/2"},
		{Owner: "acme", Repo: "a", Number: 1, URL: "https://github.com/acme/a/pull/1"},
	})
	checked := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	store.SaveState(map[string]watch.Snapshot{
		"acme/a#1": {Checks: "passing", ReviewDecision: "APPROVED", Title: "Add thing", CheckedAt: checked},
		"zed/b#2":  {State: "MERGED", Mergeable: "CONFLICTING", Checks: "failing", Title: "Fix bug"},
	})
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := listWatched(watch.Watcher{Store: store}, cmd); err != nil {
		t.Fatal(err)
	}
	raw := out.String()
	if !strings.Contains(raw, "\x1b]8;;https://github.com/acme/a/pull/1\x1b\\") {
		t.Errorf("repo key should be an OSC 8 hyperlink to the PR, got %q", raw)
	}
	plain := ui.StripANSI(raw)
	if !strings.Contains(plain, "Watching 2 PR(s)") {
		t.Errorf("missing header: %q", plain)
	}
	if strings.Index(plain, "acme/a#1") > strings.Index(plain, "zed/b#2") {
		t.Errorf("rows should sort by key: %q", plain)
	}
	if !strings.Contains(plain, "✅ approved") {
		t.Errorf("missing status glyphs for acme/a#1: %q", plain)
	}
	if !strings.Contains(plain, "merged ❌ ⚠️") {
		t.Errorf("missing status glyphs for zed/b#2: %q", plain)
	}
	if !strings.Contains(plain, "—") {
		t.Errorf("unchecked PR should show em dash age: %q", plain)
	}
}

func TestStatusLabel(t *testing.T) {
	cases := []struct {
		snap watch.Snapshot
		want string
	}{
		{watch.Snapshot{}, ""},
		{watch.Snapshot{Checks: "running"}, "🔄"},
		{watch.Snapshot{State: "CLOSED", ReviewDecision: "CHANGES_REQUESTED"}, "closed changes-req"},
	}
	for _, c := range cases {
		if got := statusLabel(c.snap); got != c.want {
			t.Errorf("statusLabel(%+v) = %q, want %q", c.snap, got, c.want)
		}
	}
}
