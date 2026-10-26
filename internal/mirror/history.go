package mirror

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/localshred/kk-prs/internal/board"
)

const maxReleases = 8

var trailingPRNumber = regexp.MustCompile(`\(#(\d+)\)\s*$`)

// Contains reports whether the release branch contains oid. An empty or
// unknown oid is never contained.
func (r *Repo) Contains(oid, release string) bool {
	if oid == "" {
		return false
	}
	return r.isAncestor(oid, originPrefix+release)
}

// FindPick finds the newest commit on release, but not on the default branch
// when there is one, that cherry-picks oid, falling back to the newest whose
// message mentions "(#number)". It runs one git log per call. Via is the PR
// number at the end of that commit's subject when it differs from number.
func (r *Repo) FindPick(oid string, number int, release string) (board.Pick, bool) {
	cherry := ""
	args := []string{"log", "--format=%cI%x1f%s%x1f%B%x1e", "-F"}
	if oid != "" {
		cherry = "cherry picked from commit " + oid
		args = append(args, "--grep="+cherry)
	}
	args = append(args, "--grep=(#"+strconv.Itoa(number)+")", originPrefix+release)
	if r.DefaultBranch != "" {
		args = append(args, "--not", originPrefix+r.DefaultBranch)
	}
	out, err := r.git(args...)
	if err != nil {
		return board.Pick{}, false
	}
	var fallback []string
	for _, record := range strings.Split(out, "\x1e") {
		fields := strings.SplitN(strings.TrimLeft(record, "\n"), "\x1f", 3)
		if len(fields) != 3 {
			continue
		}
		if cherry != "" && strings.Contains(fields[2], cherry) {
			return parsePick(fields, number)
		}
		if fallback == nil {
			fallback = fields
		}
	}
	if fallback == nil {
		return board.Pick{}, false
	}
	return parsePick(fallback, number)
}

// IntroducedAt reports when oid reached release after the release was cut:
// the committer date of the oldest first-parent commit on release, past the
// fork point from the default branch, that has oid as an ancestor or is oid.
// It is false when oid came in with the cut, the fork point is unknown, or
// no such commit exists.
func (r *Repo) IntroducedAt(oid, release string) (time.Time, bool) {
	if oid == "" {
		return time.Time{}, false
	}
	forkPoint, ok := r.forkPoint(release)
	if !ok || r.isAncestor(oid, forkPoint) {
		return time.Time{}, false
	}
	out, err := r.git("rev-list", "--first-parent", "--reverse", forkPoint+".."+originPrefix+release)
	if err != nil {
		return time.Time{}, false
	}
	commits := strings.Fields(out)
	first := sort.Search(len(commits), func(i int) bool {
		return r.isAncestor(oid, commits[i])
	})
	if first == len(commits) {
		return time.Time{}, false
	}
	return r.commitDate(commits[first])
}

// Releases lists the newest mirrored release branches matching Patterns,
// at most eight across all patterns, newest first by board.VersionLess.
// CutAt is the committer date of the branch's fork point from the default
// branch, zero when that is unknown. The list is computed once per Repo.
func (r *Repo) Releases() []board.Release {
	r.releasesOnce.Do(func() {
		r.releases = r.listReleases()
	})
	return r.releases
}

func (r *Repo) commitDate(oid string) (time.Time, bool) {
	out, err := r.git("log", "-1", "--format=%cI", oid)
	if err != nil {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(out))
	return at, err == nil
}

func (r *Repo) cutAt(release string) time.Time {
	forkPoint, ok := r.forkPoint(release)
	if !ok {
		return time.Time{}
	}
	at, _ := r.commitDate(forkPoint)
	return at
}

func (r *Repo) forkPoint(release string) (string, bool) {
	if r.DefaultBranch == "" {
		return "", false
	}
	out, err := r.git("merge-base", originPrefix+release, originPrefix+r.DefaultBranch)
	oid := strings.TrimSpace(out)
	return oid, err == nil && oid != ""
}

func (r *Repo) isAncestor(oid, ref string) bool {
	_, err := r.git("merge-base", "--is-ancestor", oid, ref)
	return err == nil
}

func (r *Repo) listReleases() []board.Release {
	seen := map[string]bool{}
	names := []string{}
	for _, pattern := range r.Patterns {
		out, err := r.git("for-each-ref", "--format=%(refname:short)", originPrefix+pattern)
		if err != nil {
			continue
		}
		for _, short := range strings.Fields(out) {
			name := strings.TrimPrefix(short, "origin/")
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	sort.SliceStable(names, func(i, j int) bool {
		return board.VersionLess(names[j], names[i])
	})
	if len(names) > maxReleases {
		names = names[:maxReleases]
	}
	releases := []board.Release{}
	for _, name := range names {
		releases = append(releases, board.Release{CutAt: r.cutAt(name), Name: name})
	}
	return releases
}

func parsePick(fields []string, number int) (board.Pick, bool) {
	at, err := time.Parse(time.RFC3339, fields[0])
	if err != nil {
		return board.Pick{}, false
	}
	pick := board.Pick{At: at}
	if m := trailingPRNumber.FindStringSubmatch(fields[1]); m != nil {
		if via, err := strconv.Atoi(m[1]); err == nil && via != number {
			pick.Via = via
		}
	}
	return pick, true
}
