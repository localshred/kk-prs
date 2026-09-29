// kk-prs/cmd/watch.go
package cmd

import (
	"fmt"
	"time"

	"github.com/localshred/kk-prs/internal/gh"
	"github.com/localshred/kk-prs/internal/watch"
	"github.com/localshred/koolkit/pkg/run"
	"github.com/spf13/cobra"
)

func listWatched(w watch.Watcher, cmd *cobra.Command) error {
	wl := w.Store.ReadWatchlist()
	if len(wl) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "📭 Not watching any PRs. Add one with: kk prs watch <pr-url>")
		return nil
	}
	state := w.Store.ReadState()
	for _, e := range wl {
		snap := state[e.Ref().Key()]
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s\n", e.Ref().Key(), snap.Checks, snap.Title)
	}
	return nil
}

func newWatchCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "watch [pr-url]",
		Short: "Watch PRs for updates and notify on change",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := newWatcher()
			if len(args) == 0 {
				return listWatched(w, cmd)
			}
			ref, ok := watch.ParseRef(args[0])
			if !ok {
				return fmt.Errorf("not a PR reference: %s", args[0])
			}
			msg, err := w.Add(ref)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "👀", msg)
			return nil
		},
	}
	c.AddCommand(&cobra.Command{
		Use: "tick", Short: "Check all watched PRs now (run by launchd)",
		RunE: func(cmd *cobra.Command, _ []string) error { return newWatcher().Tick() },
	})
	c.AddCommand(&cobra.Command{
		Use: "list", Short: "List watched PRs",
		RunE: func(cmd *cobra.Command, _ []string) error { return listWatched(newWatcher(), cmd) },
	})
	c.AddCommand(&cobra.Command{
		Use: "rm <pr-url>", Short: "Stop watching a PR", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, ok := watch.ParseRef(args[0])
			if !ok {
				return fmt.Errorf("not a PR reference: %s", args[0])
			}
			if newWatcher().Remove(ref) {
				fmt.Fprintln(cmd.OutOrStdout(), "🗑️  stopped watching", ref.Key())
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "not watching", ref.Key())
			}
			return nil
		},
	})
	return c
}

func newWatcher() watch.Watcher {
	c := gh.Client{Run: run.Default}
	return watch.Watcher{
		Store:  watch.NewStore(),
		Fetch:  watch.NewGHFetcher(c),
		Notify: watch.NewBeeepNotifier(),
		Now:    nowISO,
	}
}

func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }
