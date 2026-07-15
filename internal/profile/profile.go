// Package profile defines the pure, versioned, serializable Profile — the single
// contract between computation (parsing + scoring) and presentation (TUI, JSON,
// and future consumers: a web landing page, a recruiter grading portal).
//
// Nothing here may depend on how a Profile is rendered. Keep field names stable and
// bump SchemaVersion on breaking changes so external consumers can migrate.
package profile

import "time"

// SchemaVersion is the version of the Profile JSON contract. External consumers
// (recruiter portal, landing page) should check this before deserializing.
const SchemaVersion = 1

// Dimension is one axis of the collaboration profile.
type Dimension string

const (
	PromptQuality       Dimension = "prompt_quality"
	IterationEfficiency Dimension = "iteration_efficiency"
	ToolLeverage        Dimension = "tool_leverage"
	ContextManagement   Dimension = "context_management"
	TokenEconomy        Dimension = "token_economy"
)

// Order is the canonical display/aggregation order of the five dimensions (also the
// order of radar-chart axes). Kept explicit so output is deterministic.
var Order = []Dimension{
	PromptQuality,
	IterationEfficiency,
	ToolLeverage,
	ContextManagement,
	TokenEconomy,
}

// Title returns the human-readable label for a dimension.
func (d Dimension) Title() string {
	switch d {
	case PromptQuality:
		return "Prompt Quality"
	case IterationEfficiency:
		return "Iteration Efficiency"
	case ToolLeverage:
		return "Tool Leverage"
	case ContextManagement:
		return "Context Management"
	case TokenEconomy:
		return "Token Economy"
	default:
		return string(d)
	}
}

// Signal is the output of a Scorer for one session (or aggregated): a 0..10 score
// plus the raw counts that produced it. Retaining the evidence is what makes every
// grade explainable and auditable.
type Signal struct {
	// Score is 0..10. Only meaningful when Sufficient is true.
	Score float64 `json:"score"`
	// Sufficient is false when there was too little data to grade this dimension
	// (renders as "insufficient data", never as a low score — fairness gate).
	Sufficient bool `json:"sufficient"`
	// Weight of observations behind this signal (e.g. number of scorable prompts).
	// Used to weight per-session signals when aggregating across sessions.
	Observations float64 `json:"observations"`
	// Evidence is the retained raw counts/rates, keyed by a short stable name, e.g.
	// {"first_prompt_with_path_pct": 36, "prompts_with_error": 12}.
	Evidence map[string]float64 `json:"evidence,omitempty"`
	// Notes are short, human-readable explanations derived from the evidence.
	Notes []string `json:"notes,omitempty"`
}

// DimensionResult is an aggregated Signal for one dimension across all sessions.
type DimensionResult struct {
	Dimension Dimension `json:"dimension"`
	Title     string    `json:"title"`
	Signal    Signal    `json:"signal"`
}

// Archetype is the collaboration persona derived from the shape of the radar.
type Archetype struct {
	Name        string `json:"name"`        // "Architect", "Prober", ...
	Blurb       string `json:"blurb"`       // one-line description
	Explanation string `json:"explanation"` // why this archetype was chosen (dimension shape)
}

// Insight is one actionable tip. Deterministic template tips by default; optional
// LLM-authored prose via --deep-eval (which may only phrase, never re-score).
type Insight struct {
	Dimension Dimension `json:"dimension,omitempty"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Source    string    `json:"source"` // "heuristic" or "deep-eval"
}

// Stats are the fun, headline numbers shown alongside the grade.
type Stats struct {
	Sessions          int            `json:"sessions"`
	Turns             int            `json:"turns"`
	UserPrompts       int            `json:"user_prompts"`
	TopTools          []Count        `json:"top_tools"`
	TopSlashCommands  []Count        `json:"top_slash_commands"`
	CacheHitRate      float64        `json:"cache_hit_rate"`      // 0..1
	TotalTokens       int64          `json:"total_tokens"`
	PermissionModeMix map[string]int `json:"permission_mode_mix"` // e.g. {"auto":195,"plan":23}
}

// Count is a name/count pair (ranked lists), kept ordered for deterministic output.
type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Profile is the complete, serializable result of an analysis run.
type Profile struct {
	SchemaVersion int               `json:"schema_version"`
	GeneratedAt   time.Time         `json:"generated_at"`
	Source        string            `json:"source"` // "claude-code"
	Overall       float64           `json:"overall"` // weighted mean of sufficient dimensions, 0..10
	Dimensions    []DimensionResult `json:"dimensions"`
	Archetype     Archetype         `json:"archetype"`
	Insights      []Insight         `json:"insights"`
	Stats         Stats             `json:"stats"`
}
