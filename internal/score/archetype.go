package score

import (
	"math"

	"github.com/siddham/synch/internal/profile"
)

// An archetype is a collaboration persona with a signature: the relative emphasis
// it expects across the five dimensions. We name the persona whose signature best
// matches the *shape* of a user's radar (cosine similarity over the dimensions we
// actually graded), so the result is deterministic and explainable — never a black
// box, and never dependent on a single axis in isolation.
type archetype struct {
	name  string
	blurb string
	sig   map[profile.Dimension]float64 // relative emphasis, 0..1 per dimension
}

// catalog is ordered; ties in similarity break toward the earlier entry, so the
// mapping from a radar shape to a persona is stable across runs.
var catalog = []archetype{
	{
		name:  "Architect",
		blurb: "You plan in precise strokes — grounded briefs and clean context, the work specced before a line is written.",
		sig:   map[profile.Dimension]float64{profile.PromptQuality: 1.0, profile.ContextManagement: 1.0, profile.IterationEfficiency: 0.4, profile.ToolLeverage: 0.3, profile.TokenEconomy: 0.4},
	},
	{
		name:  "Surgeon",
		blurb: "Precise, minimal incisions. You say exactly what's needed and land it in a few clean moves.",
		sig:   map[profile.Dimension]float64{profile.PromptQuality: 1.0, profile.IterationEfficiency: 1.0, profile.TokenEconomy: 0.8, profile.ContextManagement: 0.3, profile.ToolLeverage: 0.2},
	},
	{
		name:  "Conductor",
		blurb: "You direct a whole orchestra of tools, agents, and servers — delegating the work in concert.",
		sig:   map[profile.Dimension]float64{profile.ToolLeverage: 1.0, profile.ContextManagement: 0.5, profile.PromptQuality: 0.4, profile.IterationEfficiency: 0.4, profile.TokenEconomy: 0.3},
	},
	{
		name:  "Pathfinder",
		blurb: "You map unknown territory by probing — reading, searching, and tracing until its shape appears.",
		sig:   map[profile.Dimension]float64{profile.ToolLeverage: 1.0, profile.IterationEfficiency: 0.9, profile.PromptQuality: 0.3, profile.ContextManagement: 0.2, profile.TokenEconomy: 0.2},
	},
	{
		name:  "Economist",
		blurb: "You extract maximum signal per token — cache-savvy, context-stable, quietly cost-lean.",
		sig:   map[profile.Dimension]float64{profile.TokenEconomy: 1.0, profile.ContextManagement: 0.6, profile.IterationEfficiency: 0.4, profile.PromptQuality: 0.3, profile.ToolLeverage: 0.2},
	},
	{
		name:  "Marathoner",
		blurb: "You go deep for hours and keep context clean the whole way — sustained, disciplined sessions.",
		sig:   map[profile.Dimension]float64{profile.ContextManagement: 1.0, profile.TokenEconomy: 0.7, profile.PromptQuality: 0.5, profile.ToolLeverage: 0.4, profile.IterationEfficiency: 0.3},
	},
}

// deriveArchetype names a collaboration persona from the *shape* of the radar so it
// is deterministic and explainable, not a black box.
func deriveArchetype(scoreByDim map[profile.Dimension]float64, suff map[profile.Dimension]bool) profile.Archetype {
	// Rank the sufficient dimensions (stable selection sort, deterministic on ties
	// via profile.Order).
	type ds struct {
		d profile.Dimension
		v float64
	}
	var ranked []ds
	for _, d := range profile.Order {
		if suff[d] {
			ranked = append(ranked, ds{d, scoreByDim[d]})
		}
	}
	if len(ranked) == 0 {
		return profile.Archetype{
			Name:        "Newcomer",
			Blurb:       "Not enough history yet to profile your style.",
			Explanation: "No dimension had sufficient data. Keep using your AI tools and re-run synch.",
		}
	}
	for i := 0; i < len(ranked); i++ {
		best := i
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].v > ranked[best].v {
				best = j
			}
		}
		ranked[i], ranked[best] = ranked[best], ranked[i]
	}
	top := ranked[0]

	explain := "Your strongest axis is " + top.d.Title() + "."
	if len(ranked) > 1 {
		explain += " " + ranked[1].d.Title() + " is next."
	}

	// Distinctive signal: heavy back-and-forth. If Iteration Efficiency is sufficient,
	// notably low, and the weakest axis, the working style is conversational — a real,
	// recognizable persona the shape-match would otherwise smooth over.
	if v, ok := scoreByDim[profile.IterationEfficiency]; ok && suff[profile.IterationEfficiency] {
		if v < 5 && ranked[len(ranked)-1].d == profile.IterationEfficiency {
			return profile.Archetype{
				Name:        "Conversationalist",
				Blurb:       "You think out loud with AI — fast, iterative, exploratory; you get there by dialogue, not by spec.",
				Explanation: explain,
			}
		}
	}

	// A balanced radar (no real spike) is its own thing — a Generalist — not the
	// nearest spiky persona. Requires enough axes to judge balance.
	if len(ranked) >= 3 && ranked[0].v-ranked[len(ranked)-1].v < 1.2 {
		return profile.Archetype{
			Name:        "Generalist",
			Blurb:       "No single mode — you're fluent across the board and adapt to whatever the task needs.",
			Explanation: explain,
		}
	}

	// Cosine similarity is degenerate with a single sufficient axis (every vector is
	// colinear), so fall back to that axis's canonical persona.
	if len(ranked) < 2 {
		a := singleAxisArchetype(top.d)
		a.Explanation = explain
		return a
	}

	best := matchCatalog(scoreByDim, suff)
	return profile.Archetype{Name: best.name, Blurb: best.blurb, Explanation: explain}
}

// matchCatalog returns the archetype whose signature is most cosine-similar to the
// user's radar, compared only over the dimensions that were actually graded.
func matchCatalog(scoreByDim map[profile.Dimension]float64, suff map[profile.Dimension]bool) archetype {
	bestIdx, bestSim := 0, math.Inf(-1)
	for i, a := range catalog {
		sim := cosineOverSufficient(scoreByDim, a.sig, suff)
		if sim > bestSim {
			bestSim, bestIdx = sim, i
		}
	}
	return catalog[bestIdx]
}

// cosineOverSufficient computes cosine similarity between the user's scores and a
// signature, restricted to the sufficient dimensions so an ungraded axis neither
// helps nor hurts the match.
func cosineOverSufficient(scoreByDim, sig map[profile.Dimension]float64, suff map[profile.Dimension]bool) float64 {
	var dot, na, nb float64
	for _, d := range profile.Order {
		if !suff[d] {
			continue
		}
		a, b := scoreByDim[d], sig[d]
		dot += a * b
		na += a * a
		nb += b * b
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// singleAxisArchetype maps a lone dominant dimension to its canonical persona.
func singleAxisArchetype(d profile.Dimension) profile.Archetype {
	switch d {
	case profile.PromptQuality:
		return profile.Archetype{Name: "Architect", Blurb: "You plan in precise strokes — grounded briefs and clean context, the work specced before a line is written."}
	case profile.ContextManagement:
		return profile.Archetype{Name: "Marathoner", Blurb: "You go deep for hours and keep context clean the whole way — sustained, disciplined sessions."}
	case profile.ToolLeverage:
		return profile.Archetype{Name: "Conductor", Blurb: "You direct a whole orchestra of tools, agents, and servers — delegating the work in concert."}
	case profile.IterationEfficiency:
		return profile.Archetype{Name: "Surgeon", Blurb: "Precise, minimal incisions. You say exactly what's needed and land it in a few clean moves."}
	case profile.TokenEconomy:
		return profile.Archetype{Name: "Economist", Blurb: "You extract maximum signal per token — cache-savvy, context-stable, quietly cost-lean."}
	default:
		return profile.Archetype{Name: "Generalist", Blurb: "No single mode — you're fluent across the board and adapt to whatever the task needs."}
	}
}
