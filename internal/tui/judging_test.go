package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func judgingAt(w, h int) judgingModel {
	return judgingModel{
		termW: w, termH: h,
		host:  "api.groq.com",
		stage: "judging your prompts",
		done:  24, total: 60,
	}
}

// The loader must state what it is doing and where, and show progress.
func TestJudgingViewShowsProgress(t *testing.T) {
	lipgloss.SetColorProfile(0)
	out := judgingAt(100, 40).View()
	for _, want := range []string{"READING YOUR PROMPTS", "api.groq.com", "judging your prompts", "24/60"} {
		if !strings.Contains(out, want) {
			t.Errorf("loader view missing %q", want)
		}
	}
}

// Cancelling must read as stopping, not as still working.
func TestJudgingViewCancelling(t *testing.T) {
	lipgloss.SetColorProfile(0)
	m := judgingAt(100, 40)
	m.canceling = true
	if !strings.Contains(m.View(), "stopping") {
		t.Error("cancelling state should say it is stopping")
	}
}

// Like every other screen, the loader reflows and never overflows its box.
func TestJudgingViewReflows(t *testing.T) {
	lipgloss.SetColorProfile(0)
	for _, sz := range []struct{ w, h int }{
		{46, 14}, {60, 20}, {74, 16}, {80, 24}, {120, 40}, {200, 50},
	} {
		m := judgingAt(sz.w, sz.h)
		out := m.View()
		if got := maxLineWidth(out); got > sz.w {
			t.Errorf("w=%d overflow: max line %d > %d", sz.w, got, sz.w)
		}
		if got := strings.Count(out, "\n") + 1; got > sz.h {
			t.Errorf("w=%d h=%d height overflow: %d lines > %d", sz.w, sz.h, got, sz.h)
		}
	}
}
