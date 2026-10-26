package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/localshred/koolkit/pkg/run"
	"github.com/localshred/koolkit/pkg/ui"
)

type releasedFixture struct {
	mainOID string
	oneOID  string
	root    string
}

type routeRunner struct {
	graphql string
}

func (r routeRunner) Look(name string) (string, bool) {
	return run.Default.Look(name)
}

func (r routeRunner) Run(name string, args ...string) (run.Result, error) {
	if name != "gh" {
		return run.Default.Run(name, args...)
	}
	if len(args) >= 2 && args[0] == "api" && args[1] == "graphql" {
		return run.Result{Stdout: r.graphql}, nil
	}
	return run.Result{Stdout: "[]"}, nil
}

func TestListMatchesRoot(t *testing.T) {
	fake := &run.Fake{Results: map[string]run.Result{
		"gh api user --jq .login": {Stdout: "bj\n"},
		"gh search prs --author=@me --state=open --json number,title,author,repository,url,createdAt --limit 50": {
			Stdout: `[{"number":7,"title":"Fix bug","url":"u","createdAt":"2026-01-01T00:00:00Z","repository":{"nameWithOwner":"o/r"}}]`,
		},
		"gh pr checks 7 --repo o/r --json name,state,link,workflow": {Stdout: "[]"},
	}}
	outputs := map[string]string{}
	for name, args := range map[string][]string{"root": {}, "list": {"list"}} {
		var out bytes.Buffer
		root := newRootCmd(fake)
		root.SetOut(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		outputs[name] = out.String()
	}
	if outputs["root"] == "" || outputs["root"] != outputs["list"] {
		t.Fatalf("root = %q, list = %q", outputs["root"], outputs["list"])
	}
}

func TestReleasedDedupesAuthors(t *testing.T) {
	fx := newReleasedFixture(t, nil)
	graphql := graphqlJSON(
		openNode(10, "Open work", "alice", "o/svc-a"),
		mergedNode(1, "Merged one", "alice", "o/svc-a", "main", fx.oneOID),
	)
	out := execReleased(t, routeRunner{graphql: graphql}, "--author", "alice", "--author", "alice")
	if n := strings.Count(out, "svc-a#10"); n != 1 {
		t.Errorf("open card appears %d times:\n%s", n, out)
	}
	if n := strings.Count(out, "svc-a#1 "); n != 1 {
		t.Errorf("merged card appears %d times:\n%s", n, out)
	}
	if !strings.Contains(out, "@alice") {
		t.Errorf("missing @alice:\n%s", out)
	}
	if !strings.Contains(out, "🚢 Releases for alice, alice (last 4 weeks)") {
		t.Errorf("missing header:\n%s", out)
	}
	if !strings.Contains(out, "release-A (1)") {
		t.Errorf("merged PR not placed in release-A:\n%s", out)
	}
}

func TestReleasedIgnoresRepo(t *testing.T) {
	fx := newReleasedFixture(t, []string{"kk.prs.ignoreRepo o/sdk"})
	graphql := graphqlJSON(
		openNode(10, "Open work", "alice", "o/svc-a"),
		mergedNode(1, "Merged one", "alice", "o/svc-a", "main", fx.oneOID),
		openNode(20, "Sdk work", "alice", "o/sdk"),
		mergedNode(21, "Sdk merged", "alice", "o/sdk", "main", fx.oneOID),
	)
	out := execReleased(t, routeRunner{graphql: graphql}, "--author", "alice")
	if strings.Contains(out, "sdk#") {
		t.Errorf("ignored repo shown:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("XDG_CACHE_HOME"), "koolkit", "prs-mirrors", "o", "sdk.git")); !os.IsNotExist(err) {
		t.Errorf("mirror opened for ignored repo: %v", err)
	}
}

func TestReleasedMirrorFailureWarns(t *testing.T) {
	fx := newReleasedFixture(t, nil)
	missing := filepath.Join(fx.root, "missing", "{repo}.git")
	writeGitConfig(t, fx.root, []string{"kk.prs.cloneURL file://" + missing})
	graphql := graphqlJSON(
		mergedNode(1, "Merged one", "alice", "o/svc-a", "main", fx.oneOID),
	)
	out := execReleased(t, routeRunner{graphql: graphql}, "--author", "alice")
	if !strings.Contains(out, "⚠ o/svc-a:") {
		t.Errorf("missing warning:\n%s", out)
	}
	if !strings.Contains(out, "release status unknown") {
		t.Errorf("missing unknown reason:\n%s", out)
	}
}

func graphqlJSON(nodes ...string) string {
	var open, merged []string
	for _, n := range nodes {
		if strings.Contains(n, `"state":"OPEN"`) {
			open = append(open, n)
		} else {
			merged = append(merged, n)
		}
	}
	return fmt.Sprintf(`{"data":{"open":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[%s]},"merged":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[%s]}}}`,
		strings.Join(open, ","), strings.Join(merged, ","))
}

func mergedNode(number int, title, author, repo, base, oid string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf(`{"number":%d,"title":%q,"url":"https://example.com/%s/pull/%d","createdAt":%q,"mergedAt":%q,"state":"MERGED","baseRefName":%q,"mergeCommit":{"oid":%q},"author":{"login":%q},"repository":{"nameWithOwner":%q,"defaultBranchRef":{"name":"main"}}}`,
		number, title, repo, number, now, now, base, oid, author, repo)
}

func newReleasedFixture(t *testing.T, settings []string) *releasedFixture {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	remote := filepath.Join(root, "o", "svc-a.git")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	fx := &releasedFixture{root: root}
	at := time.Now().Add(-48 * time.Hour)
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		date := fmt.Sprintf("%d +0000", at.Unix())
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
			"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	commit := func(name, subject string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(work, name), []byte(subject+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git(work, "add", ".")
		git(work, "commit", "-q", "-m", subject)
		at = at.Add(time.Hour)
		return git(work, "rev-parse", "HEAD")
	}
	git(work, "init", "-q", "-b", "main")
	commit("0.txt", "initial commit")
	fx.oneOID = commit("1.txt", "add one (#1)")
	git(work, "branch", "release-A")
	fx.mainOID = commit("2.txt", "add two (#2)")
	git(root, "init", "-q", "--bare", remote)
	git(remote, "config", "uploadpack.allowFilter", "true")
	git(work, "push", "-q", remote, "main", "release-A")

	writeGitConfig(t, root, append([]string{"kk.prs.cloneURL file://" + root + "/{repo}.git"}, settings...))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("COLUMNS", "200")
	return fx
}

func openNode(number int, title, author, repo string) string {
	return fmt.Sprintf(`{"number":%d,"title":%q,"url":"https://example.com/%s/pull/%d","createdAt":%q,"mergedAt":null,"state":"OPEN","baseRefName":"main","mergeCommit":null,"author":{"login":%q},"repository":{"nameWithOwner":%q,"defaultBranchRef":{"name":"main"}}}`,
		number, title, repo, number, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), author, repo)
}

func execReleased(t *testing.T, r run.Runner, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd := newReleasedCmd(r)
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("released: %v", err)
	}
	return ui.StripANSI(out.String())
}

func writeGitConfig(t *testing.T, root string, settings []string) {
	t.Helper()
	var sb strings.Builder
	for _, setting := range settings {
		key, value, _ := strings.Cut(setting, " ")
		parts := strings.Split(key, ".")
		section := parts[0]
		if len(parts) > 2 {
			section = fmt.Sprintf("%s %q", parts[0], strings.Join(parts[1:len(parts)-1], "."))
		}
		fmt.Fprintf(&sb, "[%s]\n\t%s = %s\n", section, parts[len(parts)-1], value)
	}
	path := filepath.Join(root, "gitconfig")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", path)
}
