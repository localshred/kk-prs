package watch

import (
	"bytes"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed launchd/com.localshred.kk-prs-watch.plist
var plistTemplate string

const agentLabel = "com.localshred.kk-prs-watch"

// EnsureAgent installs the launchd agent pointing at the running kk-prs
// binary and loads it. A missing or stale plist (e.g. one pointing at a
// binary path that no longer applies) is rewritten and reloaded, so the agent
// self-heals whenever a watch command runs.
func EnsureAgent(home string) error {
	program, err := os.Executable()
	if err != nil {
		return err
	}
	dest := filepath.Join(home, "Library", "LaunchAgents", agentLabel+".plist")
	want := renderPlist(program)
	if plistStale(dest, want) {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, []byte(want), 0o644); err != nil {
			return err
		}
		_ = exec.Command("launchctl", "unload", dest).Run()
		return exec.Command("launchctl", "load", dest).Run()
	}
	if exec.Command("launchctl", "list", agentLabel).Run() != nil {
		return exec.Command("launchctl", "load", dest).Run()
	}
	return nil
}

// plistStale reports whether the plist at dest is missing or differs from
// the rendered content.
func plistStale(dest, want string) bool {
	got, err := os.ReadFile(dest)
	return err != nil || !bytes.Equal(got, []byte(want))
}

func renderPlist(program string) string {
	return strings.ReplaceAll(plistTemplate, "__PROGRAM__", program)
}
