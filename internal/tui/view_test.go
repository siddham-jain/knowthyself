package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func maxLineWidth(s string) int {
	max := 0
	for _, ln := range strings.Split(s, "\n") {
		if w := len([]rune(ansiRe.ReplaceAllString(ln, ""))); w > max {
			max = w
		}
	}
	return max
}

func TestEaseOutCubic(t *testing.T) {
	if easeOutCubic(0) != 0 || easeOutCubic(1) != 1 {
		t.Fatalf("endpoints wrong: %v %v", easeOutCubic(0), easeOutCubic(1))
	}
	// Monotonic non-decreasing.
	prev := -1.0
	for i := 0; i <= 100; i++ {
		v := easeOutCubic(float64(i) / 100)
		if v < prev {
			t.Fatalf("not monotonic at %d: %v < %v", i, v, prev)
		}
		prev = v
	}
	// Decelerating: front-loaded, so past the halfway mark early.
	if easeOutCubic(0.5) <= 0.5 {
		t.Fatalf("expected ease-out to be front-loaded, got %v", easeOutCubic(0.5))
	}
}

// Reflow: across a width sweep, every mode renders without panic and no content
// line exceeds the terminal width.
func TestResponsiveReflow(t *testing.T) {
	lipgloss.SetColorProfile(0)
	p := profileWithSessions()
	withRead := profileWithSessions()
	withRead.DeepRead = sampleDeepRead()
	for _, w := range []int{40, 46, 47, 60, 73, 74, 100, 220} {
		for _, prof := range []profile.Profile{p, withRead} {
			m := settled(prof)
			m.updateNotice = "9.9.9" // the footer nudge must not push a line over either
			m.w, m.h = w, 44
			for mode := viewOverview; mode < viewCount; mode++ {
				m.mode = mode
				out := m.View()
				if got := maxLineWidth(out); got > w {
					t.Errorf("w=%d mode=%d overflow: max line %d > %d", w, mode, got, w)
				}
			}
		}
	}
}

func sampleDeepRead() *profile.DeepRead {
	return &profile.DeepRead{
		Model:     "claude-sonnet-5",
		Endpoint:  "api.anthropic.com",
		RubricVer: 1,
		Sample:    profile.SampleInfo{Prompts: 60, Sessions: 12, Available: 840},
		Criteria: []profile.CriterionResult{
			{Key: "goal_clarity", Title: "Goal clarity", Mean: 2.4, Max: 4, Judged: 58},
			{Key: "context_sufficiency", Title: "Context sufficiency", Mean: 3.1, Max: 4, Judged: 60},
			{Key: "constraints", Title: "Constraints & acceptance", Mean: 1.2, Max: 4, Judged: 57},
			{Key: "scope_discipline", Title: "Scope discipline", Mean: 3.6, Max: 4, Judged: 60},
			{Key: "correction_quality", Title: "Correction quality", Mean: 0, Max: 4, Judged: 0},
		},
		Findings: []profile.Insight{
			{Title: "State the done-condition", Body: "You rarely say what finished looks like, so the model guesses at scope.", Source: "deep-eval"},
		},
		Confidence: profile.ConfidenceHigh,
	}
}

// A profile with no deep read must render the tab as an invitation, never a crash or
// a blank panel.
func TestDeepReadEmptyState(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m := settled(profileWithSessions())
	m.mode = viewDeepRead
	if out := m.View(); !strings.Contains(out, "--deep-eval") {
		t.Error("empty deep read view should point at the flag that fills it")
	}
}

// The deep read is model-judged on a different scale; the panel must say so, or it
// reads as another deterministic score.
func TestDeepReadLabelsItsProvenance(t *testing.T) {
	lipgloss.SetColorProfile(0)
	p := profileWithSessions()
	p.DeepRead = sampleDeepRead()
	m := settled(p)
	m.mode = viewDeepRead
	out := m.View()
	for _, want := range []string{"claude-sonnet-5", "60 prompts", "high", "/4"} {
		if !strings.Contains(out, want) {
			t.Errorf("deep read view missing %q", want)
		}
	}
}

// The first-run reveal must also reflow cleanly across the width sweep.
func TestRevealReflow(t *testing.T) {
	lipgloss.SetColorProfile(0)
	p := profileWithSessions()
	p.Archetype.Traits = []string{"Polyglot", "Night Owl"}
	for _, w := range []int{46, 47, 60, 73, 74, 100, 220} {
		m := settled(p)
		m.atReveal = true
		m.w, m.h = w, 44
		out := m.View()
		if got := maxLineWidth(out); got > w {
			t.Errorf("w=%d reveal overflow: max line %d > %d", w, got, w)
		}
		if !strings.Contains(out, "YOU ARE A") {
			t.Errorf("w=%d reveal missing headline", w)
		}
	}
}

// Below the minimum width, show a clear message rather than a broken layout.
func TestTooNarrowMessage(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m := settled(profileWithSessions())
	m.w, m.h = 30, 20
	if !strings.Contains(m.View(), "too narrow") {
		t.Fatalf("expected too-narrow message")
	}
}

// Narrow terminals stack panels and use the short footer. Given enough vertical room
// (stacked panels are tall), every panel still shows.
func TestNarrowStacks(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m := settled(profileWithSessions())
	m.w, m.h = 60, 80
	out := m.View()
	for _, want := range []string{"COLLABORATION RADAR", "OVERALL", "DIMENSIONS", "TELEMETRY", "INSPECT"} {
		if !strings.Contains(out, want) {
			t.Errorf("narrow overview missing %q", want)
		}
	}
	if !strings.Contains(out, "q quit") {
		t.Errorf("footer missing short keys")
	}
}

// The core responsiveness fix: no view may ever render more lines than the terminal
// height (overflow is what corrupted the alt-screen scrollback until a resize).
func TestNeverOverflowsHeight(t *testing.T) {
	lipgloss.SetColorProfile(0)
	p := profileWithSessions()
	p.DeepRead = sampleDeepRead()
	for _, sz := range []struct{ w, h int }{
		{60, 20}, {80, 24}, {100, 30}, {74, 16}, {120, 40}, {46, 14},
	} {
		m := settled(p)
		m.w, m.h = sz.w, sz.h
		for mode := viewOverview; mode < viewCount; mode++ {
			m.mode = mode
			if got := strings.Count(m.View(), "\n") + 1; got > sz.h {
				t.Errorf("w=%d h=%d mode=%d overflow: %d lines > %d", sz.w, sz.h, mode, got, sz.h)
			}
		}
		// The reveal too.
		m.atReveal = true
		if got := strings.Count(m.View(), "\n") + 1; got > sz.h {
			t.Errorf("w=%d h=%d reveal overflow: %d lines > %d", sz.w, sz.h, got, sz.h)
		}
	}
}

// Below the minimum height, show a clear message rather than a broken frame.
func TestTooShortMessage(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m := settled(profileWithSessions())
	m.w, m.h = 80, 8
	if !strings.Contains(m.View(), "too short") {
		t.Fatalf("expected too-short message")
	}
}

// A window resize is reflected without a redraw crash.
func TestResizeUpdatesWidth(t *testing.T) {
	m := settled(profileWithSessions())
	m = advance(m, tea.WindowSizeMsg{Width: 64, Height: 30})
	if m.layout().w != clampInt(64-2, minWidth, maxWidth) {
		t.Fatalf("layout width not updated: %d", m.layout().w)
	}
	if !m.layout().twoCol && 64-2 >= twoColMin {
		t.Fatalf("twoCol flag wrong")
	}
}
