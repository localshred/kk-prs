// kk-prs/internal/watch/snapshot_test.go
package watch

import "testing"

func TestParseRefFormats(t *testing.T) {
	r, ok := ParseRef("owner/repo#42")
	if !ok || r.Owner != "owner" || r.Repo != "repo" || r.Number != 42 {
		t.Fatalf("short ref = %+v ok=%v", r, ok)
	}
	if r.Key() != "owner/repo#42" {
		t.Fatalf("key = %q", r.Key())
	}
	u, ok := ParseRef("https://github.com/o/r/pull/7")
	if !ok || u.Number != 7 || u.URL != "https://github.com/o/r/pull/7" {
		t.Fatalf("url ref = %+v ok=%v", u, ok)
	}
	if _, ok := ParseRef("garbage"); ok {
		t.Fatal("garbage should not parse")
	}
	if _, ok := ParseRef("owner/repo#99999999999999999999"); ok {
		t.Fatal("overflowing number should not parse")
	}
}

func TestChecksSummary(t *testing.T) {
	if got := ChecksSummary(nil); got != "none" {
		t.Errorf("empty = %q", got)
	}
	fail := []RollupEntry{{State: "FAILURE"}}
	if got := ChecksSummary(fail); got != "failing" {
		t.Errorf("failure = %q", got)
	}
	pending := []RollupEntry{{Status: "IN_PROGRESS"}}
	if got := ChecksSummary(pending); got != "running" {
		t.Errorf("pending = %q", got)
	}
	pass := []RollupEntry{{State: "SUCCESS"}, {Status: "COMPLETED", Conclusion: "SUCCESS"}}
	if got := ChecksSummary(pass); got != "passing" {
		t.Errorf("pass = %q", got)
	}
}

func TestComputeChanges(t *testing.T) {
	old := Snapshot{State: "OPEN", Comments: 1, Checks: "running", ReviewDecision: ""}
	nw := Snapshot{State: "OPEN", Comments: 3, Checks: "passing", ReviewDecision: "APPROVED"}
	got := ComputeChanges(old, nw)
	joined := ""
	for _, g := range got {
		joined += g + "|"
	}
	for _, want := range []string{"2 new comment", "Approved", "CI passing"} {
		if !contains(joined, want) {
			t.Errorf("changes %q missing %q", joined, want)
		}
	}
}

func TestComputeChangesMerged(t *testing.T) {
	got := ComputeChanges(Snapshot{State: "OPEN"}, Snapshot{State: "MERGED"})
	if len(got) != 1 || !contains(got[0], "Merged") {
		t.Fatalf("merged changes = %v", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return len(sub) == 0
}
