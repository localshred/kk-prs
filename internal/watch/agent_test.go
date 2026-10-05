package watch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderPlistInvokesPluginBinary(t *testing.T) {
	out := renderPlist("/Users/bj/.local/share/koolkit/bin/kk-prs")
	if strings.Contains(out, "__PROGRAM__") {
		t.Fatal("template still has __PROGRAM__ placeholder")
	}
	if strings.Contains(out, "/opt/homebrew/bin/kk") {
		t.Fatal("plist must not hardcode the Homebrew kk path")
	}
	want := "<string>/Users/bj/.local/share/koolkit/bin/kk-prs</string>\n\t\t<string>watch</string>\n\t\t<string>tick</string>"
	if !strings.Contains(out, want) {
		t.Fatalf("plist should invoke `<kk-prs> watch tick`, got:\n%s", out)
	}
	if !strings.Contains(out, "com.localshred.kk-prs-watch") {
		t.Fatal("plist missing the agent label")
	}
}

func TestPlistStale(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "agent.plist")
	if !plistStale(dest, "new") {
		t.Error("missing plist should be stale")
	}
	os.WriteFile(dest, []byte("old"), 0o644)
	if !plistStale(dest, "new") {
		t.Error("plist with different content should be stale")
	}
	os.WriteFile(dest, []byte("new"), 0o644)
	if plistStale(dest, "new") {
		t.Error("plist matching the rendered content should not be stale")
	}
}
