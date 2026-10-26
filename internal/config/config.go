// Package config reads kk prs settings from git config.
package config

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/localshred/koolkit/pkg/run"
)

// Settings holds the git-config-driven options for kk prs released.
type Settings struct {
	CloneURL        string
	ExtraTrunks     []string
	IgnoreRepos     map[string]bool
	ReleasePatterns []string
	Weeks           int
	WorkItemPattern *regexp.Regexp
	WorkItemURL     string
}

const (
	defaultCloneURL = "git@github.com:{repo}.git"
	defaultWeeks    = 4
)

// DefaultReleasePatterns are the origin branch globs used when
// kk.releasePattern is unset.
var DefaultReleasePatterns = []string{"release*", "release*/**"}

// Load reads every setting through r, applying defaults for unset or
// invalid values. An invalid kk.prs.workItemPattern leaves WorkItemPattern
// nil.
func Load(r run.Runner) Settings {
	s := Settings{
		CloneURL:        defaultCloneURL,
		ExtraTrunks:     fields(lines(r, "kk.trunks")),
		IgnoreRepos:     map[string]bool{},
		ReleasePatterns: DefaultReleasePatterns,
		Weeks:           defaultWeeks,
	}
	if patterns := fields(lines(r, "kk.releasePattern")); len(patterns) > 0 {
		s.ReleasePatterns = patterns
	}
	for _, repo := range fields(lines(r, "kk.prs.ignoreRepo")) {
		s.IgnoreRepos[repo] = true
	}
	if v := last(lines(r, "kk.prs.cloneURL")); v != "" {
		s.CloneURL = v
	}
	if n, err := strconv.Atoi(last(lines(r, "kk.prs.weeks"))); err == nil && n > 0 {
		s.Weeks = n
	}
	if v := last(lines(r, "kk.prs.workItemPattern")); v != "" {
		if re, err := regexp.Compile(v); err == nil {
			s.WorkItemPattern = re
		}
	}
	s.WorkItemURL = last(lines(r, "kk.prs.workItemURL"))
	return s
}

// Trunks returns defaultBranch (when non-empty), main, master, then
// ExtraTrunks, with duplicates removed.
func (s Settings) Trunks(defaultBranch string) []string {
	names := []string{}
	seen := map[string]bool{}
	candidates := append([]string{defaultBranch, "main", "master"}, s.ExtraTrunks...)
	for _, name := range candidates {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// WorkItemLink returns the first WorkItemPattern match in title, else in
// body, and the WorkItemURL with {id} replaced. It returns empty strings
// when the pattern or URL is unset or nothing matches.
func (s Settings) WorkItemLink(title, body string) (id, url string) {
	if s.WorkItemPattern == nil || s.WorkItemURL == "" {
		return "", ""
	}
	id = s.WorkItemPattern.FindString(title)
	if id == "" {
		id = s.WorkItemPattern.FindString(body)
	}
	if id == "" {
		return "", ""
	}
	return id, strings.ReplaceAll(s.WorkItemURL, "{id}", id)
}

func fields(lines []string) []string {
	values := []string{}
	for _, line := range lines {
		values = append(values, strings.Fields(line)...)
	}
	return values
}

func last(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

func lines(r run.Runner, key string) []string {
	res, err := r.Run("git", "config", "--get-all", key)
	if err != nil {
		return nil
	}
	out := []string{}
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
