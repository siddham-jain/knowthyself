package score

import "github.com/siddham-jain/knowthyself/internal/model"

// view holds the per-session slices the scorers need, computed once.
type view struct {
	prompts   []model.Turn // scorable human prompts
	assistant []model.Turn // assistant turns
	all       []model.Turn // every turn (for adjacency / clarification loops)
}

func newView(s model.Session) view {
	v := view{all: s.Turns}
	for _, t := range s.Turns {
		switch {
		case t.Scorable():
			v.prompts = append(v.prompts, t)
		case t.Role == model.RoleAssistant:
			v.assistant = append(v.assistant, t)
		}
	}
	return v
}

// clamp confines x to [lo, hi].
func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// ratio returns num/den, or 0 when den == 0 (avoids divide-by-zero on sparse data).
func ratio(num, den float64) float64 {
	if den == 0 {
		return 0
	}
	return num / den
}
