package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/insight/deepeval"
)

func seedStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	var s deepeval.Store
	s.Add("groq", deepeval.Provider{BaseURL: "https://api.groq.com/openai/v1", Model: "llama-3.3-70b-versatile", Dialect: deepeval.DialectOpenAI, KeyEnv: "GROQ_API_KEY"})
	s.Add("local", deepeval.Provider{BaseURL: "http://localhost:11434/v1", Model: "llama3.1", Dialect: deepeval.DialectOpenAI})
	s.Active = "local"
	if err := deepeval.SaveStore(dir, s); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The settings screen must never render wider than the terminal, at any width the
// dashboard itself supports.
func TestSettingsReflow(t *testing.T) {
	lipgloss.SetColorProfile(0)
	dir := seedStore(t)
	for _, w := range []int{46, 47, 60, 74, 100, 220} {
		m, err := newSettingsModel(w, dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, state := range []func(*settingsModel){
			func(*settingsModel) {},
			func(m *settingsModel) { m.testing = true },
			func(m *settingsModel) { m.testing = false; m.status = "local reachable · 412ms" },
			func(m *settingsModel) { m.status = ""; m.problem = "api.groq.com rejected the API key (HTTP 401)" },
		} {
			state(&m)
			if got := maxLineWidth(m.View()); got > w {
				t.Errorf("w=%d overflow: %d > %d", w, got, w)
			}
		}
	}
}

func TestSettingsEmptyStateOffersAdd(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m, err := newSettingsModel(80, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	out := m.View()
	if !strings.Contains(out, "add") {
		t.Errorf("empty settings should offer to add:\n%s", out)
	}
}

func TestSettingsMarksActiveAndMoves(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m, _ := newSettingsModel(80, seedStore(t))
	if m.active != "local" {
		t.Fatalf("active = %q", m.active)
	}
	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	if next.(settingsModel).cursor != 1 {
		t.Error("cursor did not move")
	}
	// `u` switches the active provider and persists it.
	moved := next.(settingsModel)
	after, _ := moved.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	s := after.(settingsModel)
	if s.active != m.names[1] {
		t.Errorf("active = %q, want %q", s.active, m.names[1])
	}
	reloaded, _ := deepeval.LoadStore(s.dir)
	if reloaded.Active != m.names[1] {
		t.Errorf("the switch was not saved: %q", reloaded.Active)
	}
}

// Actions that need their own full-screen program must quit and report what to run —
// Bubble Tea cannot nest alt-screen programs.
func TestSettingsDefersFullScreenActions(t *testing.T) {
	m, _ := newSettingsModel(80, seedStore(t))
	for key, want := range map[rune]settingsAction{'a': actAdd, 'e': actEdit, 'x': actRemove} {
		next, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		s := next.(settingsModel)
		if s.action != want {
			t.Errorf("%c → action %v, want %v", key, s.action, want)
		}
		if cmd == nil {
			t.Errorf("%c should quit so the sub-program can run", key)
		}
	}
}

// `s` from the dashboard must ask the caller to open settings, not try to nest.
func TestDashboardSKeyDefersToCaller(t *testing.T) {
	m := settled(profileWithSessions())
	next, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !next.(model).openSettings {
		t.Error("s did not request settings")
	}
	if cmd == nil {
		t.Error("s should quit the dashboard so settings can take the screen")
	}
}

func TestFooterAdvertisesSettings(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m := settled(profileWithSessions())
	m.w, m.h = 120, 40
	if !strings.Contains(m.View(), "settings") {
		t.Error("the footer should advertise settings")
	}
}
