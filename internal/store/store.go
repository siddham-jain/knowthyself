// Package store defines the Repository seam over the local cache and provides a
// SQLite-backed implementation. Scoring and reporting depend only on the Repository
// interface, so the cache backend can be swapped or mocked without touching them.
package store

import (
	"context"
	"time"

	"github.com/siddham-jain/knowthyself/internal/model"
)

// SyncState records what we last knew about a session's backing file, so delta-sync
// can re-parse only what changed (compare ModTime/Size).
type SyncState struct {
	SessionID  string
	Source     string
	Path       string
	ModTime    time.Time
	Size       int64
	LastSynced time.Time
}

// Repository is the persistence contract: the normalized sessions and the sync
// bookkeeping that powers <50ms warm boots.
type Repository interface {
	// SyncStates returns the last-known file state for every cached session of a
	// source, keyed by session id, so the engine can diff against Discover output.
	SyncStates(ctx context.Context, source string) (map[string]SyncState, error)
	// SaveSession upserts a parsed session and its sync state atomically.
	SaveSession(ctx context.Context, s model.Session, st SyncState) error
	// DeleteSession removes a session no longer present on disk.
	DeleteSession(ctx context.Context, source, sessionID string) error
	// LoadSessions returns all cached sessions for a source (normalized model),
	// used by the scoring pass. Ordered by StartedAt for determinism.
	LoadSessions(ctx context.Context, source string) ([]model.Session, error)
	// Close releases the underlying resources.
	Close() error
}
