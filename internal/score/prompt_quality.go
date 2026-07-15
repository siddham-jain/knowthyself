package score

import (
	"github.com/siddham/synch/internal/model"
	"github.com/siddham/synch/internal/profile"
	"github.com/siddham/synch/internal/text"
)

// minPromptsForSignal is the fairness floor: below it, Prompt Quality and Iteration
// Efficiency report "insufficient data" rather than a possibly-unfair low score.
const minPromptsForSignal = 3

// promptQuality grades how much actionable, language-independent context the user
// gives: file paths, code, errors, URLs, and appropriate specificity. It rewards
// grounding but never requires all types (a clear feature request needs no stack
// trace), and it is script-aware so non-English prompts are graded fairly.
type promptQuality struct{}

func (promptQuality) Dimension() profile.Dimension { return profile.PromptQuality }
func (promptQuality) Weight() float64              { return 0.25 }

func (promptQuality) Score(s model.Session) profile.Signal {
	v := newView(s)
	n := len(v.prompts)
	if n < minPromptsForSignal {
		return profile.Signal{Sufficient: false}
	}

	var withPath, withError, withCode, withURL, vague int
	var sum float64
	for _, p := range v.prompts {
		q, feats := promptScore(p.Text)
		sum += q
		if feats.path {
			withPath++
		}
		if feats.err {
			withError++
		}
		if feats.code {
			withCode++
		}
		if feats.url {
			withURL++
		}
		if feats.vague {
			vague++
		}
	}

	return profile.Signal{
		Score:        sum / float64(n),
		Sufficient:   true,
		Observations: float64(n),
		Evidence: map[string]float64{
			"prompts":            float64(n),
			"prompts_with_path":  float64(withPath),
			"prompts_with_error": float64(withError),
			"prompts_with_code":  float64(withCode),
			"prompts_with_url":   float64(withURL),
			"prompts_vague":      float64(vague),
		},
	}
}

type promptFeats struct{ path, err, code, url, vague bool }

// promptScore rates a single prompt 0..10 from structural signals only.
func promptScore(txt string) (float64, promptFeats) {
	f := promptFeats{
		path: text.HasFilePath(txt),
		err:  text.HasError(txt),
		code: text.HasCode(txt),
		url:  text.HasURL(txt),
	}

	score := 3.5 // baseline for a real, non-vague prompt

	grounding := 0.0
	if f.path {
		grounding += 2.0
	}
	if f.code {
		grounding += 1.5
	}
	if f.url {
		grounding += 1.0
	}
	score += clamp(grounding, 0, 3.5)

	if f.err {
		score += 1.5 // pasting the actual error is high-signal
	}

	switch text.LengthBand(txt) {
	case "vague":
		f.vague = true
		score -= 1.5
	case "short":
		score += 0.3
	case "substantial":
		score += 2.0
	case "long":
		score += 1.0 // useful, but wall-of-text has diminishing returns
	}

	return clamp(score, 0, 10), f
}

func init() { Register(promptQuality{}) }
