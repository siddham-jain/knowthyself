package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

func profileWithSessions() profile.Profile {
	p := sampleProfile()
	base := time.Unix(1_700_000_000, 0).UTC()
	for i := 0; i < 6; i++ {
		p.Sessions = append(p.Sessions, profile.SessionSummary{
			ID:        "s" + string(rune('0'+i)),
			Label:     "proj · 07-1" + string(rune('0'+i)),
			StartedAt: base.Add(time.Duration(i) * time.Hour),
			Overall:   5.0 + float64(i)*0.5,
			Dimensions: map[profile.Dimension]float64{
				profile.PromptQuality:     5 + float64(i)*0.3,
				profile.ToolLeverage:      7,
				profile.TokenEconomy:      9,
				profile.ContextManagement: 6.5,
			},
			Prompts: 5 + i,
			Tokens:  int64(i+1) * 1_000_000,
		})
	}
	return p
}

// advance returns the model after applying a message (typed back from tea.Model).
func advance(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}

// settled returns a model past the boot animation AND past the first-run reveal,
// i.e. on the graded dashboard — the state most view tests exercise.
func settled(p profile.Profile) model {
	m := newModel(p)
	m = advance(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m.booting = false
	m.progress = 1
	m.atReveal = false
	return m
}

func TestRevealThenDashboard(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m := newModel(profileWithSessions())
	m = advance(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	// Skip the boot animation → land on the reveal.
	m = advance(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !m.atReveal {
		t.Fatalf("expected to land on the reveal after boot")
	}
	out := m.View()
	for _, want := range []string{"YOU ARE A", "full breakdown"} {
		if !strings.Contains(out, want) {
			t.Errorf("reveal missing %q", want)
		}
	}
	// Any key opens the dashboard.
	m = advance(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if m.atReveal {
		t.Fatalf("a key should dismiss the reveal")
	}
	if !strings.Contains(m.View(), "INSPECT") {
		t.Fatalf("expected the overview dashboard after the reveal")
	}
}

func TestBootViewRenders(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m := newModel(profileWithSessions())
	m = advance(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	out := m.View()
	if !strings.Contains(out, "INITIALIZING") {
		t.Fatalf("boot view missing initializing banner")
	}
	// A boot tick advances progress and requests another frame.
	next, cmd := m.Update(tickMsg(time.Now()))
	if next.(model).progress <= 0 {
		t.Fatalf("tick did not advance progress")
	}
	if cmd == nil {
		t.Fatalf("boot tick should schedule another frame")
	}
}

func TestOverviewInspectorAndNav(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m := settled(profileWithSessions())
	out := m.View()
	for _, want := range []string{"INSPECT", "OVERVIEW", "with file path", "↑↓ select"} {
		if !strings.Contains(out, want) {
			t.Errorf("overview missing %q", want)
		}
	}
	// Down arrow moves the dimension cursor.
	m = advance(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.dimCursor != 1 {
		t.Fatalf("cursor did not move: %d", m.dimCursor)
	}
}

func TestTabCyclesViews(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m := settled(profileWithSessions())
	m = advance(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.mode != viewSessions {
		t.Fatalf("tab did not switch to sessions")
	}
	if !strings.Contains(m.View(), "SESSION") {
		t.Fatalf("sessions view not rendered")
	}
	m = advance(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.mode != viewTrends || !strings.Contains(m.View(), "JOURNEY") {
		t.Fatalf("trends view not rendered")
	}
	// Wrap back to overview.
	m = advance(m, tea.KeyMsg{Type: tea.KeyTab})
	if m.mode != viewOverview {
		t.Fatalf("tab did not wrap to overview")
	}
}

func TestSessionsDrillDown(t *testing.T) {
	lipgloss.SetColorProfile(0)
	p := profileWithSessions()
	m := settled(p)
	m.mode = viewSessions
	// The list opens focused on the newest session (last, since it's chronological),
	// so the work you just did is what you see first.
	if m.sessCursor != len(p.Sessions)-1 {
		t.Fatalf("session cursor = %d, want newest (%d)", m.sessCursor, len(p.Sessions)-1)
	}
	m = advance(m, tea.KeyMsg{Type: tea.KeyUp})
	m = advance(m, tea.KeyMsg{Type: tea.KeyUp})
	if m.sessCursor != len(p.Sessions)-3 {
		t.Fatalf("session cursor = %d after two ups", m.sessCursor)
	}
	if !strings.Contains(m.View(), "SESSION ·") {
		t.Fatalf("session detail not shown")
	}
}

func TestAnyKeySkipsBootAndQuit(t *testing.T) {
	m := newModel(profileWithSessions())
	m = advance(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	// A non-quit key during boot skips the animation.
	m = advance(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.booting {
		t.Fatalf("key did not skip boot")
	}
	// q quits.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("q should quit")
	}
}
