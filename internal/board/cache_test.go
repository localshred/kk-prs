package board

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "cache.json")
	c := LoadCache(path)
	c.Put("acme/svc-a#12", "release-A")
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, ok := LoadCache(path).Get("acme/svc-a#12")
	if !ok || got != "release-A" {
		t.Fatalf("Get = %q, %v; want release-A, true", got, ok)
	}
	if _, ok := LoadCache(path).Get("acme/svc-b#1"); ok {
		t.Fatal("unexpected hit for absent key")
	}
}

func TestLoadCacheCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := LoadCache(path)
	if _, ok := c.Get("acme/svc-a#12"); ok {
		t.Fatal("corrupt file should load empty")
	}
	c.Put("acme/svc-a#12", "release-B")
	if err := c.Save(); err != nil {
		t.Fatalf("Save after corrupt load: %v", err)
	}
	if got, _ := LoadCache(path).Get("acme/svc-a#12"); got != "release-B" {
		t.Fatalf("Get = %q; want release-B", got)
	}
}

func TestLoadCacheMissing(t *testing.T) {
	c := LoadCache(filepath.Join(t.TempDir(), "absent.json"))
	if _, ok := c.Get("acme/svc-a#12"); ok {
		t.Fatal("missing file should load empty")
	}
}
