package score

import (
	"github.com/siddham-jain/knowthyself/internal/model"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

// minInputTokens is the fairness floor for Token Economy: tiny sessions don't carry
// a meaningful cost signal.
const minInputTokens = 5000

// tokenEconomy grades cost-efficiency, dominated by cache-hit rate — the single
// biggest lever a developer controls (stable, reused context reads from cache
// instead of re-billing full input). A secondary factor rewards reading cache
// rather than constantly rewriting it.
type tokenEconomy struct{}

func (tokenEconomy) Dimension() profile.Dimension { return profile.TokenEconomy }
func (tokenEconomy) Weight() float64              { return 0.20 }

func (tokenEconomy) Score(s model.Session) profile.Signal {
	totalInput := s.Tokens.TotalInput()
	if totalInput < minInputTokens {
		return profile.Signal{Sufficient: false}
	}

	hitRate := ratio(float64(s.Tokens.CacheRead), float64(totalInput))
	// Of the cached tokens, what share were reused (read) vs freshly written?
	cacheEff := ratio(float64(s.Tokens.CacheRead),
		float64(s.Tokens.CacheRead+s.Tokens.CacheCreation))

	score := clamp(1.5+hitRate*7.5+cacheEff*1.0, 0, 10)

	return profile.Signal{
		Score:        score,
		Sufficient:   true,
		Observations: float64(totalInput),
		Evidence: map[string]float64{
			"total_input_tokens":  float64(totalInput),
			"output_tokens":       float64(s.Tokens.Output),
			"cache_read_tokens":   float64(s.Tokens.CacheRead),
			"cache_create_tokens": float64(s.Tokens.CacheCreation),
		},
	}
}

func init() { Register(tokenEconomy{}) }
