package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/localshred/kk-prs/internal/board"
	"github.com/localshred/kk-prs/internal/config"
	"github.com/localshred/kk-prs/internal/gh"
	"github.com/localshred/kk-prs/internal/mirror"
	"github.com/localshred/kk-prs/internal/render"
	"github.com/localshred/koolkit/pkg/run"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

const (
	defaultWidth   = 120
	maxOpenMirrors = 4
	mirrorMaxAge   = 5 * time.Minute
)

var checkGlyphs = map[string]string{
	"✅": "✓",
	"❌": "✗",
	"🔄": "↻",
}

func newReleasedCmd(r run.Runner) *cobra.Command {
	var weeks int
	var authors []string
	var refresh, verbose bool
	cmd := &cobra.Command{
		Use:   "released",
		Short: "Show recent PRs as a board of the releases they reached",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runReleased(r, releasedOptions{
				authors: authors, refresh: refresh, verbose: verbose, weeks: weeks,
			}, cmd.OutOrStdout())
		},
	}
	cmd.Flags().IntVar(&weeks, "weeks", 0, "Weeks of merged PRs to include (default kk.prs.weeks, else 4)")
	cmd.Flags().StringArrayVar(&authors, "author", []string{"@me"}, "PR author to include; repeatable")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Fetch every mirror even if recently fetched")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed check status for open PRs")
	return cmd
}

type releasedOptions struct {
	authors []string
	refresh bool
	verbose bool
	weeks   int
}

func boardWidth() int {
	if n, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && n > 0 {
		return n
	}
	if ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ); err == nil && ws.Col > 0 {
		return int(ws.Col)
	}
	return defaultWidth
}

func cacheHome() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache")
}

func collectPRs(c gh.Client, s config.Settings, authors []string, since time.Time) ([]gh.BoardPR, error) {
	var prs []gh.BoardPR
	seen := map[string]bool{}
	for _, author := range authors {
		found, err := c.RecentPRs(author, since, s.WorkItemPattern != nil)
		if err != nil {
			return nil, err
		}
		for _, pr := range found {
			if seen[pr.URL] || s.IgnoreRepos[pr.Repo] {
				continue
			}
			seen[pr.URL] = true
			prs = append(prs, pr)
		}
	}
	return prs, nil
}

func firstLine(err error) string {
	line, _, _ := strings.Cut(strings.TrimSpace(err.Error()), "\n")
	return line
}

func headerLine(authors []string, weeks int) string {
	noun := "weeks"
	if weeks == 1 {
		noun = "week"
	}
	return fmt.Sprintf("🚢 Releases for %s (last %d %s)", strings.Join(authors, ", "), weeks, noun)
}

func openHistories(m mirror.Mirror, s config.Settings, merged []gh.BoardPR, force bool) (map[string]board.History, []string) {
	defaults := map[string]string{}
	var repos []string
	for _, pr := range merged {
		if _, ok := defaults[pr.Repo]; !ok {
			defaults[pr.Repo] = pr.DefaultBranch
			repos = append(repos, pr.Repo)
		}
	}
	histories := map[string]board.History{}
	warnings := make([]string, len(repos))
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, maxOpenMirrors)
	for i, repo := range repos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			opened, err := m.Open(repo, defaults[repo], s.Trunks(defaults[repo]), s.ReleasePatterns, force)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				warnings[i] = fmt.Sprintf("⚠ %s: %s", repo, firstLine(err))
				return
			}
			histories[repo] = opened
		}()
	}
	wg.Wait()
	var out []string
	for _, w := range warnings {
		if w != "" {
			out = append(out, w)
		}
	}
	return histories, out
}

func openCards(c gh.Client, open []gh.BoardPR) []board.Card {
	cards := make([]board.Card, 0, len(open))
	for _, pr := range open {
		checks := filterChecks(c.Checks(pr.Repo, pr.Number))
		cards = append(cards, board.Card{PR: pr, CheckGlyph: checkGlyphs[render.OverallStatus(checks)]})
	}
	return cards
}

func runReleased(r run.Runner, opts releasedOptions, out io.Writer) error {
	s := config.Load(r)
	weeks := opts.weeks
	if weeks <= 0 {
		weeks = s.Weeks
	}
	now := time.Now()
	c := gh.Client{Run: r}
	prs, err := collectPRs(c, s, opts.authors, now.Add(-time.Duration(weeks)*7*24*time.Hour))
	if err != nil {
		return err
	}
	var open, merged []gh.BoardPR
	for _, pr := range prs {
		if pr.State == "OPEN" {
			open = append(open, pr)
		} else {
			merged = append(merged, pr)
		}
	}

	cacheRoot := filepath.Join(cacheHome(), "koolkit")
	m := mirror.Mirror{
		CloneURL: s.CloneURL,
		MaxAge:   mirrorMaxAge,
		Now:      time.Now,
		Root:     filepath.Join(cacheRoot, "prs-mirrors"),
		Run:      r,
	}
	histories, warnings := openHistories(m, s, merged, opts.refresh)
	cache := board.LoadCache(filepath.Join(cacheRoot, "prs", "placements.json"))
	b := board.Place(board.Input{
		Cache:     cache,
		Histories: histories,
		Merged:    merged,
		Open:      openCards(c, open),
		Settings:  s,
	})
	b.Warnings = append(b.Warnings, warnings...)
	if err := cache.Save(); err != nil {
		b.Warnings = append(b.Warnings, "⚠ placement cache: "+firstLine(err))
	}

	fmt.Fprintf(out, "%s\n\n", headerLine(opts.authors, weeks))
	fmt.Fprint(out, render.Board(b, boardWidth(), showAuthor(opts.authors), now))
	for _, w := range b.Warnings {
		fmt.Fprintln(out, w)
	}
	if opts.verbose {
		for _, pr := range open {
			renderVerbose(c, toPR(pr), out)
		}
	}
	return nil
}

func showAuthor(authors []string) bool {
	if len(authors) > 1 {
		return true
	}
	for _, a := range authors {
		if a != "@me" {
			return true
		}
	}
	return false
}

func toPR(b gh.BoardPR) gh.PR {
	pr := gh.PR{Number: b.Number, Title: b.Title, URL: b.URL, CreatedAt: b.CreatedAt.UTC().Format(time.RFC3339)}
	pr.Repository.NameWithOwner = b.Repo
	pr.Author.Login = b.Author
	return pr
}
