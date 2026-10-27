package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/localshred/kk-prs/internal/gh"
	"github.com/localshred/koolkit/pkg/run"
	"github.com/localshred/koolkit/pkg/ui"
)

const listSearchJSON = `{"data":{"search":{"nodes":[{"number":7,"title":"Fix bug","url":"u",` +
	`"createdAt":"2026-01-01T00:00:00Z","baseRefName":"release-B","author":{"login":"alice"},` +
	`"repository":{"nameWithOwner":"o/r"}}]}}}`

type listRunner struct {
	search string
}

func (l listRunner) Look(string) (string, bool) { return "", true }

func (l listRunner) Run(name string, args ...string) (run.Result, error) {
	switch {
	case len(args) >= 2 && args[0] == "api" && args[1] == "user":
		return run.Result{Stdout: "bj\n"}, nil
	case len(args) >= 2 && args[0] == "api" && args[1] == "graphql":
		return run.Result{Stdout: l.search}, nil
	default:
		return run.Result{Stdout: "[]"}, nil
	}
}

func TestRunAuthoredEmpty(t *testing.T) {
	f := listRunner{search: `{"data":{"search":{"nodes":[]}}}`}
	var out bytes.Buffer
	if err := runAuthored(gh.Client{Run: f}, false, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("No open PRs")) {
		t.Fatalf("out = %q", out.String())
	}
}

func TestRunAuthoredRendersRow(t *testing.T) {
	f := listRunner{search: listSearchJSON}
	var out bytes.Buffer
	if err := runAuthored(gh.Client{Run: f}, false, &out); err != nil {
		t.Fatal(err)
	}
	plain := ui.StripANSI(out.String())
	for _, want := range []string{"o/r#7", "→ release-B", "Fix bug"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in %q", want, plain)
		}
	}
}

func TestRunAuthoredVerboseShowsBase(t *testing.T) {
	var out bytes.Buffer
	if err := runAuthored(gh.Client{Run: listRunner{search: listSearchJSON}}, true, &out); err != nil {
		t.Fatal(err)
	}
	if plain := ui.StripANSI(out.String()); !strings.Contains(plain, "into release-B") {
		t.Errorf("verbose output missing base: %q", plain)
	}
}

func TestRunReviewsShowsBase(t *testing.T) {
	var out bytes.Buffer
	if err := runReviews(gh.Client{Run: listRunner{search: listSearchJSON}}, &out); err != nil {
		t.Fatal(err)
	}
	if plain := ui.StripANSI(out.String()); !strings.Contains(plain, "→ release-B") {
		t.Errorf("review output missing base: %q", plain)
	}
}
