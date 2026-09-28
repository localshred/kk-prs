package watch

import (
	"strings"
	"testing"
)

func TestRenderPlistExpandsHome(t *testing.T) {
	out := renderPlist("/home/bj")
	if strings.Contains(out, "__HOME__") {
		t.Fatal("template still has __HOME__ placeholder")
	}
	if !strings.Contains(out, "com.localshred.kk-prs-watch") {
		t.Fatal("plist missing the agent label")
	}
	if !strings.Contains(out, "prs</string>") || !strings.Contains(out, "watch</string>") {
		t.Fatal("plist should invoke `kk prs watch tick`")
	}
}
