package gh

import (
	"testing"

	"github.com/localshred/koolkit/pkg/run"
)

func TestAuthoredParsesJSON(t *testing.T) {
	f := &run.Fake{Results: map[string]run.Result{
		"gh search prs --author=@me --state=open --json number,title,author,repository,url,createdAt --limit 50": {
			Stdout: `[{"number":7,"title":"Fix","url":"u","createdAt":"2026-01-01T00:00:00Z","author":{"login":"bj"},"repository":{"nameWithOwner":"o/r"}}]`,
		},
	}}
	prs, err := Client{Run: f}.Authored()
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 || prs[0].Number != 7 || prs[0].Repository.NameWithOwner != "o/r" {
		t.Fatalf("authored = %+v", prs)
	}
}

func TestChecksSwallowsError(t *testing.T) {
	f := &run.Fake{Errs: map[string]error{
		"gh pr checks 7 --repo o/r --json name,state,link,workflow": errFake,
	}}
	if got := (Client{Run: f}).Checks("o/r", 7); got != nil {
		t.Fatalf("checks on error should be nil, got %+v", got)
	}
}

var errFake = &fakeErr{}

type fakeErr struct{}

func (*fakeErr) Error() string { return "boom" }
