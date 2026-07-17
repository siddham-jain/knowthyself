package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/siddham/reflect/internal/provider/claude"
	"github.com/siddham/reflect/internal/store"
)

// fixedClock returns a deterministic time for reproducible sync bookkeeping.
func fixedClock() time.Time { return time.Unix(1_700_000_000, 0) }

// writeSession writes a minimal one-line session file under a Claude-style base.
func writeSession(t *testing.T, base, project, name, content string) string {
	t.Helper()
	dir := filepath.Join(base, "projects", project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSyncDeltaAndDelete(t *testing.T) {
	base := t.TempDir()
	p := claude.New(base)
	repo, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	ctx := context.Background()

	writeSession(t, base, "-proj-a", "s1.jsonl",
		`{"type":"user","message":{"role":"user","content":"first"}}`+"\n")
	writeSession(t, base, "-proj-a", "s2.jsonl",
		`{"type":"user","message":{"role":"user","content":"second"}}`+"\n")

	// First sync: both parsed.
	res, err := Sync(ctx, p, repo, fixedClock)
	if err != nil {
		t.Fatal(err)
	}
	if res.Parsed != 2 || res.Unchanged != 0 {
		t.Fatalf("first sync: %+v", res)
	}

	// Second sync, nothing changed: both skipped via delta.
	res, err = Sync(ctx, p, repo, fixedClock)
	if err != nil {
		t.Fatal(err)
	}
	if res.Parsed != 0 || res.Unchanged != 2 {
		t.Fatalf("delta sync should skip unchanged: %+v", res)
	}

	// Modify s2 (append a line + bump mtime): only s2 re-parsed.
	p2 := writeSession(t, base, "-proj-a", "s2.jsonl",
		`{"type":"user","message":{"role":"user","content":"second"}}`+"\n"+
			`{"type":"user","message":{"role":"user","content":"more"}}`+"\n")
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(p2, future, future); err != nil {
		t.Fatal(err)
	}
	res, err = Sync(ctx, p, repo, fixedClock)
	if err != nil {
		t.Fatal(err)
	}
	if res.Parsed != 1 || res.Unchanged != 1 {
		t.Fatalf("only changed file should re-parse: %+v", res)
	}

	// Delete s1 on disk: sync removes it from the cache.
	if err := os.Remove(filepath.Join(base, "projects", "-proj-a", "s1.jsonl")); err != nil {
		t.Fatal(err)
	}
	res, err = Sync(ctx, p, repo, fixedClock)
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted != 1 {
		t.Fatalf("expected 1 deletion: %+v", res)
	}

	sessions, err := repo.LoadSessions(ctx, p.ID())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].ID != "s2" {
		t.Fatalf("expected only s2 to remain, got %+v", sessions)
	}
}

func TestSyncEmptyBase(t *testing.T) {
	// No ~/.claude at all must be a clean no-op, not an error.
	p := claude.New(filepath.Join(t.TempDir(), "does-not-exist"))
	repo, _ := store.Open(":memory:")
	defer repo.Close()
	res, err := Sync(context.Background(), p, repo, fixedClock)
	if err != nil {
		t.Fatal(err)
	}
	if res.Discovered != 0 || res.Parsed != 0 {
		t.Fatalf("empty base should be a no-op: %+v", res)
	}
}
