package engine

import (
	"context"
	"path/filepath"
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
		Sessions:      perSessionSummaries(sessions),
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

// perSessionSummaries scores each session individually (Evaluate over a single
// session yields its own dimension scores) to build the chronological timeline used
// by the drill-down and trend views. Sessions arrive ordered by StartedAt.
func perSessionSummaries(sessions []model.Session) []profile.SessionSummary {
	out := make([]profile.SessionSummary, 0, len(sessions))
	for _, s := range sessions {
		ev := score.Evaluate([]model.Session{s})
		dims := map[profile.Dimension]float64{}
		for _, d := range ev.Dimensions {
			if d.Signal.Sufficient {
				dims[d.Dimension] = d.Signal.Score
			}
		}
		prompts := 0
		for _, t := range s.Turns {
			if t.Scorable() {
				prompts++
			}
		}
		out = append(out, profile.SessionSummary{
			ID:         s.ID,
			Label:      sessionLabel(s),
			StartedAt:  s.StartedAt,
			Overall:    ev.Overall,
			Dimensions: dims,
			Prompts:    prompts,
			Tokens:     s.Tokens.Output + s.Tokens.TotalInput(),
		})
	}
	return out
}

// sessionLabel builds a short human label: project basename + start date.
func sessionLabel(s model.Session) string {
	name := filepath.Base(s.Cwd)
	if name == "" || name == "." || name == "/" {
		name = s.ID
		if len(name) > 8 {
			name = name[:8]
		}
	}
	if !s.StartedAt.IsZero() {
		return name + " · " + s.StartedAt.Format("01-02 15:04")
	}
	return name
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
