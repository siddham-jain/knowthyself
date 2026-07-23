package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
)

// The deep-eval consent gate. Nothing leaves the machine until this screen is
// accepted, and it shows exactly what would be sent — the endpoint, the model, the
// volume, and the redacted text itself, which the user can page through.

// ConsentRequest is what the user is being asked to approve.
type ConsentRequest struct {
	Host    string
	Model   string
	Prompts int
	Chars   int
	// Samples are the redacted prompts, exactly as they would be transmitted.
	Samples []string
}

// estimatedTokens is a deliberately rough figure; ~4 characters per token is close
// enough to set expectations without implying a precision we don't have.
func (r ConsentRequest) estimatedTokens() int { return r.Chars / 4 }

type consentModel struct {
	req     ConsentRequest
	termW   int
	cursor  int // 0 = send, 1 = cancel
	choice  int // -1 until decided
	preview int // -1 when not previewing, else index into Samples
}

func (m consentModel) Init() tea.Cmd { return nil }

func (m consentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW = msg.Width
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m consentModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.preview >= 0 {
		switch key {
		case "right", "l", "n", " ":
			m.preview = minInt(m.preview+1, len(m.req.Samples)-1)
		case "left", "h", "p":
			m.preview = maxInt(m.preview-1, 0)
		default:
			m.preview = -1
		}
		return m, nil
	}

	switch key {
	case "up", "k", "left", "h":
		m.cursor = 0
	case "down", "j", "right", "l":
		m.cursor = 1
	case "v":
		if len(m.req.Samples) > 0 {
			m.preview = 0
		}
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
		m.choice = 1 // bailing out is a refusal, never an approval
		return m, tea.Quit
	}
	return m, nil
}

func (m consentModel) View() string {
	if m.preview >= 0 {
		return m.previewView()
	}
	width := momentWidth(m.termW)
	inner := clampInt(width-2, 30, 64)
	accent := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)

	blocks := []string{
		accent.Render("Send " + fmt.Sprintf("%d", m.req.Prompts) + " redacted prompts for a deep read?"),
		design.Label.Render(wrap(
			"Your scores stay local and deterministic — this only adds a written read of how you phrase things. Secrets, paths, URLs and emails are stripped before anything is sent.",
			inner)),
		"",
		// Joined as one block so the label/value columns line up; centring each row
		// separately would stagger them.
		lipgloss.JoinVertical(lipgloss.Left,
			detail("endpoint", m.req.Host),
			detail("model   ", m.req.Model),
			detail("sending ", fmt.Sprintf("%d prompts · %s chars · ~%s tokens",
				m.req.Prompts, commas(m.req.Chars), commas(m.req.estimatedTokens())))),
		"",
		design.Dim.Render("v — read exactly what would be sent"),
		"",
	}

	for i, opt := range []string{"Yes — send it", "No, keep everything local"} {
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

func (m consentModel) previewView() string {
	width := clampInt(m.termW-4, 40, 88)
	body := wrap(m.req.Samples[m.preview], width-4)
	header := design.Label.Render(fmt.Sprintf("PROMPT %d OF %d", m.preview+1, len(m.req.Samples)))
	panel := panelBox(width).Render(header + "\n\n" + lipgloss.NewStyle().Foreground(design.Ink).Render(body))
	hint := design.Dim.Render("←/→ move · any other key goes back")
	return "\n" + lipgloss.PlaceHorizontal(m.termW, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, panel, "", hint)) + "\n"
}

// RunConsentPrompt shows the gate and reports whether the user approved the send.
// Any exit other than an explicit yes is a refusal.
func RunConsentPrompt(termW int, req ConsentRequest) (bool, error) {
	m := consentModel{req: req, termW: termW, cursor: 0, choice: -1, preview: -1}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return false, err
	}
	return final.(consentModel).choice == 0, nil
}
