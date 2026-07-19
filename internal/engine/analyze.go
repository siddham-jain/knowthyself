package engine

import (
	"context"
	"path/filepath"
	"sort"
	"time"

	"github.com/siddham-jain/knowthyself/internal/insight"
	"github.com/siddham-jain/knowthyself/internal/model"
	"github.com/siddham-jain/knowthyself/internal/profile"
	"github.com/siddham-jain/knowthyself/internal/score"
	"github.com/siddham-jain/knowthyself/internal/text"
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
	p.Archetype.Traits = deriveTraits(p.Stats, now())
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

// computeStats derives the fun, headline numbers shown alongside the grade and
// behind the first-run reveal (hours together, tenure, peak hour, languages,
// projects). Everything is drawn from data already parsed onto the sessions.
func computeStats(sessions []model.Session) profile.Stats {
	st := profile.Stats{Sessions: len(sessions), PeakHour: -1, PermissionModeMix: map[string]int{}}
	toolCounts := map[string]int{}
	slashCounts := map[string]int{}
	scriptCounts := map[string]int{}
	projects := map[string]bool{}
	var hourHist [24]int
	var totalInput, cacheRead int64
	var totalDur time.Duration

	for _, s := range sessions {
		st.Turns += len(s.Turns)
		st.TotalTokens += s.Tokens.Output + s.Tokens.TotalInput()
		totalInput += s.Tokens.TotalInput()
		cacheRead += s.Tokens.CacheRead
		if s.Cwd != "" {
			projects[s.Cwd] = true
		}
		if !s.StartedAt.IsZero() && !s.EndedAt.IsZero() && s.EndedAt.After(s.StartedAt) {
			totalDur += s.EndedAt.Sub(s.StartedAt)
		}
		if !s.StartedAt.IsZero() && (st.FirstSessionAt.IsZero() || s.StartedAt.Before(st.FirstSessionAt)) {
			st.FirstSessionAt = s.StartedAt
		}
		for _, t := range s.Turns {
			if !t.Timestamp.IsZero() {
				hourHist[t.Timestamp.Hour()]++
			}
			if t.Scorable() {
				st.UserPrompts++
				scriptCounts[text.DominantScript(t.Text)]++
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
	st.CollabSeconds = int64(totalDur.Seconds())
	st.Projects = len(projects)
	st.PeakHour = peakHour(hourHist)
	st.Languages = languageMix(scriptCounts)
	st.TopTools = topN(toolCounts, 5)
	st.TopSlashCommands = topN(slashCounts, 5)
	return st
}

// peakHour returns the hour (0..23) with the most turn activity, or -1 if there is
// no timestamped activity. Ties break toward the earlier hour for determinism.
func peakHour(hist [24]int) int {
	best, bestH := 0, -1
	for h, c := range hist {
		if c > best {
			best, bestH = c, h
		}
	}
	return bestH
}

// languageMix maps dominant-script counts to friendly language names, most-used
// first. Latin→English, Devanagari→Hindi; "other" scripts are only surfaced when
// they're all there is, so a code-heavy corpus doesn't sprout a noisy label.
func languageMix(scriptCounts map[string]int) []string {
	type lc struct {
		name string
		n    int
	}
	var langs []lc
	if n := scriptCounts["latin"]; n > 0 {
		langs = append(langs, lc{"English", n})
	}
	if n := scriptCounts["devanagari"]; n > 0 {
		langs = append(langs, lc{"Hindi", n})
	}
	if len(langs) == 0 {
		if n := scriptCounts["other"]; n > 0 {
			langs = append(langs, lc{"Other", n})
		}
	}
	sort.SliceStable(langs, func(i, j int) bool { return langs[i].n > langs[j].n })
	out := make([]string, len(langs))
	for i, l := range langs {
		out[i] = l.name
	}
	return out
}

// deriveTraits produces the celebratory badges shown under the archetype. Each is
// deterministic and strictly positive framing — a flourish, never a deficiency.
func deriveTraits(st profile.Stats, now time.Time) []string {
	var traits []string
	if len(st.Languages) >= 2 {
		traits = append(traits, "Polyglot")
	}
	if !st.FirstSessionAt.IsZero() && now.Sub(st.FirstSessionAt) >= 90*24*time.Hour {
		traits = append(traits, "Veteran")
	}
	if st.Sessions > 0 && st.CollabSeconds/int64(st.Sessions) >= int64((45*time.Minute).Seconds()) {
		traits = append(traits, "Deep Diver")
	}
	switch {
	case st.PeakHour >= 21 || (st.PeakHour >= 0 && st.PeakHour <= 4):
		traits = append(traits, "Night Owl")
	case st.PeakHour >= 5 && st.PeakHour <= 8:
		traits = append(traits, "Early Bird")
	}
	return traits
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
