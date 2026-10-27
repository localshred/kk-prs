// Package cmd implements the kk-prs command.
package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/localshred/kk-prs/internal/gh"
	"github.com/localshred/kk-prs/internal/render"
	"github.com/localshred/koolkit/pkg/run"
	"github.com/localshred/koolkit/pkg/ui"
	"github.com/spf13/cobra"
)

func Execute() int {
	if err := newRootCmd(run.Default).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "kk-prs:", err)
		return 1
	}
	return 0
}

func newListFlags(cmd *cobra.Command, r run.Runner) {
	var reviews, verbose bool
	cmd.Flags().BoolVarP(&reviews, "reviews", "r", false, "Show PRs requesting your review")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed check status")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		c := gh.Client{Run: r}
		if reviews {
			return runReviews(c, cmd.OutOrStdout())
		}
		return runAuthored(c, verbose, cmd.OutOrStdout())
	}
}

func newRootCmd(r run.Runner) *cobra.Command {
	root := &cobra.Command{
		Use:           "kk-prs",
		Short:         "Show open PRs with statuses or review requests",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	newListFlags(root, r)
	list := &cobra.Command{
		Use:   "list",
		Short: "List your open PRs (same as running kk prs with no subcommand)",
		Args:  cobra.NoArgs,
	}
	newListFlags(list, r)
	root.AddCommand(list, newReleasedCmd(r), newWatchCmd())
	return root
}

func runAuthored(c gh.Client, verbose bool, out io.Writer) error {
	user, err := c.User()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "🔍 Fetching open PRs for %s...\n\n", ui.Cyan("@"+user))
	prs, err := c.Authored()
	if err != nil {
		return err
	}
	sort.Slice(prs, func(i, j int) bool { return prs[i].CreatedAt < prs[j].CreatedAt })
	if len(prs) == 0 {
		fmt.Fprintln(out, "📭 No open PRs found authored by you.")
		return nil
	}
	fmt.Fprintf(out, "📋 Found %d open PR(s):\n", len(prs))
	if verbose {
		for _, pr := range prs {
			renderVerbose(c, pr, out)
		}
		return nil
	}
	var rows [][]string
	for _, pr := range prs {
		checks := filterChecks(c.Checks(pr.Repository.NameWithOwner, pr.Number))
		rows = append(rows, render.CompactRow(pr, render.OverallStatus(checks)))
	}
	ui.PrintTable(out, rows)
	return nil
}

func runReviews(c gh.Client, out io.Writer) error {
	user, err := c.User()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "👀 Fetching review requests for %s...\n\n", ui.Cyan("@"+user))
	prs, err := c.ReviewRequests()
	if err != nil {
		return err
	}
	sort.Slice(prs, func(i, j int) bool { return prs[i].CreatedAt < prs[j].CreatedAt })
	if len(prs) == 0 {
		fmt.Fprintln(out, "📭 No review requests found.")
		return nil
	}
	fmt.Fprintf(out, "📋 Found %d review request(s):\n", len(prs))
	var rows [][]string
	for _, pr := range prs {
		rows = append(rows, render.ReviewRow(pr))
	}
	ui.PrintTable(out, rows)
	return nil
}

func filterChecks(checks []gh.Check) []gh.Check {
	var out []gh.Check
	for _, c := range checks {
		if !render.IgnoredCheck.MatchString(c.Name) {
			out = append(out, c)
		}
	}
	return out
}

func renderVerbose(c gh.Client, pr gh.PR, out io.Writer) {
	checks := render.SortChecks(filterChecks(c.Checks(pr.Repository.NameWithOwner, pr.Number)))
	fmt.Fprintf(out, "\n🔀 %s\n", ui.Hyperlink(pr.URL, fmt.Sprintf("PR #%d: %s", pr.Number, pr.Title)))
	fmt.Fprintf(out, "   📦 %s\n", pr.Repository.NameWithOwner)
	if pr.BaseRefName != "" {
		fmt.Fprintf(out, "   🌿 into %s\n", pr.BaseRefName)
	}
	fmt.Fprintf(out, "   🕐 opened %s ago\n", render.AgeCell(pr.CreatedAt))
	if len(checks) == 0 {
		fmt.Fprintln(out, "   ℹ️  No checks found")
		return
	}
	fmt.Fprintln(out, "   📊 Checks:")
	for _, ch := range checks {
		fmt.Fprintf(out, "   %s %s\n", checkSymbol(ch.State), ch.Name)
	}
}

func checkSymbol(state string) string {
	switch state {
	case "SUCCESS":
		return "✅"
	case "ERROR", "FAILURE":
		return "❌"
	case "PENDING", "IN_PROGRESS":
		return "🔄"
	case "QUEUED":
		return "⏳"
	case "SKIPPED":
		return "⏭️"
	case "CANCELLED":
		return "🚫"
	default:
		return "❓"
	}
}
