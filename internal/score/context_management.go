package score

import (
	"github.com/siddham/synch/internal/model"
	"github.com/siddham/synch/internal/profile"
)

// contextManagement grades context hygiene: deliberate resets (/clear), planning
// (plan mode), cache reuse (a proxy for stable, reused context), and whether the
// user let context overflow into forced compactions. It's about discipline, not
// raw token cost (that's Token Economy).
type contextManagement struct{}

func (contextManagement) Dimension() profile.Dimension { return profile.ContextManagement }
func (contextManagement) Weight() float64              { return 0.15 }

func (contextManagement) Score(s model.Session) profile.Signal {
	v := newView(s)
	if len(v.assistant) < minAssistantTurns {
		return profile.Signal{Sufficient: false}
	}

	var clears, plans, compactions int
	for _, t := range v.all {
		if t.SlashCommand == "/clear" {
			clears++
		}
		if t.PermissionMode == "plan" {
			plans++
		}
		if t.IsCompaction {
			compactions++
		}
	}
	cacheReuse := ratio(float64(s.Tokens.CacheRead), float64(s.Tokens.TotalInput()))

	score := 5.0
	score += clamp(cacheReuse*1.5, 0, 1.5) // reused context => coherent working set
	if clears > 0 {
		score += 1.5 // deliberate resets at task boundaries
	}
	if plans > 0 {
		score += 1.0 // planning before acting
	}
	// Forced compaction implies context was allowed to overflow; mild penalty.
	score -= clamp(float64(compactions)*0.75, 0, 2.0)

	score = clamp(score, 0, 10)

	return profile.Signal{
		Score:        score,
		Sufficient:   true,
		Observations: float64(len(v.assistant)),
		Evidence: map[string]float64{
			"clears":            float64(clears),
			"plan_mode_turns":   float64(plans),
			"compactions":       float64(compactions),
			"cache_reuse_x1000": cacheReuse * 1000, // stored as int-ish for summing sanity
		},
	}
}

func init() { Register(contextManagement{}) }
