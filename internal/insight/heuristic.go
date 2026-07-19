package insight

import (
	"context"
	"fmt"
	"sort"

	"github.com/siddham-jain/knowthyself/internal/profile"
)

// Heuristic is the default, deterministic Engine. It turns the retained evidence
// counts on each dimension into concrete, quantified tips ("you named a file in 36%
// of prompts") and orders them weakest-dimension-first. No network, no model.
type Heuristic struct {
	// Max caps the number of tips returned (0 => default of 4).
	Max int
}

func (Heuristic) Name() string { return "heuristic" }

func (h Heuristic) Generate(_ context.Context, p profile.Profile) ([]profile.Insight, error) {
	max := h.Max
	if max <= 0 {
		max = 4
	}

	// Consider sufficient dimensions, weakest first — that's where advice pays off.
	type dr struct {
		res   profile.DimensionResult
		score float64
	}
	var ranked []dr
	for _, d := range p.Dimensions {
		if d.Signal.Sufficient {
			ranked = append(ranked, dr{d, d.Signal.Score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score < ranked[j].score })

	var out []profile.Insight
	for _, r := range ranked {
		if tip, ok := tipFor(r.res); ok {
			out = append(out, tip)
		}
		if len(out) >= max {
			break
		}
	}
	return out, nil
}

// tipFor produces a single quantified tip for a dimension from its evidence.
func tipFor(d profile.DimensionResult) (profile.Insight, bool) {
	e := d.Signal.Evidence
	pct := func(num, den string) float64 {
		if e[den] == 0 {
			return 0
		}
		return 100 * e[num] / e[den]
	}
	mk := func(title, body string) (profile.Insight, bool) {
		return profile.Insight{Dimension: d.Dimension, Title: title, Body: body, Source: "heuristic"}, true
	}

	switch d.Dimension {
	case profile.PromptQuality:
		p := pct("prompts_with_path", "prompts")
		if p < 60 {
			return mk("Name files up front",
				fmt.Sprintf("You included a file path in only %.0f%% of prompts. Leading with the file the model should touch cuts exploratory turns.", p))
		}
		if e["prompts_vague"] > 0 {
			return mk("Trim the vague openers",
				fmt.Sprintf("%.0f prompts were too terse to act on. A one-line 'what' + 'where' beats 'fix it'.", e["prompts_vague"]))
		}
	case profile.IterationEfficiency:
		if e["clarification_loops"] > 0 {
			return mk("Pre-empt the clarifying question",
				fmt.Sprintf("The model stopped to ask you %.0f times. Front-loading the missing detail avoids the stall.", e["clarification_loops"]))
		}
		if e["correction_reprompts"] > 0 {
			return mk("Fewer course-corrections",
				fmt.Sprintf("%.0f prompts were corrections. A sharper first ask converges faster.", e["correction_reprompts"]))
		}
	case profile.ToolLeverage:
		if e["subagent_calls"] == 0 {
			return mk("Delegate with sub-agents",
				"You haven't used sub-agents. For broad searches or parallel work, delegating keeps your main context clean.")
		}
		if e["skill_calls"] == 0 && e["mcp_calls"] == 0 {
			return mk("Automate with skills & MCP",
				"You lean on the built-in tools. Skills and MCP servers can turn your repetitive asks into one-shot commands.")
		}
		return mk("Keep leaning on delegation",
			fmt.Sprintf("Strong tool use — %.0f sub-agent calls and %.0f MCP calls. Push more repetitive work into skills.", e["subagent_calls"], e["mcp_calls"]))
	case profile.ContextManagement:
		if e["compactions"] > 0 {
			return mk("Reset before you overflow",
				fmt.Sprintf("Context was auto-compacted %.0f times. A deliberate /clear at task boundaries keeps sessions sharp.", e["compactions"]))
		}
		if e["clears"] == 0 {
			return mk("Use /clear at task boundaries",
				"You never reset context. Starting a fresh task in a clean session improves focus and cache efficiency.")
		}
	case profile.TokenEconomy:
		return mk("Protect your cache",
			"Avoid editing far up in a long session — it invalidates cached context and re-bills the input. Append, don't rewrite.")
	}
	return profile.Insight{}, false
}
