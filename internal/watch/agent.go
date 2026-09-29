package watch

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed launchd/com.localshred.kk-prs-watch.plist
var plistTemplate string

const agentLabel = "com.localshred.kk-prs-watch"

// EnsureAgent installs and loads the launchd agent if not already present.
func EnsureAgent(home string) error {
	dest := filepath.Join(home, "Library", "LaunchAgents", agentLabel+".plist")
	switch _, err := os.Stat(dest); {
	case err == nil:
		// already installed
	case os.IsNotExist(err):
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, []byte(renderPlist(home)), 0o644); err != nil {
			return err
		}
	default:
		return err
	}
	if exec.Command("launchctl", "list", agentLabel).Run() != nil {
		return exec.Command("launchctl", "load", dest).Run()
	}
	return nil
}

func renderPlist(home string) string {
	return strings.ReplaceAll(plistTemplate, "__HOME__", home)
}
