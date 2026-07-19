// Package insight defines the InsightEngine seam: how actionable tips are generated
// from a computed Profile. The default engine is deterministic and template-based.
// An optional --deep-eval engine may call an LLM to *phrase* qualitative insights,
// but it consumes the already-computed Profile and can never change a score.
package insight

import (
	"context"

	"github.com/siddham-jain/knowthyself/internal/profile"
)

// Engine turns a computed Profile into a list of insights (tips).
type Engine interface {
	// Name identifies the engine ("heuristic", "deep-eval").
	Name() string
	// Generate produces insights from the profile. Implementations MUST treat the
	// profile's scores as read-only inputs.
	Generate(ctx context.Context, p profile.Profile) ([]profile.Insight, error)
}
