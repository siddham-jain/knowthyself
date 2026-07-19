package tui

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
)

// This file is the first-run gate: the very first time knowthyself is run on a real
// terminal, it asks whether to profile now instead of dropping a newcomer straight
// into their dashboard. Consent first, reveal second.

// firstRunModel is a two-choice prompt: "yes, show me" or "not right now".
type firstRunModel struct {
	termW  int
	cursor int // 0 = yes, 1 = no
	choice int // -1 until the user decides; 0 = yes, 1 = no
}

func (m firstRunModel) Init() tea.Cmd { return nil }

func (m firstRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "left", "h":
			m.cursor = 0
		case "down", "j", "right", "l":
			m.cursor = 1
		case "y", "Y", "1":
			m.choice = 0
			return m, tea.Quit
		case "n", "N", "2":
			m.choice = 1
			return m, tea.Quit
		case "enter", " ":
			m.choice = m.cursor
			return m, tea.Quit
		case "ctrl+c", "esc", "q":
			m.choice = 1 // treat a bail-out as "not now"
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m firstRunModel) View() string {
	width := momentWidth(m.termW)
	inner := clampInt(width-2, 30, 64)

	blocks := []string{}
	if m.termW >= 46 {
		blocks = append(blocks, wordmarkArt(), "")
	}
	blocks = append(blocks,
		lipgloss.NewStyle().Foreground(design.Accent).Bold(true).Render("Meet how you work with AI?"),
		design.Label.Render(wrap(
			"knowthyself reads your Claude Code history and plots how you actually collaborate — your strengths, your habits, your shape. Want to see it now?",
			inner)),
		"",
	)

	options := []string{"Yes — show me how I work with AI", "No, I don't want to know"}
	for i, opt := range options {
		if i == m.cursor {
			marker := lipgloss.NewStyle().Foreground(design.Accent).Bold(true).Render("▸ ")
			blocks = append(blocks, marker+lipgloss.NewStyle().Foreground(design.Ink).Bold(true).Render(opt))
		} else {
			blocks = append(blocks, "  "+lipgloss.NewStyle().Foreground(design.Muted).Render(opt))
		}
	}

	blocks = append(blocks,
		"",
		lipgloss.NewStyle().Foreground(design.Faint).Render(strings.Repeat("╌", inner)),
		design.Dim.Render("↑/↓ move · enter select · y / n shortcut"),
	)

	frame := lipgloss.JoinVertical(lipgloss.Center, blocks...)
	return "\n" + lipgloss.PlaceHorizontal(m.termW, lipgloss.Center, frame) + "\n"
}

// RunFirstRunPrompt shows the consent screen and reports whether the user chose to
// run the analysis now. Bailing out (esc/ctrl-c/q) counts as "not now".
func RunFirstRunPrompt(termW int) (bool, error) {
	m := firstRunModel{termW: termW, cursor: 0, choice: -1}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return false, err
	}
	return final.(firstRunModel).choice == 0, nil
}

// RenderMaybeLater is the parting screen when a first-time user declines: no data is
// read, and we leave them the one command they need.
func RenderMaybeLater(w io.Writer, termW int) {
	accent := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)
	frame := lipgloss.JoinVertical(lipgloss.Left,
		design.Title.Render("Maybe later 👋"),
		"",
		design.Label.Render("No rush. Whenever you're curious how you work with AI, just run:"),
		"",
		accent.Render("  knowthyself"),
	)
	fmt.Fprintln(w, "\n"+frame+"\n")
}
