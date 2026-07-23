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
	Name        string   `json:"name"`             // "Architect", "Conductor", ...
	Blurb       string   `json:"blurb"`            // one-line identity description
	Explanation string   `json:"explanation"`      // why this archetype was chosen (dimension shape)
	Traits      []string `json:"traits,omitempty"` // deterministic badges: "Polyglot", "Night Owl", ...
}

// Insight is one actionable tip. Deterministic template tips by default; optional
// LLM-authored prose via --deep-eval (which may only phrase, never re-score).
type Insight struct {
	Dimension Dimension `json:"dimension,omitempty"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Source    string    `json:"source"` // "heuristic" or "deep-eval"
}

// Stats are the fun, headline numbers shown alongside the grade — the "by the
// numbers" material behind the first-run reveal.
type Stats struct {
	Sessions          int            `json:"sessions"`
	Turns             int            `json:"turns"`
	UserPrompts       int            `json:"user_prompts"`
	TopTools          []Count        `json:"top_tools"`
	TopSlashCommands  []Count        `json:"top_slash_commands"`
	CacheHitRate      float64        `json:"cache_hit_rate"` // 0..1
	TotalTokens       int64          `json:"total_tokens"`
	PermissionModeMix map[string]int `json:"permission_mode_mix"` // e.g. {"auto":195,"plan":23}

	// Reveal "by the numbers" — awe stats, all derived from logs already on disk.
	CollabSeconds  int64     `json:"collab_seconds"`      // summed per-session span (EndedAt-StartedAt)
	FirstSessionAt time.Time `json:"first_session_at"`    // earliest session start (tenure)
	PeakHour       int       `json:"peak_hour"`           // 0..23 local hour of most activity; -1 if unknown
	Languages      []string  `json:"languages,omitempty"` // e.g. ["English","Hindi"], most-used first
	Projects       int       `json:"projects"`            // distinct project working directories
}

// Count is a name/count pair (ranked lists), kept ordered for deterministic output.
type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SessionSummary is a per-session score line, ordered chronologically. It powers
// the drill-down and the trend/journey view, and gives external consumers a
// timeline without re-scoring.
type SessionSummary struct {
	ID         string                `json:"id"`
	Label      string                `json:"label"` // human label, e.g. "medkit-app · 07-14"
	StartedAt  time.Time             `json:"started_at"`
	Overall    float64               `json:"overall"`
	Dimensions map[Dimension]float64 `json:"dimensions"` // only sufficient dimensions
	Prompts    int                   `json:"prompts"`
	Tokens     int64                 `json:"tokens"`
}

// Profile is the complete, serializable result of an analysis run.
type Profile struct {
	SchemaVersion int               `json:"schema_version"`
	GeneratedAt   time.Time         `json:"generated_at"`
	Source        string            `json:"source"`  // "claude-code"
	Overall       float64           `json:"overall"` // weighted mean of sufficient dimensions, 0..10
	Dimensions    []DimensionResult `json:"dimensions"`
	Archetype     Archetype         `json:"archetype"`
	Insights      []Insight         `json:"insights"`
	Stats         Stats             `json:"stats"`
	Sessions      []SessionSummary  `json:"sessions,omitempty"` // chronological, for drill-down & trends

	// DeepRead is the optional, model-judged qualitative read (--deep-eval). Nil
	// unless the user opted in for this run. Additive: consumers that predate it
	// ignore the field, so its presence is not a schema break.
	DeepRead *DeepRead `json:"deep_read,omitempty"`
}

// Confidence levels for a DeepRead, set from how much of the sample survived
// validation.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// DeepRead is a model's reading of the actual prompt text, on criteria the
// deterministic scorers are blind to. It is never merged into Overall and never
// changes a Signal — presentation must keep it visibly separate.
type DeepRead struct {
	Model      string            `json:"model"`
	Endpoint   string            `json:"endpoint"` // host only, never credentials
	RubricVer  int               `json:"rubric_version"`
	JudgedAt   time.Time         `json:"judged_at"`
	Sample     SampleInfo        `json:"sample"`
	Criteria   []CriterionResult `json:"criteria"`
	Findings   []Insight         `json:"findings,omitempty"`
	Confidence string            `json:"confidence"`
}

// SampleInfo records what the read was actually based on, so a small sample is
// visible rather than hidden.
type SampleInfo struct {
	Prompts   int `json:"prompts"`   // prompts judged
	Sessions  int `json:"sessions"`  // sessions they came from
	Available int `json:"available"` // scorable prompts the sample was drawn from
}

// CriterionResult is one rubric criterion aggregated over the sample. Mean is on the
// rubric's own 0..Max anchored scale, never rescaled into the 0..10 radar space.
type CriterionResult struct {
	Key     string  `json:"key"`
	Title   string  `json:"title"`
	Mean    float64 `json:"mean"`
	Max     int     `json:"max"`
	Judged  int     `json:"judged"` // judgments that applied to this criterion
	Summary string  `json:"summary,omitempty"`
}
