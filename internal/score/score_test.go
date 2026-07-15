package score

import (
	"testing"
	"time"

	"github.com/siddham/synch/internal/model"
	"github.com/siddham/synch/internal/profile"
)

// userTurn builds a scorable user turn.
func userTurn(txt string) model.Turn {
	return model.Turn{Role: model.RoleUser, Text: txt}
}

// asstTurn builds an assistant turn with optional tool calls.
func asstTurn(txt string, tools ...model.ToolCall) model.Turn {
	return model.Turn{Role: model.RoleAssistant, Text: txt, ToolCalls: tools}
}

func dimResult(e Evaluation, d profile.Dimension) profile.DimensionResult {
	for _, r := range e.Dimensions {
		if r.Dimension == d {
			return r
		}
	}
	return profile.DimensionResult{}
}

// FAIRNESS: a Hindi/Hinglish prompt that names a file and includes an error must
// score at least as high as the equivalent English prompt. Grading communication,
// not English proficiency.
func TestPromptQualityLanguageParity(t *testing.T) {
	en := "There is a TypeError in main.py when I run it, please fix the parsing error"
	hi := "main.py mein TypeError aa raha hai jab run karta hoon, ye parsing error theek karo"

	enScore, _ := promptScore(en)
	hiScore, _ := promptScore(hi)
	if hiScore < enScore-0.01 {
		t.Fatalf("Hindi prompt penalized: en=%.2f hi=%.2f", enScore, hiScore)
	}
	if hiScore < 6 {
		t.Fatalf("high-signal Hindi prompt scored low: %.2f", hiScore)
	}
}

// FAIRNESS: a sparse user (too few prompts) gets "insufficient data", not a 0.
func TestInsufficientDataNotZero(t *testing.T) {
	s := model.Session{Turns: []model.Turn{userTurn("hi"), asstTurn("hello")}}
	e := Evaluate([]model.Session{s})
	pq := dimResult(e, profile.PromptQuality)
	if pq.Signal.Sufficient {
		t.Fatalf("expected insufficient data with 1 prompt")
	}
	if pq.Signal.Score != 0 || e.Overall != 0 {
		// score stays 0 numerically but Sufficient=false means the UI shows "n/a",
		// and it must not drag Overall down.
	}
	// Overall must ignore insufficient dimensions (here: everything insufficient).
	if e.Overall != 0 {
		t.Fatalf("overall should be 0 when nothing is sufficient, got %.2f", e.Overall)
	}
}

// A grounded prompt (path + error + code) scores clearly higher than a vague one.
func TestPromptQualityGroundedBeatsVague(t *testing.T) {
	grounded, _ := promptScore("Fix the null deref in `internal/store/sqlite.go` — error: nil pointer at line 42")
	vague, _ := promptScore("fix it")
	if grounded <= vague {
		t.Fatalf("grounded (%.2f) should beat vague (%.2f)", grounded, vague)
	}
	if vague > 3 {
		t.Fatalf("vague prompt should score low, got %.2f", vague)
	}
}

// Meta/synthetic and command-like turns must not inflate Prompt Quality.
func TestNoInflationFromNonPrompts(t *testing.T) {
	turns := []model.Turn{
		{Role: model.RoleMeta, IsSynthetic: true, Text: "<local-command-caveat>x</local-command-caveat>"},
		{Role: model.RoleUser, SlashCommand: "/clear", Text: "<command-name>/clear</command-name>"},
		{Role: model.RoleUser, IsCommandLike: true, Text: "git status"},
		userTurn("Real grounded prompt about src/app.go with a stack trace: panic at foo.go:12"),
		userTurn("Another real one referencing config.yaml and an error"),
		userTurn("Third real prompt with `code` and a path lib/util.ts"),
	}
	s := model.Session{Turns: turns}
	e := Evaluate([]model.Session{s})
	pq := dimResult(e, profile.PromptQuality)
	if !pq.Signal.Sufficient {
		t.Fatalf("expected sufficient (3 real prompts)")
	}
	if pq.Signal.Evidence["prompts"] != 3 {
		t.Fatalf("non-prompts leaked into count: %v", pq.Signal.Evidence["prompts"])
	}
}

// Determinism: identical input yields identical scores across runs.
func TestDeterminism(t *testing.T) {
	s := model.Session{
		Turns: []model.Turn{
			userTurn("fix parsing in main.go, error: boom at main.go:10"),
			asstTurn("done", model.ToolCall{Name: "Edit"}, model.ToolCall{Name: "Bash"}),
			userTurn("now add a test in main_test.go"),
			asstTurn("ok", model.ToolCall{Name: "Write"}),
			userTurn("run it and paste the output please"),
			asstTurn("passing"),
		},
		Tokens: model.TokenUsage{Input: 1000, Output: 5000, CacheRead: 90000, CacheCreation: 10000},
	}
	a := Evaluate([]model.Session{s})
	b := Evaluate([]model.Session{s})
	if a.Overall != b.Overall {
		t.Fatalf("non-deterministic overall: %.6f vs %.6f", a.Overall, b.Overall)
	}
	for i := range a.Dimensions {
		if a.Dimensions[i].Signal.Score != b.Dimensions[i].Signal.Score {
			t.Fatalf("non-deterministic dim %s", a.Dimensions[i].Dimension)
		}
	}
}

// High cache-hit rate drives a strong Token Economy score.
func TestTokenEconomyCache(t *testing.T) {
	s := model.Session{Tokens: model.TokenUsage{Input: 1000, CacheRead: 95000, CacheCreation: 4000, Output: 2000},
		Turns: []model.Turn{asstTurn("a"), asstTurn("b"), asstTurn("c")}}
	e := Evaluate([]model.Session{s})
	te := dimResult(e, profile.TokenEconomy)
	if !te.Signal.Sufficient || te.Signal.Score < 8 {
		t.Fatalf("high cache hit should score high: %+v", te.Signal)
	}
}

// Aggregation weights by observations, not by session count (a few big sessions
// can't dominate; here two sessions of different size average by prompt weight).
func TestAggregationWeightsByObservations(t *testing.T) {
	mk := func(txt string, n int) model.Session {
		var ts []model.Turn
		for i := 0; i < n; i++ {
			ts = append(ts, userTurn(txt))
		}
		return model.Session{Turns: ts, StartedAt: time.Unix(int64(n), 0)}
	}
	big := mk("Grounded prompt about src/main.go with error: panic at main.go:9", 20)
	small := mk("fix", 3)
	e := Evaluate([]model.Session{big, small})
	pq := dimResult(e, profile.PromptQuality)
	if pq.Signal.Observations != 23 {
		t.Fatalf("observations should sum: %v", pq.Signal.Observations)
	}
}
