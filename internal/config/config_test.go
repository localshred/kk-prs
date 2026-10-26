package config

import (
	"errors"
	"reflect"
	"testing"

	"github.com/localshred/koolkit/pkg/run"
)

func fake(values map[string]string) *run.Fake {
	f := &run.Fake{Results: map[string]run.Result{}, Errs: map[string]error{}}
	keys := []string{
		"kk.trunks", "kk.releasePattern", "kk.prs.weeks", "kk.prs.ignoreRepo",
		"kk.prs.cloneURL", "kk.prs.workItemPattern", "kk.prs.workItemURL",
	}
	for _, k := range keys {
		cmd := "git config --get-all " + k
		if v, ok := values[k]; ok {
			f.Results[cmd] = run.Result{Stdout: v}
		} else {
			f.Errs[cmd] = errors.New("exit status 1")
		}
	}
	return f
}

func TestLoadDefaults(t *testing.T) {
	s := Load(fake(nil))
	if s.Weeks != 4 {
		t.Fatalf("Weeks = %d", s.Weeks)
	}
	if s.CloneURL != "git@github.com:{repo}.git" {
		t.Fatalf("CloneURL = %q", s.CloneURL)
	}
	if !reflect.DeepEqual(s.ReleasePatterns, DefaultReleasePatterns) {
		t.Fatalf("ReleasePatterns = %v", s.ReleasePatterns)
	}
	if s.WorkItemPattern != nil {
		t.Fatalf("WorkItemPattern = %v", s.WorkItemPattern)
	}
}

func TestLoadInvalidWorkItemPattern(t *testing.T) {
	s := Load(fake(map[string]string{"kk.prs.workItemPattern": "WI-(\n"}))
	if s.WorkItemPattern != nil {
		t.Fatalf("WorkItemPattern = %v", s.WorkItemPattern)
	}
}

func TestLoadMultiValued(t *testing.T) {
	s := Load(fake(map[string]string{
		"kk.releasePattern": "rel-*\nhot-* x-*\n",
		"kk.prs.ignoreRepo": "o/sdk\n",
		"kk.trunks":         "dev\n",
	}))
	want := []string{"rel-*", "hot-*", "x-*"}
	if !reflect.DeepEqual(s.ReleasePatterns, want) {
		t.Fatalf("ReleasePatterns = %v", s.ReleasePatterns)
	}
	if !s.IgnoreRepos["o/sdk"] {
		t.Fatalf("IgnoreRepos = %v", s.IgnoreRepos)
	}
	if !reflect.DeepEqual(s.ExtraTrunks, []string{"dev"}) {
		t.Fatalf("ExtraTrunks = %v", s.ExtraTrunks)
	}
}

func TestLoadBadWeeks(t *testing.T) {
	if s := Load(fake(map[string]string{"kk.prs.weeks": "zero\n"})); s.Weeks != 4 {
		t.Fatalf("Weeks = %d", s.Weeks)
	}
	if s := Load(fake(map[string]string{"kk.prs.weeks": "2\n"})); s.Weeks != 2 {
		t.Fatalf("Weeks = %d", s.Weeks)
	}
}

func TestTrunksAdditive(t *testing.T) {
	s := Settings{ExtraTrunks: []string{"dev", "main"}}
	if got, want := s.Trunks("trunk"), []string{"trunk", "main", "master", "dev"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Trunks = %v", got)
	}
	if got, want := s.Trunks(""), []string{"main", "master", "dev"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Trunks = %v", got)
	}
}

func TestWorkItemLink(t *testing.T) {
	s := Load(fake(map[string]string{
		"kk.prs.workItemPattern": `WI-\d+` + "\n",
		"kk.prs.workItemURL":     "https://t.example/{id}\n",
	}))
	if id, url := s.WorkItemLink("WI-1234 fix", ""); id != "WI-1234" || url != "https://t.example/WI-1234" {
		t.Fatalf("got %q %q", id, url)
	}
	if id, url := s.WorkItemLink("fix", "see WI-9"); id != "WI-9" || url != "https://t.example/WI-9" {
		t.Fatalf("got %q %q", id, url)
	}
	if id, url := s.WorkItemLink("fix", "nothing"); id != "" || url != "" {
		t.Fatalf("got %q %q", id, url)
	}
	s.WorkItemURL = ""
	if id, url := s.WorkItemLink("WI-1234 fix", ""); id != "" || url != "" {
		t.Fatalf("got %q %q", id, url)
	}
}
