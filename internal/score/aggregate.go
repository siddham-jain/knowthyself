package score

import (
	"github.com/siddham-jain/knowthyself/internal/model"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

// Evaluation is the scoring output before insights/stats are attached.
type Evaluation struct {
	Dimensions []profile.DimensionResult
	Overall    float64
	Archetype  profile.Archetype
}

// Evaluate runs every registered scorer over all sessions and aggregates the
// per-session signals into per-dimension results.
//
// Fairness mechanics (see plan): scores are computed per session then aggregated
// weighted by observations, so a few large sessions can't dominate; dimensions with
// no sufficient session render as "insufficient data" rather than a low score;
// evidence counts are summed across sessions so every grade stays auditable; and
// the overall score is the weighted mean of only the sufficient dimensions (weights
// renormalized), so a missing signal redistributes weight instead of scoring zero.
func Evaluate(sessions []model.Session) Evaluation {
	scorers := All()
	results := make([]profile.DimensionResult, 0, len(scorers))
	scoreByDim := map[profile.Dimension]float64{}
	suffByDim := map[profile.Dimension]bool{}

	for _, sc := range scorers {
		dim := sc.Dimension()
		var wsum, osum float64
		evidence := map[string]float64{}
		anySufficient := false

		for _, s := range sessions {
			sig := sc.Score(s)
			for k, v := range sig.Evidence {
				evidence[k] += v // counts sum cleanly across sessions
			}
			if sig.Sufficient {
				anySufficient = true
				wsum += sig.Score * sig.Observations
				osum += sig.Observations
			}
		}

		agg := profile.Signal{Evidence: evidence}
		if anySufficient && osum > 0 {
			agg.Sufficient = true
			agg.Score = wsum / osum
			agg.Observations = osum
		}
		results = append(results, profile.DimensionResult{
			Dimension: dim,
			Title:     dim.Title(),
			Signal:    agg,
		})
		scoreByDim[dim] = agg.Score
		suffByDim[dim] = agg.Sufficient
	}

	overall := weightedOverall(scorers, scoreByDim, suffByDim)
	arch := deriveArchetype(scoreByDim, suffByDim)
	return Evaluation{Dimensions: results, Overall: overall, Archetype: arch}
}

// weightedOverall is the weighted mean over sufficient dimensions only, with weights
// renormalized so absent signals redistribute rather than drag the score down.
func weightedOverall(scorers []Scorer, scoreByDim map[profile.Dimension]float64, suffByDim map[profile.Dimension]bool) float64 {
	var num, den float64
	for _, sc := range scorers {
		d := sc.Dimension()
		if suffByDim[d] {
			num += scoreByDim[d] * sc.Weight()
			den += sc.Weight()
		}
	}
	if den == 0 {
		return 0
	}
	return num / den
}
