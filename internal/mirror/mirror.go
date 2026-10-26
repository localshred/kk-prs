// Package mirror keeps kk-owned, bare, commit-only mirrors of GitHub repos
// holding just their trunk and release branches, and answers release history
// questions from them.
package mirror

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/localshred/koolkit/pkg/run"
)

// Mirror opens per-repo mirrors under Root.
type Mirror struct {
	// CloneURL is the remote URL template; "{repo}" becomes owner/name.
	CloneURL string
	// MaxAge is how long a fetch stays fresh before Open fetches again.
	MaxAge time.Duration
	Now    func() time.Time
	// Root is the directory holding <owner>/<name>.git mirrors.
	Root string
	Run  run.Runner
}

// Repo is one opened mirror. It implements board.History.
type Repo struct {
	DefaultBranch string
	Dir           string
	// Patterns are the release branch globs, as for MatchPattern.
	Patterns []string
	Run      run.Runner
}

const (
	fetchedAtFile = "kk-fetched-at"
	headsPrefix   = "refs/heads/"
	originPrefix  = "refs/remotes/origin/"
)

// MatchPattern reports whether branch matches pattern with for-each-ref glob
// semantics: "*" does not cross "/", and a trailing "/**" matches any branch
// nested below a prefix matching the rest of the pattern.
func MatchPattern(pattern, branch string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "/**"); ok {
		depth := strings.Count(prefix, "/") + 1
		segments := strings.Split(branch, "/")
		if len(segments) <= depth {
			return false
		}
		matched, _ := path.Match(prefix, strings.Join(segments[:depth], "/"))
		return matched
	}
	matched, _ := path.Match(pattern, branch)
	return matched
}

// Open returns the mirror of repo (owner/name), creating it if missing. Unless
// the last fetch is younger than MaxAge and force is false, it fetches the
// remote heads named in trunks or matching any of patterns, first dropping
// every other mirrored branch so renamed branches cannot block the fetch.
func (m Mirror) Open(repo, defaultBranch string, trunks, patterns []string, force bool) (*Repo, error) {
	r := &Repo{
		DefaultBranch: defaultBranch,
		Dir:           filepath.Join(m.Root, filepath.FromSlash(repo)+".git"),
		Patterns:      patterns,
		Run:           m.Run,
	}
	if _, err := os.Stat(r.Dir); os.IsNotExist(err) {
		if err := m.create(r, repo); err != nil {
			return nil, err
		}
	}
	if !force && m.fresh(r.Dir) {
		return r, nil
	}
	if err := m.fetch(r, trunks, patterns); err != nil {
		return nil, fmt.Errorf("mirror %s: %w", repo, err)
	}
	return r, nil
}

func (m Mirror) create(r *Repo, repo string) error {
	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return fmt.Errorf("mirror %s: %w", repo, err)
	}
	steps := [][]string{
		{"init", "--bare", "-q"},
		{"config", "remote.origin.url", strings.ReplaceAll(m.CloneURL, "{repo}", repo)},
		{"config", "remote.origin.promisor", "true"},
		{"config", "remote.origin.partialclonefilter", "tree:0"},
	}
	for _, args := range steps {
		if _, err := r.git(args...); err != nil {
			_ = os.RemoveAll(r.Dir)
			return fmt.Errorf("mirror %s: %w", repo, err)
		}
	}
	return nil
}

func (m Mirror) fetch(r *Repo, trunks, patterns []string) error {
	out, err := r.git("ls-remote", "--heads", "origin")
	if err != nil {
		return err
	}
	wanted := map[string]bool{}
	refspecs := []string{}
	for _, line := range strings.Split(out, "\n") {
		_, ref, ok := strings.Cut(line, "\t")
		branch, isHead := strings.CutPrefix(ref, headsPrefix)
		if !ok || !isHead || !wantBranch(branch, trunks, patterns) {
			continue
		}
		wanted[originPrefix+branch] = true
		refspecs = append(refspecs, "+"+ref+":"+originPrefix+branch)
	}
	if err := r.prune(wanted); err != nil {
		return err
	}
	if len(refspecs) > 0 {
		args := append([]string{"fetch", "--filter=tree:0", "--no-tags", "--quiet", "origin"}, refspecs...)
		if _, err := r.git(args...); err != nil {
			return err
		}
	}
	stamp := m.Now().Format(time.RFC3339) + "\n"
	return os.WriteFile(filepath.Join(r.Dir, fetchedAtFile), []byte(stamp), 0o644)
}

func (m Mirror) fresh(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, fetchedAtFile))
	if err != nil {
		return false
	}
	fetchedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	return err == nil && fetchedAt.After(m.Now().Add(-m.MaxAge))
}

// git runs a git command against the mirror without lazy-fetching missing
// objects, returning stdout or, on any non-zero exit, an error carrying
// git's stderr.
func (r *Repo) git(args ...string) (string, error) {
	res, err := r.Run.Run("git", append([]string{"--no-lazy-fetch", "-C", r.Dir}, args...)...)
	if err == nil && res.Code != 0 {
		err = fmt.Errorf("exit status %d", res.Code)
	}
	if err != nil {
		return res.Stdout, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(res.Stderr))
	}
	return res.Stdout, nil
}

func (r *Repo) prune(wanted map[string]bool) error {
	out, err := r.git("for-each-ref", "--format=%(refname)", originPrefix)
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(out) {
		if wanted[ref] {
			continue
		}
		if _, err := r.git("update-ref", "-d", ref); err != nil {
			return err
		}
	}
	return nil
}

func wantBranch(branch string, trunks, patterns []string) bool {
	for _, trunk := range trunks {
		if branch == trunk {
			return true
		}
	}
	for _, pattern := range patterns {
		if MatchPattern(pattern, branch) {
			return true
		}
	}
	return false
}
