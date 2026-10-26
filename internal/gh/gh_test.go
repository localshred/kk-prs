package gh

import (
	"errors"
	"strings"
	"testing"
	"time"

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

type ghStub struct {
	results []run.Result
	errs    []error
	calls   [][]string
}

func (s *ghStub) Run(name string, args ...string) (run.Result, error) {
	i := len(s.calls)
	s.calls = append(s.calls, append([]string{name}, args...))
	var res run.Result
	if i < len(s.results) {
		res = s.results[i]
	}
	var err error
	if i < len(s.errs) {
		err = s.errs[i]
	}
	return res, err
}

func (s *ghStub) Look(name string) (string, bool) { return "", false }

const pageOneJSON = `{"data":{
"open":{"pageInfo":{"hasNextPage":false,"endCursor":"o1"},"nodes":[
 {"number":1,"title":"Open one","url":"u1","createdAt":"2026-10-01T10:00:00Z","mergedAt":null,"state":"OPEN","baseRefName":"main","mergeCommit":null,"author":{"login":"alice"},"repository":{"nameWithOwner":"o/svc-a","defaultBranchRef":{"name":"main"}}}]},
"merged":{"pageInfo":{"hasNextPage":false,"endCursor":"m1"},"nodes":[
 {"number":2,"title":"Merged two","url":"u2","createdAt":"2026-10-02T10:00:00Z","mergedAt":"2026-10-03T12:30:00Z","state":"MERGED","baseRefName":"release-A","mergeCommit":{"oid":"abc123"},"author":{"login":"alice"},"repository":{"nameWithOwner":"o/svc-a","defaultBranchRef":{"name":"main"}}}]}}}`

func TestRecentPRsParsesBothSearches(t *testing.T) {
	stub := &ghStub{results: []run.Result{{Stdout: pageOneJSON}}}
	prs, err := Client{Run: stub}.RecentPRs("alice", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 2 {
		t.Fatalf("got %d PRs: %+v", len(prs), prs)
	}
	open, merged := prs[0], prs[1]
	if open.Number != 1 || open.State != "OPEN" || !open.MergedAt.IsZero() || open.MergeOID != "" || open.Author != "alice" {
		t.Fatalf("open = %+v", open)
	}
	wantMergedAt := time.Date(2026, 10, 3, 12, 30, 0, 0, time.UTC)
	if merged.Number != 2 || merged.MergeOID != "abc123" || !merged.MergedAt.Equal(wantMergedAt) ||
		merged.BaseRef != "release-A" || merged.DefaultBranch != "main" || merged.Repo != "o/svc-a" ||
		merged.State != "MERGED" || merged.URL != "u2" || merged.Title != "Merged two" {
		t.Fatalf("merged = %+v", merged)
	}
}

func TestRecentPRsPaginates(t *testing.T) {
	first := `{"data":{"open":{"pageInfo":{"hasNextPage":false,"endCursor":"o1"},"nodes":[
{"number":1,"title":"t","url":"u","createdAt":"2026-10-01T10:00:00Z","mergedAt":null,"state":"OPEN","baseRefName":"main","author":{"login":"alice"},"repository":{"nameWithOwner":"o/svc-a","defaultBranchRef":{"name":"main"}}}]},
"merged":{"pageInfo":{"hasNextPage":true,"endCursor":"m1"},"nodes":[
{"number":2,"title":"t","url":"u","createdAt":"2026-10-01T10:00:00Z","mergedAt":"2026-10-02T10:00:00Z","state":"MERGED","baseRefName":"main","author":{"login":"alice"},"repository":{"nameWithOwner":"o/svc-a","defaultBranchRef":{"name":"main"}}}]}}}`
	second := `{"data":{"open":{"pageInfo":{"hasNextPage":false,"endCursor":"o1"},"nodes":[
{"number":1,"title":"t","url":"u","createdAt":"2026-10-01T10:00:00Z","mergedAt":null,"state":"OPEN","baseRefName":"main","author":{"login":"alice"},"repository":{"nameWithOwner":"o/svc-a","defaultBranchRef":{"name":"main"}}}]},
"merged":{"pageInfo":{"hasNextPage":false,"endCursor":"m2"},"nodes":[
{"number":3,"title":"t","url":"u","createdAt":"2026-10-01T10:00:00Z","mergedAt":"2026-10-04T10:00:00Z","state":"MERGED","baseRefName":"main","author":{"login":"alice"},"repository":{"nameWithOwner":"o/svc-b","defaultBranchRef":{"name":"main"}}}]}}}`
	stub := &ghStub{results: []run.Result{{Stdout: first}, {Stdout: second}}}
	prs, err := Client{Run: stub}.RecentPRs("alice", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(stub.calls) != 2 {
		t.Fatalf("calls = %d", len(stub.calls))
	}
	if hasArgPrefix(stub.calls[0], "mergedCursor=") || hasArgPrefix(stub.calls[0], "openCursor=") {
		t.Fatalf("first call carries a cursor: %v", stub.calls[0])
	}
	if !containsArg(stub.calls[1], "mergedCursor=m1") {
		t.Fatalf("second call lacks cursor: %v", stub.calls[1])
	}
	var numbers []int
	for _, pr := range prs {
		numbers = append(numbers, pr.Number)
	}
	if len(numbers) != 3 || numbers[0] != 1 || numbers[1] != 2 || numbers[2] != 3 {
		t.Fatalf("numbers = %v", numbers)
	}
}

func TestRecentPRsBodyOnlyWhenAsked(t *testing.T) {
	for _, withBody := range []bool{false, true} {
		stub := &ghStub{results: []run.Result{{Stdout: pageOneJSON}}}
		if _, err := (Client{Run: stub}).RecentPRs("@me", time.Now(), withBody); err != nil {
			t.Fatal(err)
		}
		var query string
		for _, a := range stub.calls[0] {
			if strings.HasPrefix(a, "query=") {
				query = a
			}
		}
		if got := strings.Contains(query, "body"); got != withBody {
			t.Fatalf("withBody=%v but query contains body=%v", withBody, got)
		}
	}
}

func TestRecentPRsError(t *testing.T) {
	stub := &ghStub{errs: []error{errors.New("boom")}}
	prs, err := Client{Run: stub}.RecentPRs("alice", time.Now(), false)
	if err == nil || prs != nil {
		t.Fatalf("prs=%v err=%v", prs, err)
	}
}

func TestRecentPRsSinceVariable(t *testing.T) {
	stub := &ghStub{results: []run.Result{{Stdout: pageOneJSON}}}
	since := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)
	if _, err := (Client{Run: stub}).RecentPRs("@me", since, false); err != nil {
		t.Fatal(err)
	}
	want := "mergedQuery=is:pr is:merged author:@me merged:>=2026-10-01"
	if !containsArg(stub.calls[0], want) {
		t.Fatalf("args lack %q: %v", want, stub.calls[0])
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func hasArgPrefix(args []string, prefix string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}
