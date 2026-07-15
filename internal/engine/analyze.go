package engine

import (
	"context"
	"sort"

	"github.com/siddham/synch/internal/insight"
	"github.com/siddham/synch/internal/model"
	"github.com/siddham/synch/internal/profile"
	"github.com/siddham/synch/internal/score"
)

// Analyze runs the scoring engine over the sessions, computes headline stats, and
// attaches insights from the given engine, producing a complete Profile.
func Analyze(ctx context.Context, sessions []model.Session, ie insight.Engine, now Clock) (profile.Profile, error) {
	eval := score.Evaluate(sessions)
	p := profile.Profile{
		SchemaVersion: profile.SchemaVersion,
		GeneratedAt:   now(),
		Source:        sourceOf(sessions),
		Overall:       eval.Overall,
		Dimensions:    eval.Dimensions,
		Archetype:     eval.Archetype,
		Stats:         computeStats(sessions),
	}
	if ie != nil {
		tips, err := ie.Generate(ctx, p)
		if err != nil {
			return p, err
		}
		p.Insights = tips
	}
	return p, nil
}

func sourceOf(sessions []model.Session) string {
	if len(sessions) > 0 {
		return sessions[0].Source
	}
	return ""
}

// computeStats derives the fun, headline numbers shown alongside the grade.
func computeStats(sessions []model.Session) profile.Stats {
	st := profile.Stats{Sessions: len(sessions), PermissionModeMix: map[string]int{}}
	toolCounts := map[string]int{}
	slashCounts := map[string]int{}
	var totalInput, cacheRead int64

	for _, s := range sessions {
		st.Turns += len(s.Turns)
		st.TotalTokens += s.Tokens.Output + s.Tokens.TotalInput()
		totalInput += s.Tokens.TotalInput()
		cacheRead += s.Tokens.CacheRead
		for _, t := range s.Turns {
			if t.Scorable() {
				st.UserPrompts++
			}
			if t.SlashCommand != "" {
				slashCounts[t.SlashCommand]++
			}
			if t.PermissionMode != "" {
				st.PermissionModeMix[t.PermissionMode]++
			}
			for _, tc := range t.ToolCalls {
				toolCounts[tc.Name]++
			}
		}
	}
	if totalInput > 0 {
		st.CacheHitRate = float64(cacheRead) / float64(totalInput)
	}
	st.TopTools = topN(toolCounts, 5)
	st.TopSlashCommands = topN(slashCounts, 5)
	return st
}

// topN returns the highest-count entries, ordered by count desc then name asc for
// deterministic output.
func topN(m map[string]int, n int) []profile.Count {
	out := make([]profile.Count, 0, len(m))
	for k, v := range m {
		out = append(out, profile.Count{Name: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}
