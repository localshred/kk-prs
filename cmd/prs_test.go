package cmd

import (
	"bytes"
	"testing"

	"github.com/localshred/kk-prs/internal/gh"
	"github.com/localshred/koolkit/pkg/run"
)

func TestRunAuthoredEmpty(t *testing.T) {
	f := &run.Fake{Results: map[string]run.Result{
		"gh api user --jq .login": {Stdout: "bj\n"},
		"gh search prs --author=@me --state=open --json number,title,author,repository,url,createdAt --limit 50": {Stdout: "[]"},
	}}
	var out bytes.Buffer
	if err := runAuthored(gh.Client{Run: f}, false, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("No open PRs")) {
		t.Fatalf("out = %q", out.String())
	}
}

func TestRunAuthoredRendersRow(t *testing.T) {
	f := &run.Fake{Results: map[string]run.Result{
		"gh api user --jq .login": {Stdout: "bj\n"},
		"gh search prs --author=@me --state=open --json number,title,author,repository,url,createdAt --limit 50": {
			Stdout: `[{"number":7,"title":"Fix bug","url":"u","createdAt":"2026-01-01T00:00:00Z","repository":{"nameWithOwner":"o/r"}}]`,
		},
		"gh pr checks 7 --repo o/r --json name,state,link,workflow": {Stdout: "[]"},
	}}
	var out bytes.Buffer
	if err := runAuthored(gh.Client{Run: f}, false, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("o/r#7")) || !bytes.Contains(out.Bytes(), []byte("Fix bug")) {
		t.Fatalf("out = %q", out.String())
	}
}
