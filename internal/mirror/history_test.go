package mirror

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/localshred/kk-prs/internal/board"
)

var _ board.History = (*Repo)(nil)

func TestContains(t *testing.T) {
	fx := newFixture(t)
	m, rec := newMirror(t, fx)
	repo := openRepo(t, m, false)

	if !repo.Contains(fx.oids["#2"], "release-B") {
		t.Error("release-B should contain (#2)")
	}
	if repo.Contains(fx.oids["#2"], "release-A") {
		t.Error("release-A should not contain (#2)")
	}
	if repo.Contains("0123456789abcdef0123456789abcdef01234567", "release-B") {
		t.Error("unknown oid should not be contained")
	}
	rec.calls = nil
	if repo.Contains("", "release-B") {
		t.Error("empty oid should not be contained")
	}
	if len(rec.calls) != 0 {
		t.Errorf("empty oid should not run git, calls = %v", rec.calls)
	}
}

func TestFindPickByNumberOnly(t *testing.T) {
	fx := newFixture(t)
	m, _ := newMirror(t, fx)
	repo := openRepo(t, m, false)

	pick, ok := repo.FindPick("", 4, "release-A")
	if !ok {
		t.Fatal("FindPick by number should find the pick")
	}
	if !pick.At.Equal(fx.dates["pick"]) {
		t.Errorf("At = %v, want %v", pick.At, fx.dates["pick"])
	}
}

func TestFindPickCherryPicked(t *testing.T) {
	fx := newFixture(t)
	m, _ := newMirror(t, fx)
	repo := openRepo(t, m, false)

	pick, ok := repo.FindPick(fx.oids["#4"], 4, "release-A")
	if !ok {
		t.Fatal("FindPick should find the cherry-pick")
	}
	if pick.Via != 6 {
		t.Errorf("Via = %d, want 6", pick.Via)
	}
	if !pick.At.Equal(fx.dates["pick"]) {
		t.Errorf("At = %v, want %v", pick.At, fx.dates["pick"])
	}
	if _, ok := repo.FindPick(fx.oids["#4"], 4, "release-B"); ok {
		t.Error("release-B has no pick of (#4)")
	}
}

func TestIntroducedAt(t *testing.T) {
	fx := newFixture(t)
	m, _ := newMirror(t, fx)
	repo := openRepo(t, m, false)

	at, ok := repo.IntroducedAt(fx.oids["#5"], "release-B")
	if !ok {
		t.Fatal("(#5) should be introduced to release-B by a merge")
	}
	if !at.Equal(fx.dates["merge"]) {
		t.Errorf("IntroducedAt = %v, want %v", at, fx.dates["merge"])
	}
	if at, ok := repo.IntroducedAt(fx.oids["#2"], "release-B"); ok {
		t.Errorf("(#2) came with the cut, got %v", at)
	}
}

func TestIntroducedAtDirectlyOnRelease(t *testing.T) {
	fx := newFixture(t)
	fx.git(fx.work, fixtureBase, "checkout", "-q", "-b", "pr-9", "release-B")
	fx.commit(t, "#9", "add nine (#9)", 9)
	fx.git(fx.work, fixtureBase, "checkout", "-q", "release-B")
	fx.merge(t, "merge-9", "pr-9", 10)
	fx.commit(t, "#10", "direct ten (#10)", 11)
	fx.git(fx.work, fixtureBase, "checkout", "-q", "-b", "pr-11", "release-B")
	fx.commit(t, "#11", "add eleven (#11)", 12)
	fx.git(fx.work, fixtureBase, "checkout", "-q", "release-B")
	fx.merge(t, "merge-11", "pr-11", 13)
	fx.git(fx.work, fixtureBase, "push", "-q", fx.remote, "release-B")
	m, _ := newMirror(t, fx)
	repo := openRepo(t, m, false)

	tests := []struct {
		label, want string
	}{
		{"#9", "merge-9"},
		{"merge-9", "merge-9"},
		{"#10", "#10"},
		{"#11", "merge-11"},
	}
	for _, tt := range tests {
		at, ok := repo.IntroducedAt(fx.oids[tt.label], "release-B")
		if !ok || !at.Equal(fx.dates[tt.want]) {
			t.Errorf("IntroducedAt(%s) = %v, %v; want %v", tt.label, at, ok, fx.dates[tt.want])
		}
	}
}

func TestIntroducedAtMultiPRHotfix(t *testing.T) {
	fx := newFixture(t)
	fx.git(fx.work, fixtureBase, "checkout", "-q", "-b", "hotfix-A", "release-A")
	fx.git(fx.work, fixtureBase, "checkout", "-q", "-b", "pr-7", "hotfix-A")
	fx.commit(t, "#7", "add seven (#7)", 9)
	fx.git(fx.work, fixtureBase, "checkout", "-q", "hotfix-A")
	fx.merge(t, "merge-7", "pr-7", 10)
	fx.git(fx.work, fixtureBase, "checkout", "-q", "-b", "pr-8", "hotfix-A")
	fx.commit(t, "#8", "add eight (#8)", 11)
	fx.git(fx.work, fixtureBase, "checkout", "-q", "hotfix-A")
	fx.merge(t, "merge-8", "pr-8", 12)
	fx.git(fx.work, fixtureBase, "checkout", "-q", "release-A")
	fx.merge(t, "merge-hotfix", "hotfix-A", 13)
	fx.git(fx.work, fixtureBase, "push", "-q", fx.remote, "release-A")
	m, _ := newMirror(t, fx)
	repo := openRepo(t, m, false)

	for _, label := range []string{"#7", "merge-7", "#8", "merge-8"} {
		at, ok := repo.IntroducedAt(fx.oids[label], "release-A")
		if !ok || !at.Equal(fx.dates["merge-hotfix"]) {
			t.Errorf("IntroducedAt(%s) = %v, %v; want %v", label, at, ok, fx.dates["merge-hotfix"])
		}
	}
}

func TestReleasesCapsAtEightAcrossPatterns(t *testing.T) {
	fx := newFixture(t)
	for i := 1; i <= 9; i++ {
		fx.git(fx.remote, fixtureBase, "update-ref", fmt.Sprintf("refs/heads/release-%d", i), fx.oids["#1"])
	}
	fx.git(fx.remote, fixtureBase, "update-ref", "refs/heads/release/2.0", fx.oids["#1"])
	m, _ := newMirror(t, fx)
	repo := openRepo(t, m, false)

	got := []string{}
	for _, release := range repo.Releases() {
		got = append(got, release.Name)
	}
	want := []string{
		"release/2.0", "release-B", "release-A", "release-9",
		"release-8", "release-7", "release-6", "release-5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Releases() names = %v, want %v", got, want)
	}
}

func TestReleasesNewestFirstWithCut(t *testing.T) {
	fx := newFixture(t)
	m, _ := newMirror(t, fx)
	repo := openRepo(t, m, false)

	got := repo.Releases()
	want := []board.Release{
		{CutAt: fx.dates["#3"], Name: "release-B"},
		{CutAt: fx.dates["#1"], Name: "release-A"},
	}
	if len(got) != len(want) {
		t.Fatalf("Releases() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i].Name || !got[i].CutAt.Equal(want[i].CutAt) {
			t.Errorf("Releases()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
