package insight

import "os"

// NewDeepEval returns an LLM-backed Engine when an API key is configured, else nil.
// Deep-eval is strictly opt-in and, by contract, may only *phrase* qualitative tips
// from the already-computed Profile — it never recomputes or changes a score, so the
// radar stays deterministic and private by default.
//
// Wiring for the actual Anthropic call is added in a follow-up; returning nil here
// makes --deep-eval a safe no-op (the caller falls back to the Heuristic engine)
// until a key + client are present.
func NewDeepEval() Engine {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return nil
	}
	return nil
}
