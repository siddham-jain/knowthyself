package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
)

type confirmModel struct {
	termW          int
	headline, body string
	yes, no        string
	cursor         int
	choice         int // -1 until decided
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "left", "h":
			m.cursor = 0
		case "down", "j", "right", "l":
			m.cursor = 1
		case "y", "Y":
			m.choice = 0
			return m, tea.Quit
		case "n", "N":
			m.choice = 1
			return m, tea.Quit
		case "enter", " ":
			m.choice = m.cursor
			return m, tea.Quit
		case "ctrl+c", "esc", "q":
			m.choice = 1
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	width := momentWidth(m.termW)
	inner := clampInt(width-2, 30, 64)
	accent := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)

	blocks := []string{
		accent.Render(m.headline),
		design.Label.Render(wrap(m.body, inner)),
		"",
	}
	for i, opt := range []string{m.yes, m.no} {
		if i == m.cursor {
			blocks = append(blocks, accent.Render("▸ ")+lipgloss.NewStyle().Foreground(design.Ink).Bold(true).Render(opt))
		} else {
			blocks = append(blocks, "  "+lipgloss.NewStyle().Foreground(design.Muted).Render(opt))
		}
	}
	blocks = append(blocks,
		"",
		lipgloss.NewStyle().Foreground(design.Faint).Render(strings.Repeat("╌", inner)),
		design.Dim.Render("↑/↓ move · enter select · y / n shortcut"),
	)
	return "\n" + lipgloss.PlaceHorizontal(m.termW, lipgloss.Center, lipgloss.JoinVertical(lipgloss.Center, blocks...)) + "\n"
}

// RunConfirm asks a yes/no question. Anything other than an explicit yes is a no.
func RunConfirm(termW int, headline, body, yes, no string) (bool, error) {
	m := confirmModel{termW: termW, headline: headline, body: body, yes: yes, no: no, choice: -1}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return false, err
	}
	return final.(confirmModel).choice == 0, nil
}
