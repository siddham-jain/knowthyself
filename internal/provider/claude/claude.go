// Package claude implements the Provider seam for Claude Code, whose sessions live
// as streaming JSONL at $CLAUDE_CONFIG_DIR/projects/<encoded-cwd>/<uuid>.jsonl
// (default base ~/.claude). The format is an undocumented internal schema that
// drifts across versions, so parsing is deliberately defensive: key off the
// top-level "type" discriminator, tolerate unknown fields, and never crash on a
// malformed line.
package claude

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/siddham/synch/internal/provider"
)

// ID is the source identifier for Claude Code.
const ID = "claude-code"

// Provider discovers and parses Claude Code sessions.
type Provider struct {
	// base is the Claude config root (default ~/.claude). Injected for testability.
	base string
}

// New returns a Provider rooted at the given base directory. An empty base resolves
// to $CLAUDE_CONFIG_DIR, then ~/.claude.
func New(base string) *Provider {
	if base == "" {
		base = DefaultBase()
	}
	return &Provider{base: base}
}

// DefaultBase resolves the Claude Code config root: $CLAUDE_CONFIG_DIR if set,
// otherwise ~/.claude. (Claude Code does not follow XDG.)
func DefaultBase() string {
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

// ID implements provider.Provider.
func (p *Provider) ID() string { return ID }

// Discover enumerates every session .jsonl under projects/, returning cheap refs
// (path + mtime + size) that drive delta-sync. Missing dirs yield an empty set,
// not an error — a machine may simply have no Claude Code history.
func (p *Provider) Discover(ctx context.Context) ([]provider.SessionRef, error) {
	projects := filepath.Join(p.base, "projects")
	entries, err := os.ReadDir(projects)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var refs []provider.SessionRef
	for _, proj := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !proj.IsDir() {
			continue
		}
		dir := filepath.Join(projects, proj.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			continue // unreadable project dir: skip, don't fail the whole run
		}
		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".jsonl" {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			refs = append(refs, provider.SessionRef{
				SessionID: sessionIDFromFile(f.Name()),
				Path:      filepath.Join(dir, f.Name()),
				ModTime:   info.ModTime(),
				Size:      info.Size(),
			})
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Path < refs[j].Path })
	return refs, nil
}

// sessionIDFromFile strips the .jsonl extension to recover the session UUID.
func sessionIDFromFile(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}

func init() {
	// Register the default provider so a plain build wires Claude Code in. Tests and
	// the engine may still construct provider.New(base) with an explicit root.
	provider.Register(New(""))
}
