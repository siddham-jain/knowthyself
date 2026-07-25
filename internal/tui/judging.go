package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
	"github.com/siddham-jain/knowthyself/internal/insight/deepeval"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

// The deep-read progress screen. Judging is a long series of network round trips, so
// it runs in the background while this renders a centred, framed loader in the tool's
// own idiom — the same measure and palette as the cold-start screens, not a bare line
// under the prompt.

var judgeFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// JudgeFunc performs the judging, reporting progress as chunks complete.
type JudgeFunc func(progress func(stage string, done, total int)) (*profile.DeepRead, error)

type judgeProgress struct {
	stage     string
	done, tot int
}

type judgeDone struct {
	read *profile.DeepRead
	err  error
}

type judgeTickMsg struct{}

type judgingModel struct {
	termW, termH int
	host         string
	stage        string
	done, total  int
	frame        int
	finished     bool
	canceling    bool
	read         *profile.DeepRead
	err          error

	events chan tea.Msg
	cancel context.CancelFunc
}

func (m judgingModel) Init() tea.Cmd {
	return tea.Batch(judgeTick(), waitForEvent(m.events))
}

func judgeTick() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return judgeTickMsg{} })
}

// waitForEvent turns the next event from the worker into a tea.Msg. Exactly one is
// kept in flight at a time, re-armed after each progress update.
func waitForEvent(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m judgingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termW, m.termH = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c", "q":
			// Stop the work, but wait for it to unwind so nothing is left sending.
			if !m.finished && !m.canceling {
				m.canceling = true
				if m.cancel != nil {
					m.cancel()
				}
			}
		}
	case judgeProgress:
		m.stage, m.done, m.total = msg.stage, msg.done, msg.tot
		return m, waitForEvent(m.events)
	case judgeDone:
		m.finished = true
		m.read, m.err = msg.read, msg.err
		return m, tea.Quit
	case judgeTickMsg:
		m.frame++
		if !m.finished {
			return m, judgeTick()
		}
	}
	return m, nil
}

func (m judgingModel) View() string {
	width := momentWidth(m.termW)
	inner := clampInt(width-2, 30, 64)
	accent := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)

	blocks := []string{}
	// The framed mark is seven lines; only show it when there is width for the frame
	// and height to spare, so a short terminal shows the loader, not a clipped lintel.
	if m.termW >= markWidth && (m.termH == 0 || m.termH >= 18) {
		blocks = append(blocks, inscriptionArt(m.termW), "")
	}

	stage := m.stage
	if stage == "" {
		stage = deepeval.StageJudging
	}
	spin := accent.Render(string(judgeFrames[m.frame%len(judgeFrames)]))
	if m.canceling {
		spin = design.Dim.Render(string(judgeFrames[m.frame%len(judgeFrames)]))
		stage = "stopping"
	}

	hint := "esc to stop · your scores stay local either way"
	if len([]rune(hint)) > inner {
		hint = "esc to stop"
	}

	barW := clampInt(inner-len(counter(m.done, m.total))-4, 12, 42)
	blocks = append(blocks,
		accent.Render("READING YOUR PROMPTS"),
		design.Label.Render(wrap(m.host+" · nothing else leaves your machine", inner)),
		"",
		spin+"  "+lipgloss.NewStyle().Foreground(design.Ink).Render(stage),
		progressBar(m.done, m.total, barW)+"  "+design.Label.Render(counter(m.done, m.total)),
		"",
		lipgloss.NewStyle().Foreground(design.Faint).Render(strings.Repeat("╌", inner)),
		design.Dim.Render(hint),
	)

	frame := lipgloss.JoinVertical(lipgloss.Center, blocks...)
	if m.termH > 0 {
		return lipgloss.Place(m.termW, m.termH, lipgloss.Center, lipgloss.Center, frame)
	}
	return "\n" + lipgloss.PlaceHorizontal(m.termW, lipgloss.Center, frame) + "\n"
}

// progressBar is a fixed-width meter for done/total, amber for the filled span. Before
// the first count it reads as indeterminate rather than empty.
func progressBar(done, total, width int) string {
	filled := 0
	if total > 0 {
		filled = int(float64(done)/float64(total)*float64(width) + 0.5)
	}
	filled = clampInt(filled, 0, width)
	on := lipgloss.NewStyle().Foreground(design.Accent).Render(strings.Repeat("█", filled))
	off := lipgloss.NewStyle().Foreground(design.Faint).Render(strings.Repeat("░", width-filled))
	return on + off
}

func counter(done, total int) string {
	if total <= 0 {
		return "…"
	}
	return fmt.Sprintf("%d/%d", done, total)
}

// RunJudging renders the loader while work runs in the background, returning the read
// it produced. esc cancels via cancel and the read ends as ErrCanceled.
func RunJudging(termW int, host string, total int, cancel context.CancelFunc, work JudgeFunc) (*profile.DeepRead, error) {
	events := make(chan tea.Msg, 16)
	go func() {
		read, err := work(func(stage string, done, tot int) {
			events <- judgeProgress{stage: stage, done: done, tot: tot}
		})
		events <- judgeDone{read: read, err: err}
	}()

	m := judgingModel{termW: termW, host: host, total: total, stage: deepeval.StageJudging, events: events, cancel: cancel}
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return nil, err
	}
	fm := final.(judgingModel)
	switch {
	case fm.read != nil:
		return fm.read, nil
	case fm.canceling:
		return nil, deepeval.ErrCanceled{}
	default:
		return nil, fm.err
	}
}
