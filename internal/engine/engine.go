// Package engine orchestrates the analysis pipeline:
//
//	Discover -> (delta) Parse -> Store -> Load -> Score -> Aggregate -> Profile
//
// It depends only on the interface seams (provider.Provider, store.Repository, and
// the score package), so providers, scorers, and cache backends stay swappable.
package engine

import (
	"context"
	"time"

	"github.com/siddham/synch/internal/model"
	"github.com/siddham/synch/internal/provider"
	"github.com/siddham/synch/internal/store"
)

// Clock returns the current time; injectable so tests stay deterministic.
type Clock func() time.Time

// SyncResult reports what a sync did, for progress/telemetry.
type SyncResult struct {
	Discovered int
	Parsed     int // sessions (re)parsed because they were new or changed
	Unchanged  int // sessions skipped via delta-sync
	Deleted    int // cached sessions removed because they vanished on disk
}

// Sync brings the repository in line with what the provider currently has on disk,
// re-parsing only sessions whose backing file changed (mtime or size). Returns a
// summary; the up-to-date sessions can then be read with repo.LoadSessions.
func Sync(ctx context.Context, p provider.Provider, repo store.Repository, now Clock) (SyncResult, error) {
	var res SyncResult
	refs, err := p.Discover(ctx)
	if err != nil {
		return res, err
	}
	res.Discovered = len(refs)

	prior, err := repo.SyncStates(ctx, p.ID())
	if err != nil {
		return res, err
	}

	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		seen[ref.SessionID] = true
		if st, ok := prior[ref.SessionID]; ok && unchanged(st, ref) {
			res.Unchanged++
			continue
		}
		sess, err := p.Parse(ctx, ref)
		if err != nil {
			// A single unreadable session shouldn't abort the whole sync.
			continue
		}
		st := store.SyncState{
			SessionID:  ref.SessionID,
			Source:     p.ID(),
			Path:       ref.Path,
			ModTime:    ref.ModTime,
			Size:       ref.Size,
			LastSynced: now(),
		}
		if err := repo.SaveSession(ctx, sess, st); err != nil {
			return res, err
		}
		res.Parsed++
	}

	// Drop sessions that no longer exist on disk.
	for id := range prior {
		if !seen[id] {
			if err := repo.DeleteSession(ctx, p.ID(), id); err != nil {
				return res, err
			}
			res.Deleted++
		}
	}
	return res, nil
}

// unchanged reports whether a discovered ref matches the cached sync state, so the
// session can be skipped. Size + mtime is the standard cheap change check.
func unchanged(st store.SyncState, ref provider.SessionRef) bool {
	return st.Size == ref.Size && st.ModTime.Equal(ref.ModTime)
}

// Sessions is a convenience that syncs then loads the normalized sessions.
func Sessions(ctx context.Context, p provider.Provider, repo store.Repository, now Clock) ([]model.Session, SyncResult, error) {
	res, err := Sync(ctx, p, repo, now)
	if err != nil {
		return nil, res, err
	}
	sess, err := repo.LoadSessions(ctx, p.ID())
	return sess, res, err
}
