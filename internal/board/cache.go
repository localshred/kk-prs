package board

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Cache maps "owner/name#number" to the release a PR's merge commit was first
// found in. That fact never changes, so entries are never invalidated.
type Cache struct {
	entries map[string]string
	path    string
}

// LoadCache reads the cache at path. A missing or corrupt file yields an empty
// cache that still saves to path.
func LoadCache(path string) *Cache {
	c := &Cache{entries: map[string]string{}, path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	var entries map[string]string
	if err := json.Unmarshal(data, &entries); err != nil || entries == nil {
		return c
	}
	c.entries = entries
	return c
}

// Get returns the cached release for key.
func (c *Cache) Get(key string) (release string, ok bool) {
	release, ok = c.entries[key]
	return release, ok
}

// Put records the release for key.
func (c *Cache) Put(key, release string) {
	c.entries[key] = release
}

// Save writes the cache as a JSON object, creating parent directories. The
// write goes to a temp file in the same directory and is renamed into place.
func (c *Cache) Save() error {
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c.entries)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(c.path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, c.path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
