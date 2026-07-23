package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// checkEvery is how stale a cached release check may get before it is refreshed.
const checkEvery = 24 * time.Hour

// cache is the on-disk record of the last release check.
type cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// Notice reports a newer released version, or "" when there is nothing to say.
//
// It never blocks and never fails: the answer comes from the cache written by a
// previous run, and a stale cache is refreshed in the background for next time. A
// version nudge is not worth a single millisecond of startup latency.
func Notice(dir, current string) string {
	if os.Getenv("KNOWTHYSELF_NO_UPDATE_CHECK") != "" || current == "dev" {
		return ""
	}
	c, err := readCache(dir)
	if err != nil || time.Since(c.CheckedAt) > checkEvery {
		go refresh(dir)
	}
	if c.Latest != "" && Compare(current, c.Latest) < 0 {
		return c.Latest
	}
	return ""
}

func cachePath(dir string) string { return filepath.Join(dir, ".update-check") }

func readCache(dir string) (cache, error) {
	var c cache
	b, err := os.ReadFile(cachePath(dir))
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

// refresh checks GitHub and records the result. Every error is swallowed: this runs
// detached, and a failed background check must never surface to the user.
func refresh(dir string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rel, err := Latest(ctx)
	if err != nil {
		return
	}
	b, err := json.Marshal(cache{CheckedAt: time.Now(), Latest: rel.Version})
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(cachePath(dir), b, 0o644)
}
