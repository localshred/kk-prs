package mirror

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/localshred/koolkit/pkg/run"
)

const fixtureRepo = "o/svc-a"

var fixtureBase = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// fixture is a bare "remote" repository plus the working repo that built it.
type fixture struct {
	dates  map[string]time.Time
	oids   map[string]string
	remote string
	root   string
	t      *testing.T
	work   string
}

// recordingRunner runs commands through run.Default and records each call.
type recordingRunner struct {
	calls [][]string
}

func (r *recordingRunner) Look(name string) (string, bool) {
	return run.Default.Look(name)
}

func (r *recordingRunner) Run(name string, args ...string) (run.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return run.Default.Run(name, args...)
}

func (r *recordingRunner) count(subcommand string) int {
	n := 0
	for _, call := range r.calls {
		for _, arg := range call {
			if arg == subcommand {
				n++
				break
			}
		}
	}
	return n
}

func (r *recordingRunner) ran(subcommand string) bool {
	for _, call := range r.calls {
		for _, arg := range call {
			if arg == subcommand {
				return true
			}
		}
	}
	return false
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern, branch string
		want            bool
	}{
		{"release*", "release-B", true},
		{"release*", "release/2.0", false},
		{"release*/**", "release/2.0", true},
		{"release*/**", "release/2.0/rc", true},
		{"release*/**", "release-B", false},
		{"hot-*", "release-B", false},
		{"main", "main", true},
	}
	for _, tt := range tests {
		if got := MatchPattern(tt.pattern, tt.branch); got != tt.want {
			t.Errorf("MatchPattern(%q, %q) = %v, want %v", tt.pattern, tt.branch, got, tt.want)
		}
	}
}

func TestOpenFetchesOnlyTrunksAndReleases(t *testing.T) {
	fx := newFixture(t)
	m, _ := newMirror(t, fx)

	repo := openRepo(t, m, false)

	want := []string{"origin/main", "origin/release-A", "origin/release-B"}
	if got := originRefs(t, repo.Dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("mirror refs = %v, want %v", got, want)
	}
	if want := filepath.Join(m.Root, "o", "svc-a.git"); repo.Dir != want {
		t.Errorf("Dir = %q, want %q", repo.Dir, want)
	}
}

func TestOpenForceRefetches(t *testing.T) {
	fx := newFixture(t)
	m, rec := newMirror(t, fx)
	openRepo(t, m, false)
	rec.calls = nil

	openRepo(t, m, true)

	if !rec.ran("ls-remote") || !rec.ran("fetch") {
		t.Fatalf("forced Open should hit the network, calls = %v", rec.calls)
	}
}

func TestOpenPrunesDeletedRelease(t *testing.T) {
	fx := newFixture(t)
	m, _ := newMirror(t, fx)
	openRepo(t, m, false)
	fx.git(fx.remote, fixtureBase, "update-ref", "-d", "refs/heads/release-A")

	later := fixtureBase.Add(6 * time.Minute)
	m.Now = func() time.Time { return later }
	repo := openRepo(t, m, false)

	want := []string{"origin/main", "origin/release-B"}
	if got := originRefs(t, repo.Dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("mirror refs = %v, want %v", got, want)
	}
}

func TestOpenRecreatesHalfCreatedMirror(t *testing.T) {
	fx := newFixture(t)
	m, _ := newMirror(t, fx)
	dir := filepath.Join(m.Root, "o", "svc-a.git")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	repo := openRepo(t, m, false)

	if _, err := os.Stat(filepath.Join(dir, "HEAD")); err != nil {
		t.Fatalf("mirror HEAD missing: %v", err)
	}
	want := []string{"origin/main", "origin/release-A", "origin/release-B"}
	if got := originRefs(t, repo.Dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("mirror refs = %v, want %v", got, want)
	}
}

func TestOpenRepointsCloneURL(t *testing.T) {
	fx := newFixture(t)
	m, _ := newMirror(t, fx)
	goodURL := m.CloneURL
	m.CloneURL = "file://" + filepath.Join(fx.root, "missing", "{repo}.git")
	if _, err := m.Open(fixtureRepo, "main", []string{"main"}, []string{"release*"}, false); err == nil {
		t.Fatal("Open with a bad clone URL should fail")
	}

	m.CloneURL = goodURL
	later := fixtureBase.Add(6 * time.Minute)
	m.Now = func() time.Time { return later }
	repo := openRepo(t, m, false)

	want := []string{"origin/main", "origin/release-A", "origin/release-B"}
	if got := originRefs(t, repo.Dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("mirror refs = %v, want %v", got, want)
	}
}

func TestOpenSkipsFreshFetch(t *testing.T) {
	fx := newFixture(t)
	m, rec := newMirror(t, fx)
	openRepo(t, m, false)
	rec.calls = nil

	later := fixtureBase.Add(time.Minute)
	m.Now = func() time.Time { return later }
	openRepo(t, m, false)

	if rec.ran("ls-remote") || rec.ran("fetch") {
		t.Fatalf("fresh Open should skip the network, calls = %v", rec.calls)
	}
}

func TestOpenSurvivesReleaseRenamedToDirectory(t *testing.T) {
	fx := newFixture(t)
	fx.git(fx.remote, fixtureBase, "update-ref", "refs/heads/release", fx.oids["#1"])
	m, _ := newMirror(t, fx)
	openRepo(t, m, false)
	fx.git(fx.remote, fixtureBase, "update-ref", "-d", "refs/heads/release")
	fx.git(fx.remote, fixtureBase, "update-ref", "refs/heads/release/1.0", fx.oids["#1"])

	later := fixtureBase.Add(6 * time.Minute)
	m.Now = func() time.Time { return later }
	repo := openRepo(t, m, false)

	want := []string{"origin/main", "origin/release-A", "origin/release-B", "origin/release/1.0"}
	if got := originRefs(t, repo.Dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("mirror refs = %v, want %v", got, want)
	}
}

// gitEnv is a hermetic environment for fixture git commands, with author and
// committer dates fixed to at.
func gitEnv(at time.Time) []string {
	date := fmt.Sprintf("%d +0000", at.Unix())
	return append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date,
	)
}

// newFixture builds the remote: main with (#1)..(#4); release-A cut after
// (#1) and carrying a cherry-pick of (#4) as (#6); release-B cut after (#3)
// with hotfix-B's fix (#5) merged --no-ff; and an unrelated feature-x.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	fx := &fixture{
		dates:  map[string]time.Time{},
		oids:   map[string]string{},
		remote: filepath.Join(root, "o", "svc-a.git"),
		root:   root,
		t:      t,
		work:   filepath.Join(root, "work"),
	}
	if err := os.MkdirAll(fx.work, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	fx.git(fx.work, fixtureBase, "init", "-q", "-b", "main")
	fx.commit(t, "#0", "initial commit", 0)
	fx.commit(t, "#1", "add one (#1)", 1)
	fx.git(fx.work, fixtureBase, "branch", "release-A")
	fx.commit(t, "#2", "add two (#2)", 2)
	fx.commit(t, "#3", "add three (#3)", 3)
	fx.git(fx.work, fixtureBase, "branch", "release-B")
	fx.commit(t, "#4", "add four (#4)", 4)

	fx.git(fx.work, fixtureBase, "checkout", "-q", "-b", "hotfix-B", "release-B")
	fx.commit(t, "#5", "fix (#5)", 5)
	fx.git(fx.work, fixtureBase, "checkout", "-q", "release-B")
	fx.dates["merge"] = fixtureBase.Add(6 * time.Hour)
	fx.git(fx.work, fx.dates["merge"], "merge", "-q", "--no-ff", "-m", "Merge branch 'hotfix-B' into release-B", "hotfix-B")
	fx.oids["merge"] = fx.git(fx.work, fixtureBase, "rev-parse", "HEAD")

	fx.git(fx.work, fixtureBase, "checkout", "-q", "release-A")
	fx.dates["pick"] = fixtureBase.Add(7 * time.Hour)
	fx.git(fx.work, fx.dates["pick"], "cherry-pick", "-x", fx.oids["#4"])
	message := fmt.Sprintf("add four (#4) (#6)\n\n(cherry picked from commit %s)\n", fx.oids["#4"])
	fx.git(fx.work, fx.dates["pick"], "commit", "-q", "--amend", "-m", message)
	fx.oids["pick"] = fx.git(fx.work, fixtureBase, "rev-parse", "HEAD")

	fx.git(fx.work, fixtureBase, "checkout", "-q", "-b", "feature-x", "main")
	fx.commit(t, "#x", "feature x", 8)

	fx.git(root, fixtureBase, "init", "-q", "--bare", fx.remote)
	fx.git(fx.remote, fixtureBase, "config", "uploadpack.allowFilter", "true")
	fx.git(fx.work, fixtureBase, "push", "-q", fx.remote, "main", "release-A", "release-B", "hotfix-B", "feature-x")
	return fx
}

// newMirror returns a Mirror over the fixture's remote with a fake clock at
// fixtureBase and a recording runner, isolating git from the user's config.
func newMirror(t *testing.T, fx *fixture) (Mirror, *recordingRunner) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	rec := &recordingRunner{}
	return Mirror{
		CloneURL: "file://" + fx.root + "/{repo}.git",
		MaxAge:   5 * time.Minute,
		Now:      func() time.Time { return fixtureBase },
		Root:     t.TempDir(),
		Run:      rec,
	}, rec
}

func openRepo(t *testing.T, m Mirror, force bool) *Repo {
	t.Helper()
	repo, err := m.Open(fixtureRepo, "main", []string{"main", "master"}, []string{"release*", "release*/**"}, force)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return repo
}

func originRefs(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin/")
	cmd.Env = gitEnv(fixtureBase)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("for-each-ref: %v\n%s", err, stderrOf(err))
	}
	return strings.Fields(string(out))
}

func stderrOf(err error) []byte {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.Stderr
	}
	return nil
}

// commit adds a file and commits it as label, hours after fixtureBase.
func (fx *fixture) commit(t *testing.T, label, subject string, hours int) {
	t.Helper()
	at := fixtureBase.Add(time.Duration(hours) * time.Hour)
	name := filepath.Join(fx.work, strings.TrimPrefix(label, "#")+".txt")
	if err := os.WriteFile(name, []byte(subject+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	fx.git(fx.work, at, "add", ".")
	fx.git(fx.work, at, "commit", "-q", "-m", subject)
	fx.dates[label] = at
	fx.oids[label] = fx.git(fx.work, at, "rev-parse", "HEAD")
}

// git runs a fixture git command in dir and returns its trimmed stdout.
func (fx *fixture) git(dir string, at time.Time, args ...string) string {
	fx.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv(at)
	out, err := cmd.Output()
	if err != nil {
		fx.t.Fatalf("git %v: %v\n%s", args, err, stderrOf(err))
	}
	return strings.TrimSpace(string(out))
}

// merge merges branch into the checked-out branch with --no-ff as label,
// hours after fixtureBase.
func (fx *fixture) merge(t *testing.T, label, branch string, hours int) {
	t.Helper()
	at := fixtureBase.Add(time.Duration(hours) * time.Hour)
	fx.git(fx.work, at, "merge", "-q", "--no-ff", "-m", "Merge branch '"+branch+"'", branch)
	fx.dates[label] = at
	fx.oids[label] = fx.git(fx.work, at, "rev-parse", "HEAD")
}
