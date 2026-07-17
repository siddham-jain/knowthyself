// Package score defines the Scorer seam and the deterministic scoring engine.
// Each of the five dimensions is a registered Scorer; adding a dimension or a whole
// alternate rubric (e.g. a recruiter's) is a registration, not a core edit.
package score

import (
	"sort"
	"sync"

	"github.com/siddham/reflect/internal/model"
	"github.com/siddham/reflect/internal/profile"
)

// Scorer grades one dimension for a single session. It returns a Signal carrying
// the 0..10 score plus the raw evidence that produced it. Scorers must be pure and
// deterministic: same session in, same signal out.
type Scorer interface {
	// Dimension identifies which axis this scorer grades.
	Dimension() profile.Dimension
	// Weight is this dimension's relative weight in the overall score (0..1-ish;
	// weights are normalized across sufficient dimensions at aggregation time).
	Weight() float64
	// Score grades a single session. Return a Signal with Sufficient=false when the
	// session lacks enough data for this dimension.
	Score(s model.Session) profile.Signal
}

// registry holds registered scorers, keyed by dimension.
var (
	mu       sync.RWMutex
	registry = map[profile.Dimension]Scorer{}
)

// Register adds a scorer. Panics on duplicate dimension. Call from an init().
func Register(s Scorer) {
	mu.Lock()
	defer mu.Unlock()
	registry[s.Dimension()] = s
}

// All returns every registered scorer in canonical dimension order (profile.Order
// first, then any extras alphabetically) so scoring output is deterministic.
func All() []Scorer {
	mu.RLock()
	defer mu.RUnlock()
	seen := map[profile.Dimension]bool{}
	out := make([]Scorer, 0, len(registry))
	for _, d := range profile.Order {
		if s, ok := registry[d]; ok {
			out = append(out, s)
			seen[d] = true
		}
	}
	extra := make([]Scorer, 0)
	for d, s := range registry {
		if !seen[d] {
			extra = append(extra, s)
		}
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].Dimension() < extra[j].Dimension() })
	return append(out, extra...)
}
