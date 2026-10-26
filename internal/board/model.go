// Package board is the model and placement logic for the released board: PRs
// grouped into swim lanes by the release branch each one reached.
package board

import (
	"strings"
	"time"

	"github.com/localshred/kk-prs/internal/gh"
)

// Board is the full set of lanes plus any warnings gathered while building it.
type Board struct {
	Lanes    []Lane
	Warnings []string
}

// Card is one PR placed in a lane.
type Card struct {
	// CheckGlyph is "✓", "✗", "↻", or "" and is only set on open cards.
	CheckGlyph string
	// InReleaseAt is when the PR entered the lane's release; zero means not
	// shown.
	InReleaseAt time.Time
	PR          gh.BoardPR
	// Reason is the lane-2 reason text.
	Reason string
	// Via is "#16450", "cherry-pick", or "".
	Via         string
	WorkItemID  string
	WorkItemURL string
}

// History answers questions about which commits reached which release
// branches.
type History interface {
	// Contains reports whether the release branch contains the commit.
	Contains(oid, release string) bool
	// FindPick finds a cherry-pick of the commit (or PR number) on the release.
	FindPick(oid string, number int, release string) (Pick, bool)
	// IntroducedAt reports when the commit first reached the release.
	IntroducedAt(oid, release string) (time.Time, bool)
	// Releases lists release branches, newest first.
	Releases() []Release
}

// Lane is one swim lane of the board.
type Lane struct {
	Cards []Card
	CutAt time.Time
	Kind  LaneKind
	Name  string
}

// LaneKind distinguishes the open, pending, and release lanes.
type LaneKind int

// Lane kinds, in board order.
const (
	LaneOpen LaneKind = iota
	LanePending
	LaneRelease
)

// Reason strings shown on pending-lane cards.
const (
	ReasonNoReleases = "no releases"
	ReasonUnknown    = "release status unknown"
)

// Pick is a cherry-pick of a PR onto a release branch.
type Pick struct {
	At time.Time
	// Via is the PR number of the pick; 0 means unknown.
	Via int
}

// Release is a release branch and when it was cut.
type Release struct {
	CutAt time.Time
	Name  string
}

// ReasonOnTrunk is the reason for a PR merged to base but not yet released.
func ReasonOnTrunk(base string) string {
	return "on " + base
}

// ReasonWaiting is the reason for a PR merged to a non-default base.
func ReasonWaiting(base string) string {
	return "→ " + base + " · waiting"
}

// VersionLess reports whether a sorts before b in natural order: digit runs
// compare numerically, other runs lexically.
func VersionLess(a, b string) bool {
	ra, rb := splitRuns(a), splitRuns(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		if c := compareRuns(ra[i], rb[i]); c != 0 {
			return c < 0
		}
	}
	return len(ra) < len(rb)
}

func compareRuns(a, b string) int {
	if isDigit(a[0]) && isDigit(b[0]) {
		a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
		if len(a) != len(b) {
			return len(a) - len(b)
		}
	}
	return strings.Compare(a, b)
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func splitRuns(s string) []string {
	var runs []string
	start := 0
	for i := 1; i <= len(s); i++ {
		if i == len(s) || isDigit(s[i]) != isDigit(s[i-1]) {
			runs = append(runs, s[start:i])
			start = i
		}
	}
	return runs
}
