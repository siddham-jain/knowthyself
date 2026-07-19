package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

// viewMode is the active tab in the interactive dashboard.
type viewMode int

const (
	viewOverview viewMode = iota
	viewSessions
	viewTrends
	viewCount
)

// model is the Bubble Tea state for the interactive dashboard.
type model struct {
	p profile.Profile

	w, h int // terminal size

	mode       viewMode
	dimCursor  int // selected dimension in Overview
	sessCursor int // selected session in Sessions

	// boot animation
	booting  bool
	progress float64 // 0..1

	// atReveal is the first-run persona portrait the boot lands on; any key leaves
	// it for the graded dashboard.
	atReveal bool
}

// tickMsg drives the boot animation.
type tickMsg time.Time

const (
	// A ~0.9s power-on at 60fps. Progress is linear here; the view applies an
	// ease-out curve so the instrument decelerates smoothly into its settled state.
	animStep   = 1.0 / 54.0
	frameEvery = time.Second / 60
)

func newModel(p profile.Profile) model {
	// Sessions are ordered oldest-first (chronological, for the trends view), so open
	// the Sessions list focused on the newest session — that's the work you just did
	// and expect to see, not a months-old entry buried at the bottom.
	sessCursor := maxInt(0, len(p.Sessions)-1)
	return model{p: p, mode: viewOverview, sessCursor: sessCursor, booting: true, atReveal: true}
}

func (m model) Init() tea.Cmd { return tickCmd() }

func tickCmd() tea.Cmd {
	return tea.Tick(frameEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		if m.booting {
			m.progress += animStep
			if m.progress >= 1 {
				m.progress = 1
				m.booting = false
				return m, nil
			}
			return m, tickCmd()
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	// Quit always wins, in any state.
	if key == "q" || key == "ctrl+c" || key == "esc" {
		return m, tea.Quit
	}
	// Any key skips the boot animation, landing on the reveal.
	if m.booting {
		m.booting = false
		m.progress = 1
		return m, nil
	}
	// From the reveal, any key opens the dashboard (honoring a tab/number jump).
	if m.atReveal {
		m.atReveal = false
		switch key {
		case "2":
			m.mode = viewSessions
		case "3":
			m.mode = viewTrends
		default:
			m.mode = viewOverview
		}
		return m, nil
	}

	switch key {
	case "tab", "right":
		m.mode = (m.mode + 1) % viewCount
	case "shift+tab", "left":
		m.mode = (m.mode + viewCount - 1) % viewCount
	case "1":
		m.mode = viewOverview
	case "2":
		m.mode = viewSessions
	case "3":
		m.mode = viewTrends
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "r":
		// replay the boot animation, back to the reveal
		m.booting = true
		m.progress = 0
		m.atReveal = true
		return m, tickCmd()
	}
	return m, nil
}

func (m *model) moveCursor(d int) {
	switch m.mode {
	case viewOverview:
		m.dimCursor = clampInt(m.dimCursor+d, 0, len(m.p.Dimensions)-1)
	case viewSessions:
		if len(m.p.Sessions) > 0 {
			m.sessCursor = clampInt(m.sessCursor+d, 0, len(m.p.Sessions)-1)
		}
	}
}

func clampInt(x, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
