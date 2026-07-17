package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham/reflect/internal/profile"
)

func sampleProfile() profile.Profile {
	dim := func(d profile.Dimension, score float64, ok bool) profile.DimensionResult {
		return profile.DimensionResult{Dimension: d, Title: d.Title(),
			Signal: profile.Signal{Score: score, Sufficient: ok, Observations: 10}}
	}
	return profile.Profile{
		SchemaVersion: profile.SchemaVersion,
		GeneratedAt:   time.Unix(1_700_000_000, 0).UTC(),
		Source:        "claude-code",
		Overall:       6.97,
		Dimensions: []profile.DimensionResult{
			dim(profile.PromptQuality, 5.8, true),
			dim(profile.IterationEfficiency, 4.9, true),
			dim(profile.ToolLeverage, 7.2, true),
			dim(profile.ContextManagement, 7.7, true),
			dim(profile.TokenEconomy, 9.8, true),
		},
		Archetype: profile.Archetype{Name: "Conversationalist", Blurb: "You work it out in dialogue.", Explanation: "Iteration is your lowest axis."},
		Insights:  []profile.Insight{{Title: "Name files up front", Body: "Only 33% of prompts had a path.", Source: "heuristic"}},
		Stats:     profile.Stats{Sessions: 10, UserPrompts: 63, CacheHitRate: 0.97, TotalTokens: 348_600_000, TopTools: []profile.Count{{Name: "Bash", Count: 185}}},
	}
}

func TestRenderContainsKeyElements(t *testing.T) {
	lipgloss.SetColorProfile(0) // Ascii: strip color codes for stable assertions
	var buf bytes.Buffer
	if err := (Reporter{W: &buf}).Render(sampleProfile()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"REFLECT", "COLLABORATION RADAR", "OVERALL", "CONVERSATIONALIST",
		"Prompt Quality", "Token Economy", "TELEMETRY", "SIGNALS & TIPS",
		"Name files up front", "Bash", "7.0", // overall 6.97 rounds to 7.0
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard missing %q", want)
		}
	}
}

// Insufficient dimensions must render as "n/a", never as a low number.
func TestRenderInsufficientShowsNA(t *testing.T) {
	lipgloss.SetColorProfile(0)
	p := sampleProfile()
	p.Dimensions[0].Signal.Sufficient = false
	var buf bytes.Buffer
	if err := (Reporter{W: &buf}).Render(p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "n/a") {
		t.Errorf("insufficient dimension should show n/a")
	}
}

// The radar must not panic on zero/empty input.
func TestRadarEmpty(t *testing.T) {
	_ = radarBlock(nil, 20, 20)
	_ = radarBlock([]profile.DimensionResult{}, 20, 20)
}
