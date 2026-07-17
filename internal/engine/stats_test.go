package engine

import (
	"testing"
	"time"

	"github.com/siddham/reflect/internal/model"
)

func TestComputeStatsAndTraits(t *testing.T) {
	night := func(day, min int) time.Time {
		return time.Date(2025, time.January, day, 23, min, 0, 0, time.UTC)
	}
	s1 := model.Session{
		ID: "s1", Source: "claude-code", Cwd: "/home/u/proj-a",
		StartedAt: night(1, 0), EndedAt: night(1, 30),
		Turns: []model.Turn{
			{Role: model.RoleUser, Text: "please fix the parsing bug in main.go", Timestamp: night(1, 1)},
			{Role: model.RoleAssistant, Text: "done", Timestamp: night(1, 2)},
		},
		Tokens: model.TokenUsage{Input: 1000, Output: 500, CacheRead: 9000},
	}
	s2 := model.Session{
		ID: "s2", Source: "claude-code", Cwd: "/home/u/proj-b",
		StartedAt: time.Date(2025, time.February, 1, 23, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2025, time.February, 2, 0, 30, 0, 0, time.UTC), // 90 min
		Turns: []model.Turn{
			{Role: model.RoleUser, Text: "कृपया यह त्रुटि ठीक करें", Timestamp: time.Date(2025, time.February, 1, 23, 5, 0, 0, time.UTC)},
			{Role: model.RoleAssistant, Text: "ठीक है", Timestamp: time.Date(2025, time.February, 1, 23, 6, 0, 0, time.UTC)},
		},
		Tokens: model.TokenUsage{Input: 2000, Output: 800, CacheRead: 6000},
	}

	st := computeStats([]model.Session{s1, s2})

	if st.Projects != 2 {
		t.Errorf("projects = %d, want 2", st.Projects)
	}
	if st.CollabSeconds != int64((30+90)*60) {
		t.Errorf("collab seconds = %d, want %d", st.CollabSeconds, (30+90)*60)
	}
	if st.PeakHour != 23 {
		t.Errorf("peak hour = %d, want 23", st.PeakHour)
	}
	if st.FirstSessionAt != s1.StartedAt {
		t.Errorf("first session = %v, want %v", st.FirstSessionAt, s1.StartedAt)
	}
	if len(st.Languages) != 2 || st.Languages[0] == "" {
		t.Fatalf("languages = %v, want English + Hindi", st.Languages)
	}
	// English (2 latin prompt? only 1) vs Hindi (1) — both present.
	hasEN, hasHI := false, false
	for _, l := range st.Languages {
		hasEN = hasEN || l == "English"
		hasHI = hasHI || l == "Hindi"
	}
	if !hasEN || !hasHI {
		t.Errorf("languages = %v, want both English and Hindi", st.Languages)
	}

	// now well past 90 days after the first session → Veteran; 2 languages → Polyglot;
	// 60-min average session → Deep Diver; 23:00 peak → Night Owl.
	now := time.Date(2025, time.August, 1, 12, 0, 0, 0, time.UTC)
	traits := deriveTraits(st, now)
	want := map[string]bool{"Polyglot": true, "Veteran": true, "Deep Diver": true, "Night Owl": true}
	for _, tr := range traits {
		delete(want, tr)
	}
	if len(want) != 0 {
		t.Errorf("traits = %v, missing %v", traits, want)
	}
}

// A brand-new, single-language, short-session user earns no badges — traits are a
// flourish, never noise.
func TestDeriveTraitsSparse(t *testing.T) {
	s := model.Session{
		ID: "s", Cwd: "/p", StartedAt: time.Date(2025, time.July, 1, 10, 0, 0, 0, time.UTC),
		EndedAt: time.Date(2025, time.July, 1, 10, 5, 0, 0, time.UTC),
		Turns: []model.Turn{
			{Role: model.RoleUser, Text: "add a readme", Timestamp: time.Date(2025, time.July, 1, 10, 1, 0, 0, time.UTC)},
		},
	}
	st := computeStats([]model.Session{s})
	now := time.Date(2025, time.July, 2, 12, 0, 0, 0, time.UTC)
	if tr := deriveTraits(st, now); len(tr) != 0 {
		t.Fatalf("sparse user should have no traits, got %v", tr)
	}
}
