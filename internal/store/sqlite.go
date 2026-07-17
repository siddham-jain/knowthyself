package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/siddham/reflect/internal/model"
	_ "modernc.org/sqlite" // pure-Go driver, no cgo
)

// SQLite is the default Repository, backed by a single local database file. Sessions
// are stored as JSON blobs of the normalized model alongside sync bookkeeping, so a
// warm boot (nothing changed on disk) skips the expensive JSONL re-parse entirely.
type SQLite struct {
	db *sql.DB
}

// DefaultPath returns the cache DB path: $XDG_CONFIG_HOME/reflect/store.db, falling
// back to ~/.config/reflect/store.db.
func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, ".config")
		} else {
			base = ".config"
		}
	}
	return filepath.Join(base, "reflect", "store.db")
}

// Open opens (creating if needed) the SQLite cache at path. Pass ":memory:" for a
// throwaway in-memory store (tests). The parent directory is created as needed.
func Open(path string) (*SQLite, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("store: mkdir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// One connection avoids SQLite "database is locked" churn for our access pattern.
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("store: %q: %w", pragma, err)
		}
	}
	s := &SQLite{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLite) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS sessions (
    source      TEXT    NOT NULL,
    id          TEXT    NOT NULL,
    path        TEXT    NOT NULL,
    mod_time    INTEGER NOT NULL,  -- unix nanoseconds
    size        INTEGER NOT NULL,
    last_synced INTEGER NOT NULL,  -- unix nanoseconds
    started_at  INTEGER NOT NULL,  -- unix nanoseconds (0 if unknown)
    data        BLOB    NOT NULL,  -- JSON of model.Session
    PRIMARY KEY (source, id)
);`)
	return err
}

// SyncStates returns the last-known file state for every cached session of a source.
func (s *SQLite) SyncStates(ctx context.Context, source string) (map[string]SyncState, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, path, mod_time, size, last_synced FROM sessions WHERE source = ?`, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SyncState{}
	for rows.Next() {
		var st SyncState
		var mod, size, synced int64
		if err := rows.Scan(&st.SessionID, &st.Path, &mod, &size, &synced); err != nil {
			return nil, err
		}
		st.Source = source
		st.ModTime = time.Unix(0, mod)
		st.Size = size
		st.LastSynced = time.Unix(0, synced)
		out[st.SessionID] = st
	}
	return out, rows.Err()
}

// SaveSession upserts a parsed session and its sync state.
func (s *SQLite) SaveSession(ctx context.Context, sess model.Session, st SyncState) error {
	blob, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("store: marshal session %s: %w", sess.ID, err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO sessions (source, id, path, mod_time, size, last_synced, started_at, data)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(source, id) DO UPDATE SET
    path=excluded.path, mod_time=excluded.mod_time, size=excluded.size,
    last_synced=excluded.last_synced, started_at=excluded.started_at, data=excluded.data`,
		sess.Source, sess.ID, st.Path, st.ModTime.UnixNano(), st.Size,
		st.LastSynced.UnixNano(), sess.StartedAt.UnixNano(), blob)
	return err
}

// DeleteSession removes a session that no longer exists on disk.
func (s *SQLite) DeleteSession(ctx context.Context, source, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE source = ? AND id = ?`, source, sessionID)
	return err
}

// LoadSessions returns all cached sessions for a source, ordered by StartedAt.
func (s *SQLite) LoadSessions(ctx context.Context, source string) ([]model.Session, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT data FROM sessions WHERE source = ? ORDER BY started_at, id`, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Session
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			return nil, err
		}
		var sess model.Session
		if err := json.Unmarshal(blob, &sess); err != nil {
			// A corrupt row shouldn't sink the whole run; skip it.
			continue
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// Close releases the database.
func (s *SQLite) Close() error { return s.db.Close() }
