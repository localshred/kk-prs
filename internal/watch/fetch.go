package watch

import (
	"github.com/localshred/kk-prs/internal/gh"
	"github.com/localshred/koolkit/pkg/run"
)

const prViewFields = "number,title,url,state,mergeable,reviewDecision,comments,reviews,statusCheckRollup"

type ghFetcher struct{ c gh.Client }

// NewGHFetcher returns a Fetcher that uses the gh CLI.
func NewGHFetcher(c gh.Client) Fetcher { return ghFetcher{c: c} }

func (f ghFetcher) Fetch(url string) (PRView, error) {
	var v PRView
	err := runJSON(f.c, &v, "gh", "pr", "view", url, "--json", prViewFields)
	return v, err
}

func runJSON(c gh.Client, v any, name string, args ...string) error {
	return run.JSON(c.Runner(), v, name, args...)
}
