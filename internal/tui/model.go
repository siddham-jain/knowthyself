package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/siddham/synch/internal/profile"
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
}

// tickMsg drives the boot animation.
type tickMsg time.Time

const (
	animStep   = 0.045 // progress added per frame (~0.5s total)
	frameEvery = time.Second / 30
)

func newModel(p profile.Profile) model {
	return model{p: p, mode: viewOverview, booting: true}
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
	// Any key skips the boot animation; quit keys still quit.
	if m.booting {
		m.booting = false
		m.progress = 1
		if key == "q" || key == "ctrl+c" || key == "esc" {
			return m, tea.Quit
		}
		return m, nil
	}

	switch key {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
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
		// replay the boot animation
		m.booting = true
		m.progress = 0
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
