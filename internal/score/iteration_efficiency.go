package score

import (
	"github.com/siddham/reflect/internal/model"
	"github.com/siddham/reflect/internal/profile"
	"github.com/siddham/reflect/internal/text"
)

// iterationEfficiency measures convergence: how often the assistant has to stop and
// ask for clarification, and how often the user has to correct/re-prompt. Both are
// normalized per prompt. Fewer loops => a higher score. Correction detection is
// multilingual, and its absence never penalizes (only presence lowers the score),
// so a non-English user is never disadvantaged.
type iterationEfficiency struct{}

func (iterationEfficiency) Dimension() profile.Dimension { return profile.IterationEfficiency }
func (iterationEfficiency) Weight() float64              { return 0.20 }

func (iterationEfficiency) Score(s model.Session) profile.Signal {
	v := newView(s)
	n := len(v.prompts)
	if n < minPromptsForSignal {
		return profile.Signal{Sufficient: false}
	}

	// Clarification loop: an assistant turn that ends in a question and issues no
	// tool call — i.e. it halted to ask the user rather than acting.
	clar := 0
	for _, a := range v.assistant {
		if a.EndsWithQ && len(a.ToolCalls) == 0 {
			clar++
		}
	}
	// Corrections: user prompts that read as re-prompts/dissatisfaction.
	corr := 0
	for _, p := range v.prompts {
		if text.IsCorrection(p.Text) {
			corr++
		}
	}

	clarRate := ratio(float64(clar), float64(n))
	corrRate := ratio(float64(corr), float64(n))

	// Start near the top; subtract for each kind of friction. Weighted so that a
	// clarification (a hard stall) costs a bit more than a correction.
	score := clamp(9.5-6.0*clarRate-5.0*corrRate, 0, 10)

	return profile.Signal{
		Score:        score,
		Sufficient:   true,
		Observations: float64(n),
		Evidence: map[string]float64{
			"prompts":              float64(n),
			"clarification_loops":  float64(clar),
			"correction_reprompts": float64(corr),
		},
	}
}

func init() { Register(iterationEfficiency{}) }
