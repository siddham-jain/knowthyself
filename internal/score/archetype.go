package score

import "github.com/siddham/synch/internal/profile"

// deriveArchetype names a collaboration persona from the *shape* of the radar — the
// strongest dimension(s) — so it is deterministic and explainable, not a black box.
func deriveArchetype(scoreByDim map[profile.Dimension]float64, suff map[profile.Dimension]bool) profile.Archetype {
	// Rank the sufficient dimensions.
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
	// Stable selection sort by score descending (deterministic on ties via Order).
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

	// Special case: heavy back-and-forth. If Iteration Efficiency is sufficient,
	// notably low, and it's the weakest axis, the working style is conversational.
	if v, ok := scoreByDim[profile.IterationEfficiency]; ok && suff[profile.IterationEfficiency] {
		if v < 5 && ranked[len(ranked)-1].d == profile.IterationEfficiency {
			return profile.Archetype{
				Name:        "Conversationalist",
				Blurb:       "You work it out in dialogue — lots of back-and-forth with the model.",
				Explanation: "Iteration Efficiency is your lowest axis, indicating frequent clarification loops or re-prompts.",
			}
		}
	}

	a := archetypeFor(top.d)
	a.Explanation = "Your strongest axis is " + top.d.Title() + "."
	if len(ranked) > 1 {
		a.Explanation += " " + ranked[1].d.Title() + " is next."
	}
	return a
}

// archetypeFor maps a dominant dimension to its persona.
func archetypeFor(d profile.Dimension) profile.Archetype {
	switch d {
	case profile.PromptQuality:
		return profile.Archetype{Name: "Architect", Blurb: "You brief the model precisely — grounded prompts with paths, code, and errors."}
	case profile.ContextManagement:
		return profile.Archetype{Name: "Curator", Blurb: "You keep context clean — deliberate resets, planning, and high reuse."}
	case profile.ToolLeverage:
		return profile.Archetype{Name: "Prober", Blurb: "You explore widely — diverse tools, sub-agents, and MCP servers."}
	case profile.IterationEfficiency:
		return profile.Archetype{Name: "Operator", Blurb: "You converge fast — few clarification loops, little rework."}
	case profile.TokenEconomy:
		return profile.Archetype{Name: "Economist", Blurb: "You're cache-savvy — stable context and cost-efficient sessions."}
	default:
		return profile.Archetype{Name: "Collaborator", Blurb: "A balanced way of working with AI."}
	}
}
