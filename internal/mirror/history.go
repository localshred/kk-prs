package mirror

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/localshred/kk-prs/internal/board"
)

const releasesPerPattern = 8

var trailingPRNumber = regexp.MustCompile(`\(#(\d+)\)\s*$`)

// Contains reports whether the release branch contains oid. An empty or
// unknown oid is never contained.
func (r *Repo) Contains(oid, release string) bool {
	if oid == "" {
		return false
	}
	return r.isAncestor(oid, originPrefix+release)
}

// FindPick finds the newest commit on release, but not on the default branch,
// that cherry-picks oid, falling back to one whose message mentions
// "(#number)". Via is the PR number at the end of that commit's subject when
// it differs from number.
func (r *Repo) FindPick(oid string, number int, release string) (board.Pick, bool) {
	needles := []string{}
	if oid != "" {
		needles = append(needles, "cherry picked from commit "+oid)
	}
	needles = append(needles, "(#"+strconv.Itoa(number)+")")
	for _, needle := range needles {
		if pick, ok := r.searchPick(needle, number, release); ok {
			return pick, true
		}
	}
	return board.Pick{}, false
}

// IntroducedAt reports when oid reached release through a merge after the
// release was cut. It is false when oid came in with the cut or no merge on
// release brought it in.
func (r *Repo) IntroducedAt(oid, release string) (time.Time, bool) {
	if oid == "" {
		return time.Time{}, false
	}
	if forkPoint, ok := r.forkPoint(release); ok && r.isAncestor(oid, forkPoint) {
		return time.Time{}, false
	}
	out, err := r.git("log", "--ancestry-path", "--merges", "--reverse", "--format=%cI", oid+".."+originPrefix+release)
	if err != nil {
		return time.Time{}, false
	}
	first, _, _ := strings.Cut(out, "\n")
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(first))
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}

// Releases lists the mirrored release branches matching Patterns, up to
// eight per pattern, newest first. CutAt is the committer date of the
// branch's fork point from the default branch, zero when that is unknown.
func (r *Repo) Releases() []board.Release {
	seen := map[string]bool{}
	releases := []board.Release{}
	for _, pattern := range r.Patterns {
		out, err := r.git("for-each-ref", "--sort=-version:refname", "--count="+strconv.Itoa(releasesPerPattern), "--format=%(refname:short)", originPrefix+pattern)
		if err != nil {
			continue
		}
		for _, short := range strings.Fields(out) {
			name := strings.TrimPrefix(short, "origin/")
			if seen[name] {
				continue
			}
			seen[name] = true
			releases = append(releases, board.Release{CutAt: r.cutAt(name), Name: name})
		}
	}
	sort.SliceStable(releases, func(i, j int) bool {
		return board.VersionLess(releases[j].Name, releases[i].Name)
	})
	return releases
}

func (r *Repo) cutAt(release string) time.Time {
	forkPoint, ok := r.forkPoint(release)
	if !ok {
		return time.Time{}
	}
	out, err := r.git("log", "-1", "--format=%cI", forkPoint)
	if err != nil {
		return time.Time{}
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(out))
	if err != nil {
		return time.Time{}
	}
	return at
}

func (r *Repo) forkPoint(release string) (string, bool) {
	out, err := r.git("merge-base", originPrefix+release, originPrefix+r.DefaultBranch)
	oid := strings.TrimSpace(out)
	return oid, err == nil && oid != ""
}

func (r *Repo) isAncestor(oid, ref string) bool {
	res, err := r.Run.Run("git", "--no-lazy-fetch", "-C", r.Dir, "merge-base", "--is-ancestor", oid, ref)
	return err == nil && res.Code == 0
}

func (r *Repo) searchPick(needle string, number int, release string) (board.Pick, bool) {
	out, err := r.git("log", "--format=%H%x1f%cI%x1f%s", "-F", "--grep="+needle, originPrefix+release, "--not", originPrefix+r.DefaultBranch)
	if err != nil {
		return board.Pick{}, false
	}
	first, _, _ := strings.Cut(out, "\n")
	fields := strings.Split(first, "\x1f")
	if len(fields) != 3 {
		return board.Pick{}, false
	}
	at, err := time.Parse(time.RFC3339, fields[1])
	if err != nil {
		return board.Pick{}, false
	}
	pick := board.Pick{At: at}
	if m := trailingPRNumber.FindStringSubmatch(fields[2]); m != nil {
		if via, err := strconv.Atoi(m[1]); err == nil && via != number {
			pick.Via = via
		}
	}
	return pick, true
}
