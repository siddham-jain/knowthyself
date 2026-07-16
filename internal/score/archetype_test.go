package score

import (
	"testing"

	"github.com/siddham/synch/internal/profile"
)

// allSufficient marks every dimension as gradable.
func allSufficient() map[profile.Dimension]bool {
	m := map[profile.Dimension]bool{}
	for _, d := range profile.Order {
		m[d] = true
	}
	return m
}

// A radar shape must map deterministically to the persona whose signature it most
// resembles — the whole point of a shape-derived archetype.
func TestArchetypeShapeMatching(t *testing.T) {
	cases := []struct {
		name   string
		scores map[profile.Dimension]float64
		want   string
	}{
		{
			name: "prompt+context spike → Architect",
			scores: map[profile.Dimension]float64{
				profile.PromptQuality: 9, profile.ContextManagement: 9,
				profile.IterationEfficiency: 6, profile.ToolLeverage: 4, profile.TokenEconomy: 5,
			},
			want: "Architect",
		},
		{
			name: "tool-leverage spike → Conductor",
			scores: map[profile.Dimension]float64{
				profile.ToolLeverage: 9, profile.ContextManagement: 6, profile.PromptQuality: 5,
				profile.IterationEfficiency: 4, profile.TokenEconomy: 4,
			},
			want: "Conductor",
		},
		{
			name: "tool+iteration exploration → Pathfinder",
			scores: map[profile.Dimension]float64{
				profile.ToolLeverage: 9, profile.IterationEfficiency: 8, profile.PromptQuality: 4,
				profile.ContextManagement: 3, profile.TokenEconomy: 3,
			},
			want: "Pathfinder",
		},
		{
			name: "token-economy spike → Economist",
			scores: map[profile.Dimension]float64{
				profile.TokenEconomy: 9.5, profile.ContextManagement: 6, profile.IterationEfficiency: 5,
				profile.PromptQuality: 4, profile.ToolLeverage: 3,
			},
			want: "Economist",
		},
	}
	for _, tc := range cases {
		got := deriveArchetype(tc.scores, allSufficient())
		if got.Name != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got.Name, tc.want)
		}
		if got.Explanation == "" {
			t.Errorf("%s: archetype should always explain itself", tc.name)
		}
	}
}

// A flat radar is a Generalist, not the nearest spiky persona.
func TestArchetypeBalancedIsGeneralist(t *testing.T) {
	flat := map[profile.Dimension]float64{
		profile.PromptQuality: 6.5, profile.IterationEfficiency: 6.2, profile.ToolLeverage: 6.8,
		profile.ContextManagement: 6.4, profile.TokenEconomy: 6.6,
	}
	if got := deriveArchetype(flat, allSufficient()); got.Name != "Generalist" {
		t.Fatalf("balanced radar → %q, want Generalist", got.Name)
	}
}

// Low, weakest Iteration Efficiency is the distinctive Conversationalist signal.
func TestArchetypeConversationalist(t *testing.T) {
	scores := map[profile.Dimension]float64{
		profile.PromptQuality: 7, profile.IterationEfficiency: 3.5, profile.ToolLeverage: 7,
		profile.ContextManagement: 8, profile.TokenEconomy: 9,
	}
	if got := deriveArchetype(scores, allSufficient()); got.Name != "Conversationalist" {
		t.Fatalf("got %q, want Conversationalist", got.Name)
	}
}

// No sufficient data → Newcomer, never a made-up persona.
func TestArchetypeNewcomer(t *testing.T) {
	none := map[profile.Dimension]bool{}
	if got := deriveArchetype(map[profile.Dimension]float64{}, none); got.Name != "Newcomer" {
		t.Fatalf("got %q, want Newcomer", got.Name)
	}
}

// A single graded axis falls back to that axis's canonical persona (cosine is
// degenerate with one dimension).
func TestArchetypeSingleAxis(t *testing.T) {
	suff := map[profile.Dimension]bool{profile.TokenEconomy: true}
	scores := map[profile.Dimension]float64{profile.TokenEconomy: 9}
	if got := deriveArchetype(scores, suff); got.Name != "Economist" {
		t.Fatalf("got %q, want Economist", got.Name)
	}
}
