package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	for _, w := range []int{40, 46, 47, 60, 73, 74, 100, 220} {
		m := settled(p)
		m.w, m.h = w, 44
		for _, mode := range []viewMode{viewOverview, viewSessions, viewTrends} {
			m.mode = mode
			out := m.View()
			if got := maxLineWidth(out); got > w {
				t.Errorf("w=%d mode=%d overflow: max line %d > %d", w, mode, got, w)
			}
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
	for _, sz := range []struct{ w, h int }{
		{60, 20}, {80, 24}, {100, 30}, {74, 16}, {120, 40}, {46, 14},
	} {
		m := settled(p)
		m.w, m.h = sz.w, sz.h
		for _, mode := range []viewMode{viewOverview, viewSessions, viewTrends} {
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
