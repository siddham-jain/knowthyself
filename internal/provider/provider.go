// Package provider defines the Provider seam: how a specific AI-coding tool's
// on-disk logs are discovered and parsed into the normalized model. Adding a new
// source (Codex, OpenCode, Gemini) means implementing Provider and registering it —
// no change to scoring, storage, or reporting.
package provider

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/siddham/reflect/internal/model"
)

// SessionRef is a lightweight handle to a discoverable session, returned by
// Discover before the (potentially expensive) Parse. It carries just enough to
// drive delta-sync: a stable id and the on-disk file's mod time / size.
type SessionRef struct {
	SessionID string    // provider-unique session id
	Path      string    // primary file backing this session (for mtime/size checks)
	ModTime   time.Time // file modification time
	Size      int64     // file size in bytes
	Extra     any       // provider-private payload passed back to Parse (optional)
}

// Provider discovers and parses one tool's sessions.
type Provider interface {
	// ID is the stable source identifier, e.g. "claude-code".
	ID() string
	// Discover locates and enumerates all sessions on disk without fully parsing
	// them. Cheap: used to compute the delta-sync work set.
	Discover(ctx context.Context) ([]SessionRef, error)
	// Parse turns one discovered session into the normalized model. Must be
	// defensive: tolerate malformed lines, unknown record types, and schema drift.
	Parse(ctx context.Context, ref SessionRef) (model.Session, error)
}

// registry holds registered providers by id. Providers self-register via Register
// in an init(), so enabling a source is a single import.
var (
	mu       sync.RWMutex
	registry = map[string]Provider{}
)

// Register adds a provider to the global registry. Panics on duplicate id (a
// programming error). Call from an init() in the provider's package.
func Register(p Provider) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[p.ID()]; dup {
		panic(fmt.Sprintf("provider: duplicate registration for %q", p.ID()))
	}
	registry[p.ID()] = p
}

// Get returns the provider with the given id, or an error if none is registered.
func Get(id string) (Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[id]
	if !ok {
		return nil, fmt.Errorf("provider: no provider registered with id %q", id)
	}
	return p, nil
}

// All returns every registered provider, ordered by id for deterministic behavior.
func All() []Provider {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Provider, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}
